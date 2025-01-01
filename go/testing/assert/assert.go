package assert

import (
	"reflect"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

type TestingT interface {
	Errorf(string, ...any)
	Helper()
}

type Option func(any) cmp.Option

func IgnoreFields(fields ...string) Option {
	return func(v any) cmp.Option {
		if typ := reflect.TypeOf(v); typ.Kind() == reflect.Ptr {
			return cmpopts.IgnoreFields(reflect.ValueOf(v).Elem().Interface(), fields...)
		}
		return cmpopts.IgnoreFields(v, fields...)
	}
}

func Equal[T any](t TestingT, actual, expected T, opts ...Option) {
	t.Helper()

	cOpts := make([]cmp.Option, len(opts))
	for i, v := range opts {
		cOpts[i] = v(expected)
	}

	if !cmp.Equal(expected, actual, cOpts...) {
		d := cmp.Diff(expected, actual, cOpts...)
		t.Errorf("Not equal: \nexpected: %#v\nactual  : %#v\n%s", expected, actual, d)
	}
}

func NotEqual[T any](t TestingT, actual, expected T, opts ...Option) {
	t.Helper()

	cOpts := make([]cmp.Option, len(opts))
	for i, v := range opts {
		cOpts[i] = v(expected)
	}

	if cmp.Equal(expected, actual, cOpts...) {
		t.Errorf("Should not be: %#v\n", expected)
	}
}

func Len(t TestingT, object any, length int) {
	t.Helper()

	v := reflect.ValueOf(object)
	if v.Len() != length {
		t.Errorf("\"%v\" should have %d item(s), but has %d", object, length, v.Len())

	}
}

func Contains(t TestingT, object, contain any) {
	t.Helper()

	c, ok := contains(object, contain)
	if !ok {
		t.Errorf("%#v could not be applied", object)
	}
	if !c {
		t.Errorf("%#v does not contain %#v", object, contain)
	}
}

func NotContains(t TestingT, object, contain any) {
	t.Helper()

	c, ok := contains(object, contain)
	if !ok {
		t.Errorf("%#v could not be applied", object)
	}
	if c {
		t.Errorf("%#v should not contain %#v", object, contain)
	}
}

func NoError(t TestingT, err error) {
	t.Helper()

	if err != nil {
		t.Errorf("Received unexpected error:\n%+v", err)
	}
}

func Error(t TestingT, err error) {
	t.Helper()

	if err == nil {
		t.Errorf("An error is expected but got nil")
	}
}

func True(t TestingT, v bool) {
	t.Helper()

	if !v {
		t.Errorf("Should be true")
	}
}

func False(t TestingT, v bool) {
	t.Helper()

	if v {
		t.Errorf("Should be false")
	}
}

func Nil(t TestingT, v any) {
	t.Helper()

	if !isNil(v) {
		t.Errorf("Expected nil, but got: %#v", v)
	}
}

func NotNil(t TestingT, v any) {
	t.Helper()

	if isNil(v) {
		t.Errorf("Expected value not to be nil")
	}
}

func Greater[T any](t TestingT, v1, v2 T) {
	t.Helper()

	vo1 := reflect.ValueOf(v1)
	vo2 := reflect.ValueOf(v2)
	if vo1.Kind() != vo2.Kind() {
		t.Errorf("Elements should be the same: \nv1: %#v\nv2: %#v", v1, v2)
		return
	}

	switch compare(vo1, vo2) {
	case 1:
	case 0, -1:
		t.Errorf("%v is not greater than %v", vo1, vo2)
	}
}

func GreaterOrEqual[T any](t TestingT, v1, v2 T) {
	t.Helper()

	vo1 := reflect.ValueOf(v1)
	vo2 := reflect.ValueOf(v2)
	if vo1.Kind() != vo2.Kind() {
		t.Errorf("Elements should be the same: \nv1: %#v\nv2: %#v", v1, v2)
	}

	switch compare(vo1, vo2) {
	case 0, 1:
	case -1:
		t.Errorf("%v is not greater than %v", vo1, vo2)
	}
}

func Less[T any](t TestingT, v1, v2 T) {
	t.Helper()

	vo1 := reflect.ValueOf(v1)
	vo2 := reflect.ValueOf(v2)
	if vo1.Kind() != vo2.Kind() {
		t.Errorf("Elements should be the same: \nv1: %#v\nv2: %#v", v1, v2)
	}

	switch compare(vo1, vo2) {
	case -1:
	case 0, 1:
		t.Errorf("%s is not less than %s", vo1, vo2)
	}
}

func LessOrEqual[T any](t TestingT, v1, v2 T) {
	t.Helper()

	vo1 := reflect.ValueOf(v1)
	vo2 := reflect.ValueOf(v2)
	if vo1.Kind() != vo2.Kind() {
		t.Errorf("Elements should be the same: \nv1: %#v\nv2: %#v", v1, v2)
	}

	switch compare(vo1, vo2) {
	case -1, 0:
	case 1:
		t.Errorf("%s is not less than %s", vo1, vo2)
	}
}

func Empty(t TestingT, v any) {
	t.Helper()

	if !isEmpty(v) {
		t.Errorf("Should be empty, but was %v", v)
	}
}

func NotEmpty(t TestingT, v any) {
	t.Helper()

	if isEmpty(v) {
		t.Errorf("Should not be empty, but was %v", v)
	}
}

func compare(v1, v2 reflect.Value) int {
	switch v1.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		iv1 := v1.Int()
		iv2 := v2.Int()

		if iv1 < iv2 {
			return -1
		} else if iv1 == iv2 {
			return 0
		}
		return 1
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		iv1 := v1.Uint()
		iv2 := v2.Uint()

		if iv1 < iv2 {
			return -1
		} else if iv1 == iv2 {
			return 0
		}
		return 1
	case reflect.Float32, reflect.Float64:
		fv1 := v1.Float()
		fv2 := v2.Float()

		if fv1 < fv2 {
			return -1
		} else if fv1 == fv2 {
			return 0
		}
		return 1
	}

	return -1
}

func isNil(v any) bool {
	if v == nil {
		return true
	}

	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Slice, reflect.Ptr:
		return val.IsNil()
	}

	return false
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}

	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Map, reflect.Slice:
		return val.Len() == 0
	default:
		z := reflect.Zero(val.Type())
		return reflect.DeepEqual(v, z.Interface())
	}
}

func contains(object, contain any) (bool, bool) {
	typ := reflect.TypeOf(object)
	val := reflect.ValueOf(object)

	switch typ.Kind() {
	case reflect.String:
		elm := reflect.ValueOf(contain)
		return strings.Contains(val.String(), elm.String()), true
	case reflect.Map:
		keys := val.MapKeys()
		found := false
		for i := 0; i < len(keys); i++ {
			key := keys[i]
			if key.Type().Comparable() && key.Interface() == contain {
				found = true
				break
			}
		}
		return found, true
	case reflect.Slice:
		found := false
		for i := 0; i < val.Len(); i++ {
			v := val.Index(i)
			if v.Type().Comparable() && v.Interface() == contain {
				found = true
				break
			}
		}
		return found, true
	default:
		return false, false
	}
}
