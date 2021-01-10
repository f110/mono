package storage

import (
	"context"
	"io"

	"cloud.google.com/go/storage"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	"google.golang.org/api/option"

	"go.f110.dev/mono/go/pkg/logger"
)

type Google struct {
	credentialJSON []byte
	bucket         string
}

func NewGCS(creds []byte, bucket string) *Google {
	return &Google{credentialJSON: creds, bucket: bucket}
}

func (g *Google) Put(ctx context.Context, data io.Reader, path string) error {
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON(g.credentialJSON))
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	obj := client.Bucket(g.bucket).Object(path)
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
