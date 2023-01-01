package controllerutil

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/parallel"
	"go.f110.dev/mono/go/pkg/k8s/client"
)

type Controller interface {
	ObjectToKeys(obj interface{}) []string
	GetObject(key string) (runtime.Object, error)
	UpdateObject(ctx context.Context, obj runtime.Object) (runtime.Object, error)
	Reconcile(ctx context.Context, obj runtime.Object) error
	Finalize(ctx context.Context, obj runtime.Object) error
}

type Reconciler interface {
	Reconcile(ctx context.Context, obj runtime.Object) error
	Finalize(ctx context.Context, obj runtime.Object) error
}

type ControllerBase struct {
	queue      *WorkQueue
	supervisor *parallel.Supervisor
	recorder   record.EventRecorder
	log        *zap.Logger

	impl        Controller
	reconciler  Reconciler
	eventSource []cache.SharedIndexInformer
	informers   []cache.SharedIndexInformer
	finalizers  []string
}

func NewBase(
	name string,
	v Controller,
	coreClient kubernetes.Interface,
	eventSource []cache.SharedIndexInformer,
	informers []cache.SharedIndexInformer,
	finalizers []string,
) *ControllerBase {
	logger := logger.Log.Named(name)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(func(format string, args ...interface{}) {
		logger.Info(fmt.Sprintf(format, args...))
	})
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: coreClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(client.Scheme, corev1.EventSource{Component: name})

	var r Reconciler
	if fn, ok := v.(interface {
		NewReconciler(log *zap.Logger) Reconciler
	}); ok {
		r = fn.NewReconciler(logger)
	}

	return &ControllerBase{
		queue:       NewWorkQueue(name),
		recorder:    recorder,
		log:         logger,
		impl:        v,
		reconciler:  r,
		eventSource: eventSource,
		informers:   informers,
		finalizers:  finalizers,
	}
}

func (b *ControllerBase) StartWorkers(ctx context.Context, workers int) {
	hasSynced := make([]cache.InformerSynced, 0)
	for _, v := range b.informers {
		hasSynced = append(hasSynced, v.HasSynced)
	}
	for _, v := range b.eventSource {
		hasSynced = append(hasSynced, v.HasSynced)

		v.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    b.onAdd,
			UpdateFunc: b.onUpdate,
			DeleteFunc: b.onDelete,
		})
	}

	b.log.Info("Wait to sync all informers cache")
	if !b.WaitForSync(ctx) {
		return
	}

	b.supervisor = parallel.NewSupervisor(ctx)
	b.supervisor.Log = b.log
	for i := 0; i < workers; i++ {
		b.supervisor.Add(b.worker)
	}
}

func (b *ControllerBase) Shutdown() {
	b.queue.Shutdown()

	if b.supervisor != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		b.supervisor.Shutdown(ctx)
		cancel()
	}
}

func (b *ControllerBase) WaitForSync(ctx context.Context) bool {
	hasSynced := make([]cache.InformerSynced, 0)
	for _, v := range b.informers {
		hasSynced = append(hasSynced, v.HasSynced)
	}
	for _, v := range b.eventSource {
		hasSynced = append(hasSynced, v.HasSynced)
	}

	return cache.WaitForCacheSync(ctx.Done(), hasSynced...)
}

func (b *ControllerBase) EventRecorder() record.EventRecorder {
	return b.recorder
}

func (b *ControllerBase) Log() *zap.Logger {
	return b.log
}

func (b *ControllerBase) worker(ctx context.Context) {
	for {
		var obj interface{}
		select {
		case v, ok := <-b.queue.Get():
			if !ok {
				return
			}
			obj = v
		}
		b.log.Debug("Get next queue", zap.Any("queue", obj))

		err := b.process(obj.(string))
		if err != nil {
			b.log.Info("Failed sync", zap.String("key", obj.(string)), zap.Error(err))
		}
	}
}

func (b *ControllerBase) process(key string) error {
	defer b.queue.Done(key)

	ctx, cancelFunc := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFunc()
	obj, err := b.impl.GetObject(key)
	if err != nil {
		return err
	}
	if obj == nil {
		return nil
	}

	objMeta, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	if len(b.finalizers) > 0 {
		if objMeta.GetDeletionTimestamp().IsZero() {
			for _, finalizer := range b.finalizers {
				if !containsString(objMeta.GetFinalizers(), finalizer) {
					objMeta.SetFinalizers(append(objMeta.GetFinalizers(), finalizer))
					if v, err := b.impl.UpdateObject(ctx, obj); err != nil {
						return err
					} else {
						obj = v
					}
				}
			}
		}
	}

	if objMeta.GetDeletionTimestamp().IsZero() {
		if b.reconciler != nil {
			err = b.reconciler.Reconcile(ctx, obj)
		} else {
			err = b.impl.Reconcile(ctx, obj)
		}
	} else {
		if b.reconciler != nil {
			err = b.reconciler.Finalize(ctx, obj)
		} else {
			err = b.impl.Finalize(ctx, obj)
		}
	}
	if err != nil {
		if errors.Is(err, &RetryError{}) {
			b.queue.AddRateLimited(key)
			return nil
		}

		return err
	}

	b.queue.Forget(key)
	return nil
}

func (b *ControllerBase) onAdd(obj interface{}) {
	b.enqueue(obj)
}

func (b *ControllerBase) onUpdate(old, cur interface{}) {
	oldObj, err := meta.Accessor(old)
	if err != nil {
		return
	}
	curObj, err := meta.Accessor(cur)
	if err != nil {
		return
	}

	if oldObj.GetUID() != curObj.GetUID() {
		if key, err := cache.MetaNamespaceKeyFunc(oldObj); err != nil {
			return
		} else {
			b.onDelete(cache.DeletedFinalStateUnknown{Key: key, Obj: oldObj})
		}
	}

	b.enqueue(cur)
}

func (b *ControllerBase) onDelete(obj interface{}) {
	dfsu, ok := obj.(cache.DeletedFinalStateUnknown)
	if ok {
		b.enqueue(dfsu.Key)
		return
	}

	b.enqueue(obj)
}

func (b *ControllerBase) enqueue(obj interface{}) {
	keys := b.impl.ObjectToKeys(obj)
	for _, v := range keys {
		if v == "" {
			continue
		}
		b.queue.Add(v)
	}
}

func containsString(v []string, s string) bool {
	for _, item := range v {
		if item == s {
			return true
		}
	}

	return false
}
