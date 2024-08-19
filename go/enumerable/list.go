package enumerable

import (
	"iter"
)

func IsInclude[T comparable](ary []T, val T) bool {
	for _, v := range ary {
		if v == val {
			return true
		}
	}

	return false
}

func Delete[T comparable](ary []T, val T) []T {
	n := make([]T, 0, len(ary))
	for _, v := range ary {
		if v == val {
			continue
		}
		n = append(n, v)
	}
	return n
}

func Sum[T any](ary []T, m func(T) int64) int64 {
	var t int64
	for _, v := range ary {
		t += m(v)
	}
	return t
}

func Uniq[T, K comparable](ary []T, f func(T) K) []T {
	m := make(map[K]struct{})
	for _, v := range ary {
		key := f(v)
		m[key] = struct{}{}
	}

	n := make([]T, 0, len(m))
	for _, v := range ary {
		key := f(v)
		if _, ok := m[key]; ok {
			n = append(n, v)
			delete(m, key)
		}
	}
	return n
}

func Map[T, K any](ary []T, f func(T) K) []K {
	n := make([]K, len(ary))
	for i := range ary {
		n[i] = f(ary[i])
	}
	return n
}

func CollectFunc[T, K any](seq iter.Seq[T], f func(T) K) iter.Seq[K] {
	return func(yield func(K) bool) {
		for v := range seq {
			if !yield(f(v)) {
				return
			}
		}
	}
}

func InsertBefore[T comparable](s []T, before T, v ...T) []T {
	for i := range s {
		if s[i] == before {
			n := make([]T, len(s)+len(v))
			copy(n, s[:i])
			copy(n[i:], v)
			copy(n[i+len(v):], s[i:])
			return n
		}
	}
	return s
}

func FindAll[T comparable](ary []T, f func(T) bool) []T {
	n := make([]T, 0)
	for _, v := range ary {
		if f(v) {
			n = append(n, v)
		}
	}
	return n
}

func Index[T any](ary []T, f func(T) bool) int {
	for i, v := range ary {
		if f(v) {
			return i
		}
	}
	return -1
}
