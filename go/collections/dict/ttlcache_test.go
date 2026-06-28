package dict

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.f110.dev/mono/go/testing/assertion"
)

func sizeOfBytes(v []byte) int64 { return int64(len(v)) }

func TestTTLCache_GetSet(t *testing.T) {
	c := NewTTLCache[string](time.Minute, 0, 1<<20, sizeOfBytes)
	defer c.Close()

	_, ok := c.Get("missing")
	assertion.False(t, ok)

	c.Set("a", []byte("foo"))
	v, ok := c.Get("a")
	assertion.True(t, ok)
	assertion.Equal(t, string(v), "foo")
}

func TestTTLCache_Expire(t *testing.T) {
	c := NewTTLCache[string](time.Minute, 0, 1<<20, sizeOfBytes)
	defer c.Close()
	now := time.Now()
	c.now = func() time.Time { return now }

	c.Set("a", []byte("foo"))
	_, ok := c.Get("a")
	assertion.True(t, ok)

	// Advance past the TTL.
	now = now.Add(time.Minute + time.Second)
	_, ok = c.Get("a")
	assertion.False(t, ok)
	assertion.Equal(t, c.Len(), 0)
}

func TestTTLCache_EvictByBytes(t *testing.T) {
	// maxBytes only holds two 4-byte values.
	c := NewTTLCache[string](time.Minute, 0, 8, sizeOfBytes)
	defer c.Close()

	c.Set("a", []byte("aaaa"))
	c.Set("b", []byte("bbbb"))
	// Touch "a" so "b" becomes the least recently used.
	_, ok := c.Get("a")
	assertion.True(t, ok)

	c.Set("c", []byte("cccc"))

	assertion.Equal(t, c.Len(), 2)
	_, ok = c.Get("b")
	assertion.False(t, ok) // evicted as LRU
	_, ok = c.Get("a")
	assertion.True(t, ok)
	_, ok = c.Get("c")
	assertion.True(t, ok)
}

func TestTTLCache_GetOrLoad_SingleFlight(t *testing.T) {
	c := NewTTLCache[string](time.Minute, 0, 1<<20, sizeOfBytes)
	defer c.Close()

	var loadCount atomic.Int32
	release := make(chan struct{})
	load := func() ([]byte, error) {
		loadCount.Add(1)
		<-release // block so every goroutine piles up on the same key
		return []byte("value"), nil
	}

	const n = 20
	var wg sync.WaitGroup
	results := make([][]byte, n)
	for i := range n {
		wg.Go(func() {
			v, err := c.GetOrLoad("k", load)
			assertion.NoError(t, err)
			results[i] = v
		})
	}
	// Give the goroutines time to block in load before releasing.
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	assertion.Equal(t, int(loadCount.Load()), 1)
	for _, v := range results {
		assertion.Equal(t, string(v), "value")
	}
	// A subsequent hit must not call load again.
	v, err := c.GetOrLoad("k", func() ([]byte, error) {
		t.Fatal("load must not be called on a cache hit")
		return nil, nil
	})
	assertion.NoError(t, err)
	assertion.Equal(t, string(v), "value")
}

func TestTTLCache_Sweep(t *testing.T) {
	c := NewTTLCache[string](time.Minute, 0, 1<<20, sizeOfBytes)
	defer c.Close()
	now := time.Now()
	c.now = func() time.Time { return now }

	c.Set("a", []byte("foo"))
	now = now.Add(time.Minute + time.Second)

	// sweep reclaims expired entries without an explicit Get.
	c.sweep()
	assertion.Equal(t, c.Len(), 0)
}
