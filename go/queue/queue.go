package queue

import (
	"sync"

	"go.f110.dev/mono/go/list"
)

// Simple is a fifo queue
type Simple[T any] struct {
	q            *list.List[T]
	mu           sync.Mutex
	cond         *sync.Cond
	shuttingDown bool
}

func NewSimple[T any]() *Simple[T] {
	q := &Simple[T]{q: list.NewDoubleLinked[T]()}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *Simple[T]) Enqueue(v T) {
	if q.shuttingDown {
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	q.q.PushBack(v)
	q.cond.Signal()
}

func (q *Simple[T]) Dequeue() *T {
	q.mu.Lock()
	defer q.mu.Unlock()
	v := q.q.Front()
	if v == nil {
		q.cond.Wait()

		v = q.q.Front()
	}
	q.q.Remove(v)
	return &v.Value
}

func (q *Simple[T]) Shutdown() {
	if q.shuttingDown {
		return
	}

	q.shuttingDown = true
}
