package set

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSet_Add(t *testing.T) {
	s := New()
	s.Add("ok")
	s.Add("foobar")

	slice := s.ToSlice()
	assert.Len(t, slice, 2)
	assert.ElementsMatch(t, slice, []interface{}{"ok", "foobar"})
}

func TestSet_Diff(t *testing.T) {
	left := New("ok", "foobar", "baz")
	right := New("baz")

	d := left.Diff(right)
	slice := d.ToSlice()
	assert.Len(t, slice, 2)
	assert.ElementsMatch(t, slice, []interface{}{"ok", "foobar"})
}
