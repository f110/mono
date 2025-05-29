package set

import (
	"sync"
)

type Set[T comparable] struct {
	set[T]
	rw sync.RWMutex
}

type set[T comparable] map[T]struct{}

func New[T comparable](values ...T) *Set[T] {
	s := &Set[T]{set: make(set[T])}
	for _, v := range values {
		s.Add(v)
	}

	return s
}

func (s *Set[T]) Add(v T) {
	s.rw.Lock()
	s.set[v] = struct{}{}
	s.rw.Unlock()
}

func (s *Set[T]) ToSlice() []T {
	v := make([]T, 0, len(s.set))
	s.rw.RLock()
	for k, _ := range s.set {
		v = append(v, k)
	}
	s.rw.RUnlock()

	return v
}

func (s *Set[T]) Diff(other *Set[T]) *Set[T] {
	d := New[T]()
	s.rw.RLock()
	for k := range s.set {
		if !other.Has(k) {
			d.Add(k)
		}
	}
	s.rw.RUnlock()

	return d
}

func (s *Set[T]) Has(v T) bool {
	s.rw.RLock()
	_, ok := s.set[v]
	s.rw.RUnlock()

	return ok
}

func (s *Set[T]) Union(v *Set[T]) {
	s.rw.Lock()
	v.rw.RLock()
	for k := range v.set {
		s.set[k] = struct{}{}
	}
	v.rw.RUnlock()
	s.rw.Unlock()
}

func (s *Set[T]) Join(right *Set[T]) *Set[T] {
	d := New[T]()
	s.rw.RLock()
	for k := range s.set {
		if right.Has(k) {
			d.Add(k)
		}
	}
	s.rw.RUnlock()

	return d
}

func (s *Set[T]) LeftOuter(right *Set[T]) *Set[T] {
	return s.Diff(right)
}

func (s *Set[T]) RightOuter(right *Set[T]) *Set[T] {
	return right.Diff(s)
}
