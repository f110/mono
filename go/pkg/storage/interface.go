package storage

import (
	"context"
	"io"
	"time"
)

// storageInterface defines common interface for the object storage.
// This interface is intended to type check not intended to used by other package.
type storageInterface interface {
	Name() string
	Put(ctx context.Context, name string, data []byte) error
	PutReader(ctx context.Context, name string, data io.Reader) error
	Delete(ctx context.Context, name string) error
	Get(ctx context.Context, name string) (io.ReadCloser, error)
	List(ctx context.Context, prefix string) ([]*Object, error)
}

type Object struct {
	Name         string
	Size         int64
	LastModified time.Time
}
