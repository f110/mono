package gc

import (
	"context"
	"sort"
	"time"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/pkg/build/database"
	"go.f110.dev/mono/go/pkg/build/database/dao"
	"go.f110.dev/mono/go/pkg/build/web"
	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/storage"
)

type GC struct {
	interval time.Duration
	dao      dao.Options
	minio    *storage.MinIO
}

func NewGC(interval time.Duration, daoOpt dao.Options, bucket string, minioOpt storage.MinIOOptions) *GC {
	return &GC{
		interval: interval,
		dao:      daoOpt,
		minio:    storage.NewMinIOStorage(bucket, minioOpt),
	}
}

func (g *GC) Start() {
	t := time.NewTicker(g.interval)

	ctx, cancelFunc := context.WithTimeout(context.Background(), 30*time.Second)
	g.sweep(ctx)
	cancelFunc()
	for {
		select {
		case <-t.C:
			ctx, cancelFunc = context.WithTimeout(context.Background(), 30*time.Second)
			g.sweep(ctx)
			cancelFunc()
		}
	}
}

func (g *GC) sweep(ctx context.Context) {
	tasks, err := g.dao.Task.ListAll(ctx)
	if err != nil {
		logger.Log.Warn("Failed to get all tasks", zap.Error(err))
		return
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Id > tasks[j].Id
	})

	garbageTasks := tasks[web.NumberOfTaskPerJob:]
	for _, t := range garbageTasks {
		if err := g.cleanTask(ctx, t); err != nil {
			logger.Log.Info("Failed to cleanup task", zap.Error(err), zap.Int32("task_id", t.Id))
		}
	}
}

func (g *GC) cleanTask(ctx context.Context, t *database.Task) error {
	if t.FinishedAt == nil {
		return nil
	}

	if t.LogFile != "" {
		logger.Log.Info("Delete log file from object storage", zap.String("name", t.LogFile), zap.Int32("task_id", t.Id))
		if err := g.minio.Delete(ctx, t.LogFile); err != nil {
			return xerrors.WithStack(err)
		}
	}

	logger.Log.Info("Delete task", zap.Int32("task_id", t.Id))
	if err := g.dao.Task.Delete(ctx, t.Id); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}
