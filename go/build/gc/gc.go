package gc

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/storage"
)

type GC struct {
	interval time.Duration
	dao      dao.Options
	storage  *storage.S3
}

func NewGC(interval time.Duration, daoOpt dao.Options, bucket string, storageOpt storage.S3Options) *GC {
	return &GC{
		interval: interval,
		dao:      daoOpt,
		storage:  storage.NewS3(bucket, storageOpt),
	}
}

func (g *GC) Start() {
	t := time.NewTicker(g.interval)

	ctx, cancelFunc := ctxutil.WithTimeout(context.Background(), 30*time.Second)
	g.sweep(ctx)
	cancelFunc()
	for {
		select {
		case <-t.C:
			ctx, cancelFunc = ctxutil.WithTimeout(context.Background(), 30*time.Second)
			g.sweep(ctx)
			cancelFunc()
		}
	}
}

func (g *GC) sweep(ctx context.Context) {
	tasks, err := g.dao.Task.ListAll(ctx)
	if err != nil {
		slogger.Log.Warn("Failed to get all tasks", slogger.E(err))
		return
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Id > tasks[j].Id
	})

	garbageTasks := tasks[10:]
	for _, t := range garbageTasks {
		if err := g.cleanTask(ctx, t); err != nil {
			slogger.Log.Info("Failed to cleanup task", slogger.E(err), slog.Int("task_id", int(t.Id)))
		}
	}
}

func (g *GC) cleanTask(ctx context.Context, t *database.Task) error {
	if t.FinishedAt == nil {
		return nil
	}

	if t.LogFile != "" {
		slogger.Log.Info("Delete log file from object storage", slog.String("name", t.LogFile), slog.Int("task_id", int(t.Id)))
		if err := g.storage.Delete(ctx, t.LogFile); err != nil {
			return xerrors.WithStack(err)
		}
	}

	slogger.Log.Info("Delete task", slog.Int("task_id", int(t.Id)))
	if err := g.dao.Task.Delete(ctx, t.Id); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}
