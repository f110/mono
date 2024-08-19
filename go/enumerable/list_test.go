package enumerable

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsInclude(t *testing.T) {
	assert.True(t, IsInclude([]int{1, 2, 3, 4, 5}, 5))
	assert.False(t, IsInclude([]int{1, 2, 3, 4, 5}, 6))
	assert.True(t, IsInclude([]string{"foo", "bar", "baz"}, "bar"))
	assert.False(t, IsInclude([]string{"foo", "bar", "baz"}, "foobar"))
}

func TestDelete(t *testing.T) {
	assert.Equal(t, []int{1, 2, 3, 4}, Delete([]int{1, 2, 3, 4, 5}, 5))
}

func TestSum(t *testing.T) {
	assert.Equal(t, int64(55), Sum([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, func(i int) int64 { return int64(i) }))
}

func TestUniq(t *testing.T) {
	assert.Equal(t, []int{1, 2, 3, 4, 5}, Uniq([]int{1, 1, 2, 3, 4, 4, 4, 5}, func(t int) int { return t }))
}

func TestInsertBefore(t *testing.T) {
	assert.Equal(t, []int{1, 2, 3, 4, 5}, InsertBefore([]int{1, 2, 3, 5}, 5, 4))
}

func TestIndex(t *testing.T) {
	assert.Equal(t, -1, Index([]int{1, 2, 3, 4, 5}, func(i int) bool { return i == 6 }))
	assert.Equal(t, 4, Index([]int{1, 2, 3, 4, 5}, func(i int) bool { return i == 5 }))
}

func TestCollectFunc(t *testing.T) {
	assert.Equal(t, []int{2, 4, 6, 8, 10}, slices.Collect(CollectFunc(slices.Values([]int{1, 2, 3, 4, 5}), func(i int) int { return i * 2 })))
}
