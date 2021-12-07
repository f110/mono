package gomodule

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"go.f110.dev/go-memcached/client"
	merrors "go.f110.dev/go-memcached/errors"
	"go.uber.org/zap"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/storage"
)

var CacheMiss = errors.New("gomodule: cache hit miss")

type ModuleCache struct {
	cachePool     *client.SinglePool
	objectStorage *storage.S3
}

func NewModuleCache(cachePool *client.SinglePool, endpoint, region, bucket, accessKey, secretAccessKey string) *ModuleCache {
	opt := storage.NewS3OptionToExternal(endpoint, region, accessKey, secretAccessKey)
	opt.PathStyle = true
	objStorage := storage.NewS3(bucket, opt)
	return &ModuleCache{
		cachePool:     cachePool,
		objectStorage: objStorage,
	}
}

func (c *ModuleCache) Invalidate(repoRoot string) error {
	if err := c.cachePool.Delete(fmt.Sprintf("moduleRoot/%s", repoRoot)); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	return nil
}

type repoRootCache struct {
	Root    string
	RepoURL string
}

func (c *ModuleCache) GetRepoRoot(importPath string) (repoRoot string, repoURL string, err error) {
	item, err := c.cachePool.Get(fmt.Sprintf("repoRoot/%s", importPath))
	if err != nil {
		return "", "", xerrors.Errorf(": %w", err)
	}
	var root repoRootCache
	if err := json.NewDecoder(bytes.NewReader(item.Value)).Decode(&root); err != nil {
		return "", "", xerrors.Errorf(": %w", err)
	}

	return root.Root, root.RepoURL, nil
}

func (c *ModuleCache) SetRepoRoot(importPath, repoRoot, repoURL string) error {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(&repoRootCache{Root: repoRoot, RepoURL: repoURL}); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	err := c.cachePool.Set(&client.Item{
		Key:        fmt.Sprintf("repoRoot/%s", importPath),
		Value:      buf.Bytes(),
		Expiration: 60 * 60 * 24 * 7, // 1 week
	})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

type moduleRootCache struct {
	Modules []*Module
}

func (c *ModuleCache) GetModuleRoot(repoRoot string, baseDir string, vcs *VCS) (*ModuleRoot, error) {
	item, err := c.cachePool.Get(fmt.Sprintf("moduleRoot/%s", repoRoot))
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	cachedItem := moduleRootCache{}
	if err := json.NewDecoder(bytes.NewReader(item.Value)).Decode(&cachedItem); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	moduleRoot := NewModuleRootFromCache(repoRoot, cachedItem.Modules, c, vcs, filepath.Join(baseDir, repoRoot))
	return moduleRoot, nil
}

func (c *ModuleCache) SetModuleRoot(moduleRoot *ModuleRoot) error {
	item := &moduleRootCache{
		Modules: moduleRoot.Modules,
	}
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(item); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	err := c.cachePool.Set(&client.Item{
		Key:        fmt.Sprintf("moduleRoot/%s", moduleRoot.RootPath),
		Value:      buf.Bytes(),
		Expiration: 60 * 60 * 24 * 7}, // 1 week
	)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if err := c.addCachedModule(moduleRoot.RootPath); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (c *ModuleCache) GetModFile(module, version string) ([]byte, error) {
	item, err := c.cachePool.Get(fmt.Sprintf("goModFile/%s/%s", module, version))
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return item.Value, nil
}

func (c *ModuleCache) SetModFile(module, version string, modFile []byte) error {
	err := c.cachePool.Set(&client.Item{
		Key:        fmt.Sprintf("goModFile/%s/%s", module, version),
		Value:      modFile,
		Expiration: 60 * 60 * 24 * 7, // 1 week
	})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (c *ModuleCache) CachedModules() ([]string, error) {
	item, err := c.getCachedModules()
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	var cachedModules []string
	if err := json.NewDecoder(bytes.NewReader(item.Value)).Decode(&cachedModules); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return cachedModules, nil
}

func (c *ModuleCache) getCachedModules() (*client.Item, error) {
	item, err := c.cachePool.Get("cachedModules")
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	return item, nil
}

func (c *ModuleCache) addCachedModule(repoRoot string) error {
	item, err := c.getCachedModules()
	if err != nil && !errors.Is(err, merrors.ItemNotFound) {
		return xerrors.Errorf(": %w", err)
	}

	var cachedModules []string
	if item != nil {
		if err := json.NewDecoder(bytes.NewReader(item.Value)).Decode(&cachedModules); err != nil {
			return xerrors.Errorf(": %w", err)
		}
		found := false
		for _, v := range cachedModules {
			if v == repoRoot {
				found = true
				break
			}
		}
		if found {
			logger.Log.Debug("The module already cached", zap.String("repoRoot", repoRoot))
			return nil
		}
	} else {
		item = &client.Item{
			Key:        "cachedModules",
			Expiration: 60 * 60 * 24 * 7, // 1 week
		}
	}

	cachedModules = append(cachedModules, repoRoot)
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(cachedModules); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	item.Value = buf.Bytes()
	if err := c.cachePool.Set(item); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (c *ModuleCache) GetModInfo(module, sha string) (time.Time, error) {
	if len(sha) < 12 {
		return time.Time{}, merrors.ItemNotFound
	}

	item, err := c.cachePool.Get(fmt.Sprintf("modInfo/%s/%s", module, sha[:12]))
	if err != nil {
		return time.Time{}, xerrors.Errorf(": %w", err)
	}

	t, err := time.Parse(time.RFC3339, string(item.Value))
	if err != nil {
		return time.Time{}, xerrors.Errorf(": %w", err)
	}

	return t, nil
}

func (c *ModuleCache) SetModInfo(module, sha string, t time.Time) error {
	err := c.cachePool.Set(&client.Item{
		Key:        fmt.Sprintf("modInfo/%s/%s", module, sha[:12]),
		Value:      []byte(t.Format(time.RFC3339)),
		Expiration: 60 * 60 * 24 * 3, // 3 days
	})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
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

func (c *ModuleCache) SaveArchive(ctx context.Context, module, version string, data []byte) error {
	if c.objectStorage == nil {
		return nil
	}

	err := c.objectStorage.Put(ctx, fmt.Sprintf("%s@%s.zip", module, version), data)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}
