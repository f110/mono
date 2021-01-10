package etcd

import (
	"context"
	"io"

	"cloud.google.com/go/storage"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	"google.golang.org/api/option"

	"go.f110.dev/mono/go/pkg/logger"
)

type Uploader struct {
	credentialJSON []byte
	bucket         string
}

func NewUploader(creds []byte, bucket string) *Uploader {
	return &Uploader{credentialJSON: creds, bucket: bucket}
}

func (u *Uploader) Upload(ctx context.Context, data io.Reader, path string) error {
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON(u.credentialJSON))
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	obj := client.Bucket(u.bucket).Object(path)
	w := obj.NewWriter(ctx)
	if _, err := io.Copy(w, data); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if err := w.Close(); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	logger.Log.Info("Succeeded upload", zap.String("object_name", obj.ObjectName()), zap.String("bucket", obj.BucketName()))
	return nil
}
