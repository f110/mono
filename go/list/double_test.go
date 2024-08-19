package list

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoubleLinked(t *testing.T) {
	d := NewDoubleLinked[int]()

	assert.Nil(t, d.Front())

	d.PushFront(1) // []int{1}
	require.Equal(t, 1, d.Len())
	assert.Equal(t, 1, d.Front().Value)
	assert.Nil(t, d.Front().Next())
	assert.Nil(t, d.Front().Prev())

	d.PushFront(2) // []int{2, 1}
	require.Equal(t, 2, d.Len())
	assert.Equal(t, 2, d.Front().Value)
	assert.Equal(t, 1, d.Back().Value)
	assert.Equal(t, 1, d.Front().Next().Value)

	d.PushFront(3) // []int{3, 2, 1}
	require.Equal(t, 3, d.Len())
	assert.Equal(t, 3, d.Front().Value)
	assert.Equal(t, 1, d.Back().Value)
	assert.Equal(t, 2, d.Front().Next().Value)
	assert.Equal(t, 1, d.Front().Next().Next().Value)

	d.PushBack(4) // []int{3, 2, 1, 4}
	require.Equal(t, 4, d.Len())
	assert.Equal(t, 3, d.Front().Value)
	assert.Equal(t, 4, d.Back().Value)
	assert.Equal(t, 2, d.Front().Next().Value)
	assert.Equal(t, 1, d.Front().Next().Next().Value)
	assert.Equal(t, 4, d.Front().Next().Next().Next().Value)

	d.Remove(d.Front()) // []int{2, 1, 4}
	require.Equal(t, 3, d.Len())
	assert.Equal(t, 2, d.Front().Value)
	assert.Equal(t, 4, d.Back().Value)
	assert.Equal(t, 1, d.Front().Next().Value)
	assert.Equal(t, 4, d.Front().Next().Next().Value)
}
