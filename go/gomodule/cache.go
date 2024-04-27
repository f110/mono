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
	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

var CacheMiss = xerrors.Define("gomodule: cache hit miss")

type ModuleCache struct {
	cachePool     *client.SinglePool
	objectStorage *storage.S3
}

func NewModuleCache(cachePool *client.SinglePool, endpoint, region, bucket, accessKey, secretAccessKey, caCertFile string) *ModuleCache {
	opt := storage.NewS3OptionToExternal(endpoint, region, accessKey, secretAccessKey)
	opt.PathStyle = true
	opt.CACertFile = caCertFile
	objStorage := storage.NewS3(bucket, opt)
	return &ModuleCache{
		cachePool:     cachePool,
		objectStorage: objStorage,
	}
}

func (c *ModuleCache) Invalidate(module string) error {
	repoRoot, _, err := c.GetRepoRoot(module)
	if errors.Is(err, merrors.ItemNotFound) {
		return nil
	} else if err != nil {
		return xerrors.WithStack(err)
	}
	moduleRoot, err := c.GetModuleRoot(repoRoot, "", nil)
	if err != nil {
		return xerrors.WithStack(err)
	}
	key := fmt.Sprintf("moduleRoot/%s", moduleRoot.RootPath)
	logger.Log.Debug("Delete cache", zap.String("key", key))
	if err := c.cachePool.Delete(key); err != nil {
		return xerrors.WithStack(err)
	}
	if err := c.deleteCachedModuleRoot(moduleRoot.RootPath); err != nil {
		return xerrors.WithStack(err)
	}

	for _, mod := range moduleRoot.Modules {
		cachedModFiles, err := c.CachedModFiles(mod.Path)
		if err != nil {
			return xerrors.WithStack(err)
		}
		for _, v := range cachedModFiles {
			key = fmt.Sprintf("goModFile/%s/%s", mod.Path, v)
			logger.Log.Debug("Delete cache", zap.String("key", key))
			if err := c.cachePool.Delete(key); err != nil {
				logger.Log.Info("Failed invalidate mod file cache", zap.Error(err), zap.String("version", v))
			}
		}
		if err := c.cachePool.Delete(fmt.Sprintf("goModFiles/%s", mod.Path)); err != nil {
			return xerrors.WithStack(err)
		}

		cachedModInfos, err := c.CachedModInfos(mod.Path)
		if err != nil {
			return xerrors.WithStack(err)
		}
		for _, v := range cachedModInfos {
			key = fmt.Sprintf("modInfo/%s/%s", mod.Path, v)
			logger.Log.Debug("Delete cached", zap.String("key", key))
			if err := c.cachePool.Delete(key); err != nil {
				logger.Log.Info("Failed invalidate mod info", zap.Error(err), zap.String("version", v))
			}
		}
		if err := c.cachePool.Delete(fmt.Sprintf("goModInfos/%s", mod.Path)); err != nil {
			return xerrors.WithStack(err)
		}
	}

	return nil
}

func (c *ModuleCache) FlushAll() error {
	if err := c.cachePool.Flush(); err != nil {
		return xerrors.WithStack(err)
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
		return "", "", xerrors.WithStack(err)
	}
	var root repoRootCache
	if err := json.NewDecoder(bytes.NewReader(item.Value)).Decode(&root); err != nil {
		return "", "", xerrors.WithStack(err)
	}

	return root.Root, root.RepoURL, nil
}

func (c *ModuleCache) SetRepoRoot(importPath, repoRoot, repoURL string) error {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(&repoRootCache{Root: repoRoot, RepoURL: repoURL}); err != nil {
		return xerrors.WithStack(err)
	}
	err := c.cachePool.Set(&client.Item{
		Key:        fmt.Sprintf("repoRoot/%s", importPath),
		Value:      buf.Bytes(),
		Expiration: 60 * 60 * 24 * 7, // 1 week
	})
	if err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

type moduleRootCache struct {
	Modules []*Module
}

func (c *ModuleCache) GetModuleRoot(repoRoot string, baseDir string, vcs *VCS) (*ModuleRoot, error) {
	item, err := c.cachePool.Get(fmt.Sprintf("moduleRoot/%s", repoRoot))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	cachedItem := moduleRootCache{}
	if err := json.NewDecoder(bytes.NewReader(item.Value)).Decode(&cachedItem); err != nil {
		return nil, xerrors.WithStack(err)
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
		return xerrors.WithStack(err)
	}
	err := c.cachePool.Set(&client.Item{
		Key:        fmt.Sprintf("moduleRoot/%s", moduleRoot.RootPath),
		Value:      buf.Bytes(),
		Expiration: 60 * 60 * 24 * 7}, // 1 week
	)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if err := c.addCachedModuleRoot(moduleRoot.RootPath); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (c *ModuleCache) GetModFile(module, version string) ([]byte, error) {
	item, err := c.cachePool.Get(fmt.Sprintf("goModFile/%s/%s", module, version))
	if err != nil {
		return nil, xerrors.WithStack(err)
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
		return xerrors.WithStack(err)
	}
	if err := c.addCachedModFile(module, version); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (c *ModuleCache) GetModInfo(module, sha string) (time.Time, error) {
	if len(sha) < 12 {
		return time.Time{}, merrors.ItemNotFound
	}

	item, err := c.cachePool.Get(fmt.Sprintf("modInfo/%s/%s", module, sha[:12]))
	if err != nil {
		return time.Time{}, xerrors.WithStack(err)
	}

	t, err := time.Parse(time.RFC3339, string(item.Value))
	if err != nil {
		return time.Time{}, xerrors.WithStack(err)
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
		return xerrors.WithStack(err)
	}
	if err := c.addCachedModInfo(module, sha[:12]); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (c *ModuleCache) CachedModuleRoots() ([]*ModuleRoot, error) {
	item, err := c.getCachedModuleRoot()
	if err != nil && errors.Is(err, merrors.ItemNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, xerrors.WithStack(err)
	}

	var cachedModuleRoot []string
	if err := json.NewDecoder(bytes.NewReader(item.Value)).Decode(&cachedModuleRoot); err != nil {
		return nil, xerrors.WithStack(err)
	}

	var moduleRoots []*ModuleRoot
	for _, v := range cachedModuleRoot {
		moduleRoot, err := c.GetModuleRoot(v, "", nil)
		if err != nil && errors.Is(err, merrors.ItemNotFound) {
			continue
		} else if err != nil {
			return nil, xerrors.WithStack(err)
		}
		moduleRoots = append(moduleRoots, moduleRoot)
	}

	return moduleRoots, nil
}

func (c *ModuleCache) CachedModFiles(module string) ([]string, error) {
	item, err := c.getCachedModFile(module)
	if errors.Is(err, merrors.ItemNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, xerrors.WithStack(err)
	}

	var cachedModFiles []string
	if err := json.NewDecoder(bytes.NewReader(item.Value)).Decode(&cachedModFiles); err != nil {
		return nil, xerrors.WithStack(err)
	}

	return cachedModFiles, nil
}

func (c *ModuleCache) CachedModInfos(module string) ([]string, error) {
	item, err := c.getCachedModInfo(module)
	if errors.Is(err, merrors.ItemNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, xerrors.WithStack(err)
	}

	var cachedModInfos []string
	if err := json.NewDecoder(bytes.NewReader(item.Value)).Decode(&cachedModInfos); err != nil {
		return nil, xerrors.WithStack(err)
	}

	return cachedModInfos, nil
}

func (c *ModuleCache) Archive(ctx context.Context, module, version string, w io.Writer) error {
	if c == nil {
		return CacheMiss
	}

	if c.objectStorage == nil {
		return CacheMiss
	}
	r, err := c.objectStorage.Get(ctx, fmt.Sprintf("%s@%s.zip", module, version))
	if err != nil {
		return CacheMiss
	}
	defer r.Body.Close()
	if _, err := io.Copy(w, r.Body); err != nil {
		return err
	}

	return nil
}

func (c *ModuleCache) SaveArchive(ctx context.Context, module, version string, data []byte) error {
	if c == nil {
		return nil
	}
	if c.objectStorage == nil {
		return nil
	}

	err := c.objectStorage.Put(ctx, fmt.Sprintf("%s@%s.zip", module, version), data)
	if err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (c *ModuleCache) Ping() error {
	_, err := c.cachePool.Version()
	if err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (c *ModuleCache) getCachedModuleRoot() (*client.Item, error) {
	item, err := c.cachePool.Get("cachedModuleRoot")
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	return item, nil
}

func (c *ModuleCache) addCachedModuleRoot(repoRoot string) error {
	item, err := c.getCachedModuleRoot()
	if err != nil && !errors.Is(err, merrors.ItemNotFound) {
		return xerrors.WithStack(err)
	}

	var cachedModules []string
	if item != nil {
		if err := json.NewDecoder(bytes.NewReader(item.Value)).Decode(&cachedModules); err != nil {
			return xerrors.WithStack(err)
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
			Key:        "cachedModuleRoot",
			Expiration: 60 * 60 * 24 * 7, // 1 week
		}
	}

	cachedModules = append(cachedModules, repoRoot)
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(cachedModules); err != nil {
		return xerrors.WithStack(err)
	}
	item.Value = buf.Bytes()
	if err := c.cachePool.Set(item); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (c *ModuleCache) deleteCachedModuleRoot(moduleRoot string) error {
	item, err := c.getCachedModuleRoot()
	if err != nil && !errors.Is(err, merrors.ItemNotFound) {
		return xerrors.WithStack(err)
	}
	if item == nil {
		return nil
	}

	var cachedModules []string
	if err := json.NewDecoder(bytes.NewReader(item.Value)).Decode(&cachedModules); err != nil {
		return xerrors.WithStack(err)
	}
	found := false
	for i, v := range cachedModules {
		if v == moduleRoot {
			found = true
			cachedModules = append(cachedModules[:i], cachedModules[i+1:]...)
			break
		}
	}
	if !found {
		logger.Log.Debug("The module root not found", zap.String("module_root", moduleRoot))
		return nil
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(cachedModules); err != nil {
		return xerrors.WithStack(err)
	}
	item.Value = buf.Bytes()
	if err := c.cachePool.Set(item); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (c *ModuleCache) getCachedModFile(module string) (*client.Item, error) {
	item, err := c.cachePool.Get(fmt.Sprintf("goModFiles/%s", module))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	return item, nil
}

func (c *ModuleCache) addCachedModFile(module, version string) error {
	item, err := c.getCachedModFile(module)
	if err != nil && !errors.Is(err, merrors.ItemNotFound) {
		return xerrors.WithStack(err)
	}

	var cachedModFile []string
	if item != nil {
		if err := json.NewDecoder(bytes.NewReader(item.Value)).Decode(&cachedModFile); err != nil {
			return xerrors.WithStack(err)
		}
	} else {
		item = &client.Item{
			Key:        fmt.Sprintf("goModFiles/%s", module),
			Expiration: 60 * 60 * 24 * 7, // 1 week
		}
	}

	cachedModFile = append(cachedModFile, version)
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(cachedModFile); err != nil {
		return xerrors.WithStack(err)
	}
	item.Value = buf.Bytes()
	if err := c.cachePool.Set(item); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (c *ModuleCache) getCachedModInfo(module string) (*client.Item, error) {
	item, err := c.cachePool.Get(fmt.Sprintf("goModInfos/%s", module))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	return item, nil
}

func (c *ModuleCache) addCachedModInfo(module, version string) error {
	item, err := c.getCachedModInfo(module)
	if err != nil && !errors.Is(err, merrors.ItemNotFound) {
		return xerrors.WithStack(err)
	}

	var cachedModInfo []string
	if item != nil {
		if err := json.NewDecoder(bytes.NewReader(item.Value)).Decode(&cachedModInfo); err != nil {
			return xerrors.WithStack(err)
		}
	} else {
		item = &client.Item{
			Key:        fmt.Sprintf("goModInfos/%s", module),
			Expiration: 60 * 60 * 24 * 7, // 1 week
		}
	}

	cachedModInfo = append(cachedModInfo, version)
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(cachedModInfo); err != nil {
		return xerrors.WithStack(err)
	}
	item.Value = buf.Bytes()
	if err := c.cachePool.Set(item); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}
