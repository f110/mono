package storage

import (
	"context"
	"io"

	"cloud.google.com/go/storage"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	"google.golang.org/api/iterator"
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

func (g *Google) List(ctx context.Context, prefix string) ([]*storage.ObjectAttrs, error) {
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON(g.credentialJSON))
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	iter := client.Bucket(g.bucket).Objects(ctx, &storage.Query{Prefix: prefix})
	files := make([]*storage.ObjectAttrs, 0)
	for {
		objAttr, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		files = append(files, objAttr)
	}

	return files, nil
}

func (g *Google) Delete(ctx context.Context, key string) error {
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON(g.credentialJSON))
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	obj := client.Bucket(g.bucket).Object(key)
	if err := obj.Delete(ctx); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}
