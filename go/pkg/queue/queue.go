package queue

import (
	"container/list"
)

// Simple is a fifo queue
type Simple struct {
	q            *list.List
	s            chan struct{}
	shuttingDown bool
}

func NewSimple() *Simple {
	return &Simple{q: list.New(), s: make(chan struct{})}
}

func (q *Simple) Enqueue(v any) {
	if q.shuttingDown {
		return
	}

	q.q.PushBack(v)

	select {
	case q.s <- struct{}{}:
	default:
	}
}

func (q *Simple) Dequeue() any {
	v := q.q.Front()
	if v == nil {
		_, ok := <-q.s
		if !ok {
			return nil
		}
		
		v = q.q.Front()
	}
	q.q.Remove(v)
	return v.Value
}

func (q *Simple) Shutdown() {
	if q.shuttingDown {
		return
	}

	q.shuttingDown = true
	close(q.s)
}
