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
