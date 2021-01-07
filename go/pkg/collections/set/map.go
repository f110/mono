package set

import (
	"sync"
)

type Set struct {
	set
	rw sync.RWMutex
}

type set map[interface{}]struct{}

func New(values ...interface{}) *Set {
	s := &Set{set: make(set)}
	for _, v := range values {
		s.Add(v)
	}

	return s
}

func (s *Set) Add(v interface{}) {
	s.rw.Lock()
	s.set[v] = struct{}{}
	s.rw.Unlock()
}

func (s *Set) ToSlice() []interface{} {
	v := make([]interface{}, 0, len(s.set))
	s.rw.RLock()
	for k, _ := range s.set {
		v = append(v, k)
	}
	s.rw.RUnlock()

	return v
}

func (s *Set) Diff(other *Set) *Set {
	d := New()
	s.rw.RLock()
	for k := range s.set {
		if !other.Has(k) {
			d.Add(k)
		}
	}
	s.rw.RUnlock()

	return d
}

func (s *Set) Has(v interface{}) bool {
	s.rw.RLock()
	_, ok := s.set[v]
	s.rw.RUnlock()

	return ok
}
