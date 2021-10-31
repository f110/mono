package storage

import (
	"bytes"
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

var _ storageInterface = &Google{}

func NewGCS(creds []byte, bucket string) *Google {
	return &Google{credentialJSON: creds, bucket: bucket}
}

func (g *Google) Name() string {
	return "gcs"
}

func (g *Google) Put(ctx context.Context, name string, data []byte) error {
	return g.PutReader(ctx, name, bytes.NewReader(data))
}

func (g *Google) PutReader(ctx context.Context, name string, data io.Reader) error {
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON(g.credentialJSON))
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	obj := client.Bucket(g.bucket).Object(name)
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

func (g *Google) List(ctx context.Context, prefix string) ([]*Object, error) {
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON(g.credentialJSON))
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	iter := client.Bucket(g.bucket).Objects(ctx, &storage.Query{Prefix: prefix})
	var files []*Object
	for {
		objAttr, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		files = append(files, &Object{
			Name:         objAttr.Name,
			LastModified: objAttr.Updated,
			Size:         objAttr.Size,
		})
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

func (g *Google) Get(ctx context.Context, name string) (io.ReadCloser, error) {
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON(g.credentialJSON))
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	obj := client.Bucket(g.bucket).Object(name)
	r, err := obj.NewReader(ctx)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return r, nil
}
