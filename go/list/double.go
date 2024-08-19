package list

import (
	"iter"
)

type List[T any] struct {
	root *Element[T]
	len  int
}

type Element[T any] struct {
	next *Element[T]
	prev *Element[T]
	list *List[T]

	Value T
}

func (e *Element[T]) Next() *Element[T] {
	if p := e.next; e.list != nil && p != e.list.root {
		return p
	}
	return nil
}

func (e *Element[T]) Prev() *Element[T] {
	if p := e.prev; e.list != nil && p != e.list.root {
		return p
	}
	return nil
}

func NewDoubleLinked[T any]() *List[T] {
	e := &Element[T]{}
	e.next = e
	e.prev = e
	return &List[T]{root: e, len: 0}
}

func (l *List[T]) Len() int { return l.len }

func (l *List[T]) Iter() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		i := 0
		e := l.Front()
		for {
			if !yield(i, e.Value) {
				return
			}
			e = e.Next()
			i++
		}
	}
}

func (l *List[T]) Front() *Element[T] {
	if l.len == 0 {
		return nil
	}
	return l.root.next
}

func (l *List[T]) Back() *Element[T] {
	if l.len == 0 {
		return nil
	}
	return l.root.prev
}

func (l *List[T]) PushFront(v T) *Element[T] {
	return l.insert(v, l.root)
}

func (l *List[T]) PushBack(v T) *Element[T] {
	return l.insert(v, l.root.prev)
}

func (l *List[T]) Remove(e *Element[T]) {
	if e.list == l {
		l.remove(e)
	}
}

func (l *List[T]) insert(v T, at *Element[T]) *Element[T] {
	e := &Element[T]{
		Value: v,
		prev:  at,
		next:  at.next,
		list:  l,
	}
	at.next = e
	e.next.prev = e
	l.len++
	return e
}

func (l *List[T]) remove(e *Element[T]) {
	e.prev.next = e.next
	e.next.prev = e.prev
	e.next = nil
	e.prev = nil
	e.list = nil
	l.len--
}
