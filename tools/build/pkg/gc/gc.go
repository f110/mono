package gc

import (
	"context"
	"time"

	"go.uber.org/zap"
	"golang.org/x/xerrors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"go.f110.dev/mono/lib/logger"
	"go.f110.dev/mono/tools/build/pkg/database"
	"go.f110.dev/mono/tools/build/pkg/database/dao"
	"go.f110.dev/mono/tools/build/pkg/storage"
	"go.f110.dev/mono/tools/build/pkg/web"
)

type GC struct {
	interval time.Duration
	dao      dao.Options
	minio    *storage.MinIO
}

func NewGC(interval time.Duration, daoOpt dao.Options, client kubernetes.Interface, config *rest.Config, minioOpt storage.MinIOOptions, dev bool) *GC {
	return &GC{
		interval: interval,
		dao:      daoOpt,
		minio:    storage.NewMinIOStorage(client, config, minioOpt, dev),
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
	jobs, err := g.dao.Job.ListAll(ctx)
	if err != nil {
		logger.Log.Warn("Failed get job's list", zap.Error(err))
		return
	}

	for _, v := range jobs {
		tasks, err := g.dao.Task.ListByJobId(ctx, v.Id)
		if err != nil {
			logger.Log.Info("Failed get task's list", zap.Error(err), zap.Int32("job_id", v.Id))
			continue
		}
		if len(tasks) < web.NumberOfTaskPerJob {
			continue
		}

		garbageTasks := tasks[web.NumberOfTaskPerJob:]
		for _, t := range garbageTasks {
			if err := g.cleanTask(ctx, t); err != nil {
				logger.Log.Info("Failed cleanup task", zap.Error(err), zap.Int32("task_id", t.Id))
			}
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
			return xerrors.Errorf(": %w", err)
		}
	}

	logger.Log.Info("Delete task", zap.Int32("task_id", t.Id))
	if err := g.dao.Task.Delete(ctx, t.Id); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	return nil
}
