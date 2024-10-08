package codesearch

import (
	"context"
	"net/url"
	"strings"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/logger"
)

type IndexGC struct {
	backend StorageClient
	bucket  string
}

func NewIndexGC(s StorageClient, bucket string) *IndexGC {
	return &IndexGC{
		backend: s,
		bucket:  bucket,
	}
}

func (g *IndexGC) GC(ctx context.Context) error {
	manifestManager := NewManifestManager(g.backend)
	manifests, err := manifestManager.GetAll(ctx)
	if err != nil {
		return xerrors.WithStack(err)
	}

	used := make([]string, 0)
	for _, m := range manifests {
		for _, v := range m.Indexes {
			u, err := url.Parse(v)
			if err != nil {
				return xerrors.WithStack(err)
			}
			used = append(used, u.Path[1:])
		}
	}

	allFiles, err := g.backend.List(ctx, "")
	if err != nil {
		return xerrors.WithStack(err)
	}

	unusedFiles := make([]string, 0)
	var totalSize int64
GC:
	for _, v := range allFiles {
		if strings.HasPrefix(v.Name, "manifest_") {
			continue
		}
		for _, u := range used {
			if strings.HasPrefix(v.Name, u) {
				continue GC
			}
		}

		unusedFiles = append(unusedFiles, v.Name)
		totalSize += v.Size
	}

	for _, v := range unusedFiles {
		logger.Log.Debug("Delete file", zap.String("name", v))
		if err := g.backend.Delete(ctx, v); err != nil {
			return xerrors.WithStack(err)
		}
	}

	logger.Log.Info("Finish garbage collection", zap.Int("files", len(unusedFiles)), zap.Int64("deleted_bytes", totalSize))
	return nil
}
