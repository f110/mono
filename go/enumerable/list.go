package enumerable

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
