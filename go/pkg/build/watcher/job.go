package watcher

import (
	"context"
	"time"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	batchv1informers "k8s.io/client-go/informers/batch/v1"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"go.f110.dev/mono/go/pkg/logger"
)

const (
	TypeLabel = "build.f110.dev/type"
)

var Router = newRouter()

type router struct {
	typeMap map[string]func(*batchv1.Job) error
}

func newRouter() *router {
	return &router{typeMap: make(map[string]func(*batchv1.Job) error)}
}

func (r *router) Add(jobType string, f func(*batchv1.Job) error) {
	r.typeMap[jobType] = f
}

func (r *router) Dispatch(jobType string, job *batchv1.Job) error {
	if f, ok := r.typeMap[jobType]; ok {
		return f(job)
	}

	return nil
}

type JobWatcher struct {
	jobInformer batchv1informers.JobInformer
	jobLister   batchv1listers.JobLister
	queue       workqueue.RateLimitingInterface
}

func NewJobWatcher(jobInformer batchv1informers.JobInformer) *JobWatcher {
	j := &JobWatcher{
		jobInformer: jobInformer,
		jobLister:   jobInformer.Lister(),
		queue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "job-watcher"),
	}

	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    j.addJob,
		UpdateFunc: j.updateJob,
		DeleteFunc: j.deleteJob,
	})

	return j
}

func (j *JobWatcher) Run(ctx context.Context, workers int) error {
	if !cache.WaitForCacheSync(ctx.Done(), j.jobInformer.Informer().HasSynced) {
		return xerrors.New("failed to sync informer's cache")
	}

	for i := 0; i < workers; i++ {
		go wait.Until(j.worker, time.Second, ctx.Done())
	}

	select {
	case <-ctx.Done():
		break
	}

	j.queue.ShutDown()

	return nil
}

func (j JobWatcher) dispatch(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return xerrors.WithStack(err)
	}

	job, err := j.jobLister.Jobs(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return xerrors.WithStack(err)
	}

	jobType, ok := job.Labels[TypeLabel]
	if !ok {
		return nil
	}

	return Router.Dispatch(jobType, job)
}

func (j *JobWatcher) worker() {
	defer logger.Log.Debug("Finish worker")

	for j.processNextItem() {
	}
}

func (j *JobWatcher) processNextItem() bool {
	obj, shutdown := j.queue.Get()
	if shutdown {
		return false
	}
	logger.Log.Debug("Got next queue", zap.String("key", obj.(string)))

	func(obj interface{}) {
		defer j.queue.Done(obj)

		if err := j.dispatch(obj.(string)); err != nil {
			logger.Log.Info("syncJob returns an error", zap.Error(err))
			j.queue.AddRateLimited(obj)
			return
		}

		j.queue.Forget(obj)
		return
	}(obj)

	return true
}

func (j *JobWatcher) addJob(obj interface{}) {
	job := obj.(*batchv1.Job)

	if key, err := cache.MetaNamespaceKeyFunc(job); err != nil {
		return
	} else {
		j.queue.Add(key)
	}
}

func (j *JobWatcher) updateJob(_, cur interface{}) {
	job := cur.(*batchv1.Job)

	if key, err := cache.MetaNamespaceKeyFunc(job); err != nil {
		return
	} else {
		j.queue.Add(key)
	}
}

func (j *JobWatcher) deleteJob(obj interface{}) {
	job, ok := obj.(*batchv1.Job)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			logger.Log.Info("Object is not DeletedFinalStateUnknown")
			return
		}
		job, ok = tombstone.Obj.(*batchv1.Job)
		if !ok {
			logger.Log.Info("Object is DeletedFinalStateUnknown but Obj is not Pod")
			return
		}
	}

	if key, err := cache.MetaNamespaceKeyFunc(job); err != nil {
		return
	} else {
		j.queue.Add(key)
	}
}
