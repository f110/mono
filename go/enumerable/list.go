package enumerable

func IsInclude[T comparable](ary []T, val T) bool {
	for _, v := range ary {
		if v == val {
			return true
		}
	}

	return false
}
