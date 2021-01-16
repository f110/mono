package controllerutil

import (
	"sync"

	"k8s.io/client-go/util/workqueue"
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
		}
		q.ch <- item
	}
}
