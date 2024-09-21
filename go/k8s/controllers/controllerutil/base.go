package controllerutil

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/k8s/client"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/parallel"
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
	for range workers {
		b.supervisor.Add(b.worker)
	}
}

func (b *ControllerBase) Shutdown() {
	b.queue.Shutdown()

	if b.supervisor != nil {
		ctx, cancel := ctxutil.WithTimeout(context.Background(), 30*time.Second)
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

	ctx, cancelFunc := ctxutil.WithTimeout(context.Background(), 30*time.Second)
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

type GenericReconciler[T runtime.Object] interface {
	Reconcile(ctx context.Context, obj T) error
	Finalize(ctx context.Context, obj T) error
}

type GenericControllerBase[T runtime.Object] struct {
	log            *zap.Logger
	recorder       record.EventRecorder
	queue          *WorkQueue
	supervisor     *parallel.Supervisor
	informers      []cache.SharedIndexInformer
	eventSource    []cache.SharedIndexInformer
	finalizers     []string
	getObjectFn    func(namespace, name string) (T, error)
	updateObjectFn func(context.Context, T, metav1.UpdateOptions) (T, error)
	newReconciler  func() GenericReconciler[T]
}

func NewGenericControllerBase[T runtime.Object](
	name string,
	newReconciler func() GenericReconciler[T],
	coreClient kubernetes.Interface,
	eventSource []cache.SharedIndexInformer,
	informers []cache.SharedIndexInformer,
	finalizers []string,
	getObjectFn func(namespace, name string) (T, error),
	updateObjectFn func(context.Context, T, metav1.UpdateOptions) (T, error),
) *GenericControllerBase[T] {
	l := logger.Log.Named(name)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(func(format string, args ...interface{}) {
		l.Info(fmt.Sprintf(format, args...))
	})
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: coreClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(client.Scheme, corev1.EventSource{Component: name})

	return &GenericControllerBase[T]{
		log:            l,
		recorder:       recorder,
		queue:          NewWorkQueue(name),
		eventSource:    eventSource,
		informers:      informers,
		finalizers:     finalizers,
		newReconciler:  newReconciler,
		getObjectFn:    getObjectFn,
		updateObjectFn: updateObjectFn,
	}
}

func (b *GenericControllerBase[T]) StartWorkers(ctx context.Context, workers int) {
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
	for range workers {
		b.supervisor.Add(b.worker)
	}
}

func (b *GenericControllerBase[T]) Shutdown() {
	b.queue.Shutdown()

	if b.supervisor != nil {
		ctx, cancel := ctxutil.WithTimeout(context.Background(), 30*time.Second)
		b.supervisor.Shutdown(ctx)
		cancel()
	}
}

func (b *GenericControllerBase[T]) WaitForSync(ctx context.Context) bool {
	hasSynced := make([]cache.InformerSynced, 0)
	for _, v := range b.informers {
		hasSynced = append(hasSynced, v.HasSynced)
	}
	for _, v := range b.eventSource {
		hasSynced = append(hasSynced, v.HasSynced)
	}

	return cache.WaitForCacheSync(ctx.Done(), hasSynced...)
}

func (b *GenericControllerBase[T]) EventRecorder() record.EventRecorder {
	return b.recorder
}

func (b *GenericControllerBase[T]) Log() *zap.Logger {
	return b.log
}

func (b *GenericControllerBase[T]) worker(ctx context.Context) {
	for {
		var key string
		select {
		case v, ok := <-b.queue.Get():
			if !ok {
				return
			}
			key = v.(string)
		}
		b.log.Debug("Get next queue", zap.String("key", key))

		err := b.process(ctx, key)
		if err != nil {
			b.log.Info("Failed sync", zap.String("key", key), zap.Error(err))
		}
		b.log.Debug("Finished process", zap.String("key", key))
	}
}

func (b *GenericControllerBase[T]) process(workerCtx context.Context, key string) error {
	defer b.queue.Done(key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return xerrors.WithStack(err)
	}
	obj, err := b.getObjectFn(namespace, name)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	ctx, cancelFunc := ctxutil.WithTimeout(workerCtx, 30*time.Second)
	defer cancelFunc()

	target := obj.DeepCopyObject().(T)
	objMeta, err := meta.Accessor(target)
	if err != nil {
		return err
	}
	if objMeta.GetDeletionTimestamp().IsZero() {
		var updatedFinalizers bool
		for _, finalizer := range b.finalizers {
			if !containsString(objMeta.GetFinalizers(), finalizer) {
				updatedFinalizers = true
				objMeta.SetFinalizers(append(objMeta.GetFinalizers(), finalizer))
			}
		}
		if updatedFinalizers {
			if _, err := b.updateObjectFn(ctx, target, metav1.UpdateOptions{}); err != nil {
				return WrapRetryError(xerrors.WithStack(err))
			}
			return nil
		}
	} else {
		var finalizing bool
		for _, finalizer := range b.finalizers {
			if containsString(objMeta.GetFinalizers(), finalizer) {
				finalizing = true
				break
			}
		}
		if !finalizing {
			logger.Log.Debug("Skip finalize because all finalizers are removed", zap.String("key", key))
			return nil
		}
	}

	reconciler := b.newReconciler()
	if objMeta.GetDeletionTimestamp().IsZero() {
		err = reconciler.Reconcile(ctx, target)
	} else {
		err = reconciler.Finalize(ctx, target)
	}
	if err != nil {
		if errors.Is(err, &RetryError{}) {
			logger.Log.Debug("Retry queue", zap.Error(err), zap.String("name", objMeta.GetName()), zap.String("namespace", objMeta.GetNamespace()))
			b.queue.AddRateLimited(obj)
			return nil
		}

		return err
	}

	b.queue.Forget(obj)
	return nil
}

func (b *GenericControllerBase[T]) onAdd(obj any) {
	b.enqueue(obj)
}

func (b *GenericControllerBase[T]) onUpdate(old, cur any) {
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

func (b *GenericControllerBase[T]) onDelete(obj any) {
	dfsu, ok := obj.(cache.DeletedFinalStateUnknown)
	if ok {
		b.enqueue(dfsu.Key)
		return
	}

	b.enqueue(obj)
}

func (b *GenericControllerBase[T]) enqueue(obj any) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	b.Log().Debug("Enqueue", zap.String("key", key))
	b.queue.Add(key)
}
