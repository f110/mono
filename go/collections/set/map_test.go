package set

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSet_Add(t *testing.T) {
	s := New[string]()
	s.Add("ok")
	s.Add("foobar")

	slice := s.ToSlice()
	assert.Len(t, slice, 2)
	assert.ElementsMatch(t, slice, []string{"ok", "foobar"})
}

func TestSet_Diff(t *testing.T) {
	left := New[string]("ok", "foobar", "baz")
	right := New[string]("baz")

	d := left.Diff(right)
	slice := d.ToSlice()
	assert.Len(t, slice, 2)
	assert.ElementsMatch(t, slice, []string{"ok", "foobar"})
}

func TestSet_Join(t *testing.T) {
	left := New[string]("ok", "foobar", "baz")
	right := New[string]("baz")

	d := left.Join(right)
	slice := d.ToSlice()
	assert.Len(t, slice, 1)
}

func TestSet_Union(t *testing.T) {
	left := New[string]("ok", "foobar", "baz")
	right := New[string]("baz", "buz")

	left.Union(right)
	slice := left.ToSlice()
	assert.Len(t, slice, 4)
}

func TestSet_RightOuter(t *testing.T) {
	left := New[string]("ok", "foobar", "baz")
	right := New[string]("baz", "buz")

	rightOuter := left.RightOuter(right)
	slice := rightOuter.ToSlice()
	assert.Len(t, slice, 1)
}

func TestSet_LeftOuter(t *testing.T) {
	left := New[string]("ok", "foobar", "baz")
	right := New[string]("baz")

	d := left.LeftOuter(right)
	slice := d.ToSlice()
	assert.Len(t, slice, 2)
}
