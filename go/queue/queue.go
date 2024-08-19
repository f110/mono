package queue

import (
	"go.f110.dev/mono/go/list"
)

// Simple is a fifo queue
type Simple[T any] struct {
	q            *list.List[T]
	s            chan struct{}
	shuttingDown bool
}

func NewSimple[T any]() *Simple[T] {
	return &Simple[T]{q: list.NewDoubleLinked[T](), s: make(chan struct{})}
}

func (q *Simple[T]) Enqueue(v T) {
	if q.shuttingDown {
		return
	}

	q.q.PushBack(v)

	select {
	case q.s <- struct{}{}:
	default:
	}
}

func (q *Simple[T]) Dequeue() *T {
	v := q.q.Front()
	if v == nil {
		_, ok := <-q.s
		if !ok {
			return nil
		}

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
	close(q.s)
}
