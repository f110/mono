package set

import (
	"sync"
)

type Set struct {
	set
	rw sync.RWMutex
}

type set map[any]struct{}

func New(values ...any) *Set {
	s := &Set{set: make(set)}
	for _, v := range values {
		s.Add(v)
	}

	return s
}

func (s *Set) Add(v any) {
	s.rw.Lock()
	s.set[v] = struct{}{}
	s.rw.Unlock()
}

func (s *Set) ToSlice() []any {
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

func (s *Set) Has(v any) bool {
	s.rw.RLock()
	_, ok := s.set[v]
	s.rw.RUnlock()

	return ok
}

func (s *Set) Union(v *Set) {
	s.rw.Lock()
	v.rw.RLock()
	for k := range v.set {
		s.set[k] = struct{}{}
	}
	v.rw.RUnlock()
	s.rw.Unlock()
}

func (s *Set) Join(right *Set) *Set {
	d := New()
	s.rw.RLock()
	for k := range s.set {
		if right.Has(k) {
			d.Add(k)
		}
	}
	s.rw.RUnlock()

	return d
}

func (s *Set) LeftOuter(right *Set) *Set {
	return s.Diff(right)
}

func (s *Set) RightOuter(right *Set) *Set {
	return right.Diff(s)
}
