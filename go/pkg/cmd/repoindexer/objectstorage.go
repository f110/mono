package repoindexer

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/storage"
)

type ObjectStorageIndexManager struct {
	executionKey int64
	bucket       string
	backend      *storage.MinIO
}

func NewObjectStorageIndexManager(s *storage.MinIO, bucket string) *ObjectStorageIndexManager {
	return &ObjectStorageIndexManager{bucket: bucket, backend: s, executionKey: time.Now().Unix()}
}

func (s *ObjectStorageIndexManager) ExecutionKey() uint64 {
	return uint64(s.executionKey)
}

func (s *ObjectStorageIndexManager) Add(ctx context.Context, name string, files []string) (string, error) {
	for _, v := range files {
		f, err := os.Open(v)
		if err != nil {
			return "", xerrors.Errorf(": %w", err)
		}
		info, err := f.Stat()
		if err != nil {
			return "", xerrors.Errorf(": %w", err)
		}
		objectPath := filepath.Join(name, fmt.Sprintf("%d", s.executionKey), filepath.Base(v))
		err = s.backend.PutReader(ctx, objectPath, f, info.Size())
		if err != nil {
			return "", xerrors.Errorf(": %w", err)
		}
		logger.Log.Info("Successfully upload", zap.String("name", objectPath))
	}

	return fmt.Sprintf("minio://%s/%s", s.bucket, filepath.Join(name, fmt.Sprintf("%d", s.executionKey))), nil
}

func (s *ObjectStorageIndexManager) Download(ctx context.Context, indexDir string, manifest Manifest) error {
	tmpDir := filepath.Join(indexDir, ".tmp")
	if _, err := os.Stat(tmpDir); err == nil {
		logger.Log.Debug("Remove tmp directory", zap.String("dir", tmpDir))
		if err := os.RemoveAll(tmpDir); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	for _, v := range manifest.Indexes {
		files, err := s.listFiles(ctx, v)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		for _, f := range files {
			buf, err := s.backend.Get(ctx, f)
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}

			filePath := filepath.Join(tmpDir, filepath.Base(f))
			if err := os.WriteFile(filePath, buf, 0644); err != nil {
				return xerrors.Errorf(": %w", err)
			}
			logger.Log.Debug("Download file", zap.String("path", filePath))
		}
	}

	entry, err := os.ReadDir(indexDir)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	logger.Log.Debug("Remove old index files", zap.Int("len", len(entry)-1))
	for _, v := range entry {
		if v.Name() == ".tmp" {
			continue
		}
		if err := os.Remove(filepath.Join(indexDir, v.Name())); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}
	entry, err = os.ReadDir(tmpDir)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	for _, v := range entry {
		logger.Log.Debug("Move index file",
			zap.String("from", filepath.Join(tmpDir, v.Name())),
			zap.String("to", filepath.Join(indexDir, v.Name())),
		)
		if err := os.Rename(filepath.Join(tmpDir, v.Name()), filepath.Join(indexDir, v.Name())); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}
	if err := os.Remove(tmpDir); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (s *ObjectStorageIndexManager) Delete(ctx context.Context, manifests []Manifest) error {
	for _, manifest := range manifests {
		for _, index := range manifest.Indexes {
			files, err := s.listFiles(ctx, index)
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}

			for _, f := range files {
				if err := s.backend.Delete(ctx, f); err != nil {
					return xerrors.Errorf(": %w", err)
				}
			}
		}
	}

	return nil
}

func (s *ObjectStorageIndexManager) listFiles(ctx context.Context, indexDirURL string) ([]string, error) {
	u, err := url.Parse(indexDirURL)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	files, err := s.backend.List(ctx, u.Path)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return files, nil
}
