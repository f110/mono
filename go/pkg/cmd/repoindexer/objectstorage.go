package repoindexer

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/storage"
)

type ObjectStorageIndexManager struct {
	executionKey     int64
	bucket           string
	backend          StorageClient
	stripPrefixSlash bool

	uploadedFiles []string
}

type StorageClient interface {
	Name() string
	Get(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
	Put(context.Context, string, []byte) error
	PutReader(context.Context, string, io.Reader) error
	List(context.Context, string) ([]*storage.Object, error)
}

func NewObjectStorageIndexManager(s StorageClient, bucket string) *ObjectStorageIndexManager {
	stripPrefixSlash := false
	switch s.(type) {
	case *storage.MinIO:
		stripPrefixSlash = true
	default:
		stripPrefixSlash = true
	}
	return &ObjectStorageIndexManager{bucket: bucket, backend: s, executionKey: time.Now().Unix(), stripPrefixSlash: stripPrefixSlash}
}

func (s *ObjectStorageIndexManager) ExecutionKey() uint64 {
	return uint64(s.executionKey)
}

func (s *ObjectStorageIndexManager) Add(ctx context.Context, name string, files []string) (string, uint64, error) {
	totalSize := uint64(0)
	for _, v := range files {
		f, err := os.Open(v)
		if err != nil {
			return "", 0, xerrors.WithStack(err)
		}
		info, err := f.Stat()
		if err != nil {
			return "", 0, xerrors.WithStack(err)
		}
		objectPath := filepath.Join(name, fmt.Sprintf("%d", s.executionKey), filepath.Base(v))
		err = s.backend.PutReader(ctx, objectPath, f)
		if err != nil {
			return "", 0, xerrors.WithStack(err)
		}
		logger.Log.Info("Successfully upload", zap.String("name", objectPath), zap.String("bucket", s.bucket), zap.Int64("size", info.Size()))
		s.uploadedFiles = append(s.uploadedFiles, objectPath)
		totalSize += uint64(info.Size())
	}

	uploadedURL := fmt.Sprintf("%s://%s/%s", s.backend.Name(), s.bucket, filepath.Join(name, fmt.Sprintf("%d", s.executionKey)))
	return uploadedURL, totalSize, nil
}

func (s *ObjectStorageIndexManager) Download(ctx context.Context, indexDir string, manifest Manifest) error {
	tmpDir := filepath.Join(indexDir, ".tmp")
	if _, err := os.Stat(tmpDir); err == nil {
		logger.Log.Debug("Remove tmp directory", zap.String("dir", tmpDir))
		if err := os.RemoveAll(tmpDir); err != nil {
			return xerrors.WithStack(err)
		}
	}
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return xerrors.WithStack(err)
	}

	for _, v := range manifest.Indexes {
		files, err := s.listFiles(ctx, v, s.stripPrefixSlash)
		if err != nil {
			return xerrors.WithStack(err)
		}

		for _, f := range files {
			r, err := s.backend.Get(ctx, f.Name)
			if err != nil {
				return xerrors.WithStack(err)
			}
			buf, err := io.ReadAll(r)
			if err != nil {
				r.Close()
				return xerrors.WithStack(err)
			}
			r.Close()

			filePath := filepath.Join(tmpDir, filepath.Base(f.Name))
			if err := os.WriteFile(filePath, buf, 0644); err != nil {
				return xerrors.WithStack(err)
			}
			logger.Log.Debug("Download file", zap.String("path", filePath))
		}
	}

	entry, err := os.ReadDir(indexDir)
	if err != nil {
		return xerrors.WithStack(err)
	}
	logger.Log.Debug("Remove old index files", zap.Int("len", len(entry)-1))
	for _, v := range entry {
		if v.Name() == ".tmp" {
			continue
		}
		if err := os.Remove(filepath.Join(indexDir, v.Name())); err != nil {
			return xerrors.WithStack(err)
		}
	}
	entry, err = os.ReadDir(tmpDir)
	if err != nil {
		return xerrors.WithStack(err)
	}
	for _, v := range entry {
		logger.Log.Debug("Move index file",
			zap.String("from", filepath.Join(tmpDir, v.Name())),
			zap.String("to", filepath.Join(indexDir, v.Name())),
		)
		if err := os.Rename(filepath.Join(tmpDir, v.Name()), filepath.Join(indexDir, v.Name())); err != nil {
			return xerrors.WithStack(err)
		}
	}
	if err := os.Remove(tmpDir); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (s *ObjectStorageIndexManager) Delete(ctx context.Context, manifests []Manifest) error {
	for _, manifest := range manifests {
		for _, index := range manifest.Indexes {
			files, err := s.listFiles(ctx, index, s.stripPrefixSlash)
			if err != nil {
				return xerrors.WithStack(err)
			}

			for _, f := range files {
				if err := s.backend.Delete(ctx, f.Name); err != nil {
					return xerrors.WithStack(err)
				}
			}
		}
	}

	return nil
}

func (s *ObjectStorageIndexManager) CleanUploadedFiles(ctx context.Context) error {
	for _, v := range s.uploadedFiles {
		if err := s.backend.Delete(ctx, v); err != nil {
			return xerrors.WithStack(err)
		}
	}

	return nil
}

func (s *ObjectStorageIndexManager) listFiles(ctx context.Context, indexDirURL string, stripPrefixSlash bool) ([]*storage.Object, error) {
	u, err := url.Parse(indexDirURL)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	path := u.Path
	if stripPrefixSlash && u.Path[0] == '/' {
		path = u.Path[1:]
	}
	files, err := s.backend.List(ctx, path)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	return files, nil
}
