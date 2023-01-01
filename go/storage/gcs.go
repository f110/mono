package storage

import (
	"bytes"
	"context"
	"io"

	"cloud.google.com/go/storage"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"go.f110.dev/mono/go/logger"
)

type GCSOptions struct {
	Retries int
}

type Google struct {
	credentialJSON []byte
	bucket string
	opt    GCSOptions
}

var _ storageInterface = &Google{}

func NewGCS(creds []byte, bucket string, opt GCSOptions) *Google {
	return &Google{credentialJSON: creds, bucket: bucket, opt: opt}
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
		return xerrors.WithStack(err)
	}

	obj := client.Bucket(g.bucket).Object(name)
	retryCount := 1
	for {
		w := obj.NewWriter(ctx)
		if _, err := io.Copy(w, data); err != nil {
			if g.opt.Retries > 0 && retryCount < g.opt.Retries {
				logger.Log.Info("Retrying to write a object", zap.Int("retryCount", retryCount), zap.String("key", name))
				retryCount++
				continue
			}
			return xerrors.WithStack(err)
		}
		if err := w.Close(); err != nil {
			return xerrors.WithStack(err)
		}

		logger.Log.Info("Succeeded upload", zap.String("object_name", obj.ObjectName()), zap.String("bucket", obj.BucketName()))
		return nil
	}
}

func (g *Google) List(ctx context.Context, prefix string) ([]*Object, error) {
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON(g.credentialJSON))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	iter := client.Bucket(g.bucket).Objects(ctx, &storage.Query{Prefix: prefix})
	var files []*Object
	for {
		objAttr, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, xerrors.WithStack(err)
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
		return xerrors.WithStack(err)
	}

	obj := client.Bucket(g.bucket).Object(key)
	retryCount := 1
	for {
		if err := obj.Delete(ctx); err != nil {
			if g.opt.Retries > 0 && retryCount < g.opt.Retries {
				logger.Log.Info("Retrying to delete a object", zap.Int("retryCount", retryCount), zap.String("key", key))
				retryCount++
				continue
			}
			return xerrors.WithStack(err)
		}

		return nil
	}
}

func (g *Google) Get(ctx context.Context, name string) (io.ReadCloser, error) {
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON(g.credentialJSON))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	obj := client.Bucket(g.bucket).Object(name)
	retryCount := 1
	for {
		r, err := obj.NewReader(ctx)
		if err != nil {
			if g.opt.Retries > 0 && retryCount < g.opt.Retries {
				logger.Log.Info("Retrying to get a object", zap.Int("retryCount", retryCount), zap.String("key", name))
				retryCount++
				continue
			}
			return nil, xerrors.WithStack(err)
		}

		return r, nil
	}
}
