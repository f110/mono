package controllerutil

import (
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	_ "k8s.io/component-base/metrics/prometheus/workqueue"
)

type WorkQueue struct {
	ch        chan interface{}
	closeOnce sync.Once
	queue     workqueue.RateLimitingInterface
}

func NewWorkQueue(name string) *WorkQueue {
	q := &WorkQueue{
		ch:    make(chan interface{}),
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
	}
	go q.run()

	return q
}

func (q *WorkQueue) Get() <-chan interface{} {
	return q.ch
}

func (q *WorkQueue) Add(item interface{}) {
	q.queue.Add(item)
}

func (q *WorkQueue) AddRateLimited(item interface{}) {
	q.queue.AddRateLimited(item)
}

func (q *WorkQueue) Forget(item interface{}) {
	q.queue.Forget(item)
}

func (q *WorkQueue) Done(item interface{}) {
	q.queue.Done(item)
}

func (q *WorkQueue) Shutdown() {
	q.queue.ShutDown()
}

func (q *WorkQueue) run() {
	for {
		item, shutdown := q.queue.Get()
		if shutdown {
			q.closeOnce.Do(func() {
				close(q.ch)
			})
			return
		}
		q.ch <- item
	}
}

type GenericWorkQueue[T runtime.Object] struct {
	ch        chan T
	closeOnce sync.Once
	queue     workqueue.RateLimitingInterface
}

func NewGenericWorkQueue[T runtime.Object](name string) *GenericWorkQueue[T] {
	q := &GenericWorkQueue[T]{
		ch:    make(chan T),
		queue: workqueue.NewRateLimitingQueueWithConfig(workqueue.DefaultControllerRateLimiter(), workqueue.RateLimitingQueueConfig{Name: name}),
	}
	go q.run()

	return q
}

func (q *GenericWorkQueue[T]) Get() <-chan T {
	return q.ch
}

func (q *GenericWorkQueue[T]) Add(item T) {
	q.queue.Add(item)
}

func (q *GenericWorkQueue[T]) AddRateLimited(item T) {
	q.queue.AddRateLimited(item)
}

func (q *GenericWorkQueue[T]) Forget(item T) {
	q.queue.Forget(item)
}

func (q *GenericWorkQueue[T]) Done(item T) {
	q.queue.Done(item)
}

func (q *GenericWorkQueue[T]) Shutdown() {
	q.queue.ShutDown()
}

func (q *GenericWorkQueue[T]) run() {
	for {
		item, shutdown := q.queue.Get()
		if shutdown {
			q.closeOnce.Do(func() {
				close(q.ch)
			})
			return
		}
		q.ch <- item.(T)
	}
}
