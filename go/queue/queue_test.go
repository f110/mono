package queue

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSimple(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))
	q := NewSimple()
	q.Enqueue("foo")
	q.Enqueue("bar")

	assert.Equal(t, "foo", q.Dequeue().(string))
	assert.Equal(t, "bar", q.Dequeue().(string))

	// One worker is waiting a new item
	go func() {
		time.Sleep(50 * time.Millisecond)
		q.Enqueue("baz")
	}()
	assert.Equal(t, "baz", q.Dequeue().(string))

	// Multiple workers are waiting a new item
	go func() {
		time.Sleep(50 * time.Millisecond)
		q.Enqueue("foo")
		q.Enqueue("bar")
		q.Enqueue("baz")
	}()

	var got []string
	var mu sync.Mutex
	for i := 0; i < 3; i++ {
		go func() {
			v := q.Dequeue().(string)
			mu.Lock()
			got = append(got, v)
			mu.Unlock()
		}()
	}

	time.Sleep(100 * time.Millisecond)
	assert.Contains(t, got, "foo")
	assert.Contains(t, got, "bar")
	assert.Contains(t, got, "baz")
}
