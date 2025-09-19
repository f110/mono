package assertion

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

type object struct {
	ID        int
	CreatedAt time.Time
}

func TestEqual(t *testing.T) {
	actual := &object{
		ID:        1,
		CreatedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	expected := &object{
		ID:        1,
		CreatedAt: time.Date(2020, 1, 1, 0, 0, 0, 1, time.UTC),
	}

	mockT := &mockTesting{}
	Equal(mockT, actual, expected)
	if !mockT.failed {
		t.Error("expected failed")
	}
	if testing.Verbose() {
		t.Log(mockT.msg)
	}

	mockT = &mockTesting{}
	Equal(mockT, actual, expected, IgnoreFields("CreatedAt"))
	if mockT.failed {
		t.Error("expected success")
	}
}

func TestNotEqual(t *testing.T) {
	actual := &object{
		ID: 1,
	}
	expected := &object{
		ID: 2,
	}

	mockT := &mockTesting{}
	NotEqual(mockT, actual, actual)
	if !mockT.failed {
		t.Error("expected failed")
	}
	if testing.Verbose() {
		t.Log(mockT.msg)
	}

	mockT = &mockTesting{}
	NotEqual(mockT, actual, expected)
	if mockT.failed {
		t.Error("expected success")
	}
}

func TestLen(t *testing.T) {
	mockT := &mockTesting{}
	Len(mockT, []int{1, 2, 3}, 2)
	if !mockT.failed {
		t.Error("expected failed")
	}
	if testing.Verbose() {
		t.Log(mockT.msg)
	}

	mockT = &mockTesting{}
	Len(mockT, []int{1, 2, 3}, 3)
	if mockT.failed {
		t.Error("expected success")
	}
}

func TestContains(t *testing.T) {
	cases := []struct {
		object   any
		contains any
		failed   bool
	}{
		{object: "Hello World", contains: "World"},
		{object: "Hello World", contains: "Foobar", failed: true},
		{object: []string{"Hello", "World"}, contains: "World"},
		{object: []string{"Hello", "World"}, contains: "Foobar", failed: true},
		{object: map[string]string{"Hello": "World"}, contains: "Hello"},
		{object: map[string]string{"Hello": "World"}, contains: "Foobar", failed: true},
	}

	for i := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			mockT := &mockTesting{}
			Contains(mockT, cases[i].object, cases[i].contains)
			if cases[i].failed {
				if !mockT.failed {
					t.Error("expected failed")
					t.Log(mockT.msg)
				}
			} else {
				if mockT.failed {
					t.Error("expected success")
					t.Log(mockT.msg)
				}
			}
		})
	}
}

func TestNotContains(t *testing.T) {
	cases := []struct {
		object   any
		contains any
		failed   bool
	}{
		{object: "Hello World", contains: "Foobar"},
		{object: "Hello World", contains: "Hello World", failed: true},
		{object: []string{"Hello", "World"}, contains: "Foobar"},
		{object: []string{"Hello", "World"}, contains: "Hello", failed: true},
		{object: map[string]string{"Hello": "World"}, contains: "Foobar"},
		{object: map[string]string{"Hello": "World"}, contains: "Hello", failed: true},
	}

	for i := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			mockT := &mockTesting{}
			NotContains(mockT, cases[i].object, cases[i].contains)
			if cases[i].failed {
				if !mockT.failed {
					t.Error("expected failed")
					t.Log(mockT.msg)
				}
			} else {
				if mockT.failed {
					t.Error("expected success")
					t.Log(mockT.msg)
				}
			}
		})
	}
}

func TestNoError(t *testing.T) {
	mockT := &mockTesting{}
	NoError(mockT, nil)
	if mockT.failed {
		t.Error("expected success")
	}

	mockT = &mockTesting{}
	NoError(mockT, errors.New("foo"))
	if !mockT.failed {
		t.Error("expected failed")
	}
}

func TestError(t *testing.T) {
	mockT := &mockTesting{}
	Error(mockT, errors.New("foo"))
	if mockT.failed {
		t.Error("expected success")
	}

	mockT = &mockTesting{}
	Error(mockT, nil)
	if !mockT.failed {
		t.Error("expected failed")
	}
}

func TestTrue(t *testing.T) {
	mockT := &mockTesting{}
	True(mockT, true)
	if mockT.failed {
		t.Error("expected success")
	}

	mockT = &mockTesting{}
	True(mockT, false)
	if !mockT.failed {
		t.Error("expected failed")
	}
}

func TestFalse(t *testing.T) {
	mockT := &mockTesting{}
	False(mockT, false)
	if mockT.failed {
		t.Error("expected success")
	}

	mockT = &mockTesting{}
	False(mockT, true)
	if !mockT.failed {
		t.Error("expected failed")
	}
}

func TestNil(t *testing.T) {
	cases := []struct {
		Value func() any
	}{
		{Value: func() any { return nil }},
		{Value: func() any { var ch chan struct{}; return ch }},
		{Value: func() any { var f func(); return f }},
		{Value: func() any { var i interface{}; return i }},
		{Value: func() any { var m map[string]any; return m }},
		{Value: func() any { var s []int; return s }},
		{Value: func() any { var s *string; return s }},
	}

	for i := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			mockT := &mockTesting{}
			Nil(mockT, cases[i].Value())
			if mockT.failed {
				t.Error("expected success")
			}
		})
	}

	mockT := &mockTesting{}
	Nil(mockT, mockT)
	if !mockT.failed {
		t.Error("expected failed")
	}
}

func TestNotNil(t *testing.T) {
	cases := []struct {
		Value func() any
	}{
		{Value: func() any { return nil }},
		{Value: func() any { var ch chan struct{}; return ch }},
		{Value: func() any { var f func(); return f }},
		{Value: func() any { var i interface{}; return i }},
		{Value: func() any { var m map[string]any; return m }},
		{Value: func() any { var s []int; return s }},
		{Value: func() any { var s *string; return s }},
	}

	for i := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			mockT := &mockTesting{}
			NotNil(mockT, cases[i].Value())
			if !mockT.failed {
				t.Error("expected failed")
			}
		})
	}

	mockT := &mockTesting{}
	NotNil(mockT, nil)
	if !mockT.failed {
		t.Error("expected failed")
	}
}

func TestGreater_GreaterOrEqual(t *testing.T) {
	cases := []struct {
		v1    any
		v2    any
		equal bool
	}{
		{v1: 2, v2: 1},
		{v1: -1, v2: -2},
		{v1: int8(2), v2: int8(0)},
		{v1: int8(-1), v2: int8(-2)},
		{v1: int16(2), v2: int16(0)},
		{v1: int16(-1), v2: int16(-2)},
		{v1: int32(2), v2: int32(0)},
		{v1: int32(-1), v2: int32(-2)},
		{v1: int64(2), v2: int64(0)},
		{v1: int64(-1), v2: int64(-2)},
		{v1: uint(2), v2: uint(1)},
		{v1: uint8(2), v2: uint8(0)},
		{v1: uint16(2), v2: uint16(0)},
		{v1: uint32(2), v2: uint32(0)},
		{v1: uint64(2), v2: uint64(0)},
		{v1: 0.5, v2: 0.1},
		{v1: float32(0.5), v2: float32(0.1)},
	}

	for i := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			mockT := &mockTesting{}
			Greater(mockT, cases[i].v1, cases[i].v2)
			if mockT.failed {
				t.Error("expected success")
			}

			mockT = &mockTesting{}
			Greater(mockT, cases[i].v2, cases[i].v1)
			if !mockT.failed {
				t.Error("expected failed")
			}

			mockT = &mockTesting{}
			Greater(mockT, cases[i].v1, cases[i].v1)
			if !mockT.failed {
				t.Error("expected failed")
			}

			mockT = &mockTesting{}
			GreaterOrEqual(mockT, cases[i].v1, cases[i].v2)
			if mockT.failed {
				t.Error("expected success")
			}

			mockT = &mockTesting{}
			GreaterOrEqual(mockT, cases[i].v2, cases[i].v1)
			if !mockT.failed {
				t.Error("expected failed")
			}

			mockT = &mockTesting{}
			GreaterOrEqual(mockT, cases[i].v1, cases[i].v1)
			if mockT.failed {
				t.Error("expected success")
			}
		})
	}
}

func TestLess_LessOrEqual(t *testing.T) {
	cases := []struct {
		v1    any
		v2    any
		equal bool
	}{
		{v1: 1, v2: 2},
		{v1: -2, v2: -1},
		{v1: int8(0), v2: int8(2)},
		{v1: int8(-2), v2: int8(-1)},
		{v1: int16(0), v2: int16(2)},
		{v1: int16(-2), v2: int16(-1)},
		{v1: int32(0), v2: int32(2)},
		{v1: int32(-2), v2: int32(-1)},
		{v1: int64(0), v2: int64(2)},
		{v1: int64(-2), v2: int64(-1)},
		{v1: uint(1), v2: uint(2)},
		{v1: uint8(0), v2: uint8(2)},
		{v1: uint16(0), v2: uint16(2)},
		{v1: uint32(0), v2: uint32(2)},
		{v1: uint64(0), v2: uint64(2)},
		{v1: 0.1, v2: 0.5},
		{v1: float32(0.1), v2: float32(0.5)},
	}

	for i := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			mockT := &mockTesting{}
			Less(mockT, cases[i].v1, cases[i].v2)
			if mockT.failed {
				t.Error("expected success")
			}

			mockT = &mockTesting{}
			Less(mockT, cases[i].v2, cases[i].v1)
			if !mockT.failed {
				t.Error("expected failed")
			}

			mockT = &mockTesting{}
			Less(mockT, cases[i].v1, cases[i].v1)
			if !mockT.failed {
				t.Error("expected failed")
			}

			mockT = &mockTesting{}
			LessOrEqual(mockT, cases[i].v1, cases[i].v2)
			if mockT.failed {
				t.Error("expected success")
			}

			mockT = &mockTesting{}
			LessOrEqual(mockT, cases[i].v2, cases[i].v1)
			if !mockT.failed {
				t.Error("expected failed")
			}

			mockT = &mockTesting{}
			LessOrEqual(mockT, cases[i].v1, cases[i].v1)
			if mockT.failed {
				t.Error("expected success")
			}
		})
	}
}

func TestEmpty(t *testing.T) {
	cases := []struct {
		object   any
		notEmpty bool
	}{
		{object: nil},
		{object: ""},
		{object: map[string]struct{}{}},
		{object: make([]string, 0)},
	}

	for i := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			mockT := &mockTesting{}
			Empty(mockT, cases[i].object)
			if mockT.failed {
				t.Error("expected success")
			}

			mockT = &mockTesting{}
			NotEmpty(mockT, cases[i].object)
			if !mockT.failed {
				t.Error("expected failed")
			}
		})
	}
}

type mockTesting struct {
	failed bool
	msg    string
}

func (t *mockTesting) Errorf(format string, args ...any) {
	t.failed = true
	t.msg = fmt.Sprintf(format, args...)
}

func (t *mockTesting) Helper() {}
