package assertion

import (
	"reflect"
	"testing"
)

func MustNoError(t testing.TB, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("Received unexpected error:\n%+v", err)
	}
}

func MustError(t testing.TB, err error) {
	t.Helper()

	if err == nil {
		t.Fatalf("An error is expected but got nil")
	}
}

func MustLen(t testing.TB, object any, length int) {
	t.Helper()

	v := reflect.ValueOf(object)
	if v.Len() != length {
		t.Fatalf("\"%v\" should have %d item(s), but has %d", object, length, v.Len())
	}
}
