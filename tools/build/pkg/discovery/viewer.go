package discovery

import (
	"strconv"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/xerrors"
	batchv1 "k8s.io/api/batch/v1"

	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/tools/build/pkg/watcher"
)

type Viewer struct {
	mu    sync.Mutex
	cache map[int32]struct{}
}

func NewViewer() *Viewer {
	v := &Viewer{cache: make(map[int32]struct{})}
	watcher.Router.Add(jobType, v.syncJob)

	return v
}

func (d *Viewer) IsDiscovering(repoId int32) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.cache[repoId]; ok {
		return true
	}

	return false
}

func (d *Viewer) syncJob(job *batchv1.Job) error {
	rId, ok := job.Labels[labelKeyRepositoryId]
	if !ok {
		return nil
	}

	repoId, err := strconv.ParseInt(rId, 10, 32)
	if err != nil {
		logger.Log.Info("Could not parse label", zap.String(labelKeyRepositoryId, rId))
		return xerrors.Errorf(": %w", err)
	}

	d.mu.Lock()
	if job.Status.CompletionTime.IsZero() {
		d.cache[int32(repoId)] = struct{}{}
	} else {
		delete(d.cache, int32(repoId))
	}
	d.mu.Unlock()

	return nil
}
