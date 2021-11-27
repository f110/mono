package gomodule

import (
	"context"
	"errors"
	"fmt"
	"io"

	"go.f110.dev/mono/go/pkg/storage"
)

var CacheMiss = errors.New("gomodule: cache hit miss")

type ModuleCache struct {
	objectStorage *storage.S3
}

func NewModuleCache(endpoint, region, bucket, accessKey, secretAccessKey string) *ModuleCache {
	opt := storage.NewS3OptionToExternal(endpoint, region, accessKey, secretAccessKey)
	objStorage := storage.NewS3(bucket, opt)
	return &ModuleCache{
		objectStorage: objStorage,
	}
}

func (c *ModuleCache) Archive(ctx context.Context, module, version string, w io.Writer) error {
	if c.objectStorage == nil {
		return CacheMiss
	}
	r, err := c.objectStorage.Get(ctx, fmt.Sprintf("%s@%s.zip", module, version))
	if err != nil {
		return CacheMiss
	}
	defer r.Close()
	if _, err := io.Copy(w, r); err != nil {
		return err
	}

	return nil
}
