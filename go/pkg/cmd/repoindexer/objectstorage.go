package repoindexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"golang.org/x/xerrors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/storage"
)

type ObjectStorageUploader struct {
	executionKey string
	bucket       string
	backend      *storage.MinIO
}

func NewObjectStorageUploader(
	k8sClient kubernetes.Interface,
	k8sConf *rest.Config,
	name, namespace string,
	port int,
	bucket, accessKey, secretAccessKey string,
	dev bool,
) *ObjectStorageUploader {
	opt := storage.NewMinIOOptions(name, namespace, port, bucket, accessKey, secretAccessKey)
	s := storage.NewMinIOStorage(k8sClient, k8sConf, opt, dev)

	return &ObjectStorageUploader{bucket: bucket, backend: s, executionKey: fmt.Sprintf("%d", time.Now().Unix())}
}

func (s *ObjectStorageUploader) Upload(ctx context.Context, name string, files []string) (string, error) {
	for _, v := range files {
		f, err := os.Open(v)
		if err != nil {
			return "", xerrors.Errorf(": %w", err)
		}
		info, err := f.Stat()
		if err != nil {
			return "", xerrors.Errorf(": %w", err)
		}
		objectPath := filepath.Join(name, s.executionKey, filepath.Base(v))
		err = s.backend.PutReader(ctx, objectPath, f, info.Size())
		if err != nil {
			return "", xerrors.Errorf(": %w", err)
		}
		logger.Log.Info("Successfully upload", zap.String("name", objectPath))
	}

	return fmt.Sprintf("minio://%s/%s", s.bucket, filepath.Join(name, s.executionKey)), nil
}
