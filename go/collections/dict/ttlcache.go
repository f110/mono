package dict

import (
	"container/list"
	"sync"
	"time"
)

// TTLCache is a process-level, in-memory cache. Each entry expires after ttl and
// the total byte size of the live entries is kept under maxBytes by evicting the
// least recently used entries, so the peak memory usage is bounded regardless of
// how large individual values (such as packfiles) are.
//
// The zero value is not usable; create one with NewTTLCache.
type TTLCache[K comparable, V any] struct {
	ttl      time.Duration
	maxBytes int64
	sizeOf   func(V) int64

	mu       sync.Mutex
	ll       *list.List // *entry, front is the most recently used
	items    map[K]*list.Element
	inflight map[K]*call[V]
	curBytes int64

	now       func() time.Time
	closeCh   chan struct{}
	closeOnce sync.Once
}

type entry[K comparable, V any] struct {
	key       K
	value     V
	size      int64
	expiresAt time.Time
}

// call holds the in-flight load for a single key so that concurrent GetOrLoad
// callers share one execution instead of stampeding the backend.
type call[V any] struct {
	wg  sync.WaitGroup
	val V
	err error
}

// NewTTLCache returns a cache whose entries live for ttl and whose total size is
// bounded by maxBytes (sizeOf reports the size of a value). When sweepInterval is
// positive a background goroutine reclaims expired entries on that interval; call
// Close to stop it.
func NewTTLCache[K comparable, V any](ttl, sweepInterval time.Duration, maxBytes int64, sizeOf func(V) int64) *TTLCache[K, V] {
	c := &TTLCache[K, V]{
		ttl:      ttl,
		maxBytes: maxBytes,
		sizeOf:   sizeOf,
		ll:       list.New(),
		items:    make(map[K]*list.Element),
		inflight: make(map[K]*call[V]),
		now:      time.Now,
		closeCh:  make(chan struct{}),
	}
	if sweepInterval > 0 {
		go c.sweepLoop(sweepInterval)
	}
	return c
}

// Get returns the value for key if it is present and not expired.
func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.getLocked(key)
}

// Set stores value under key, replacing any existing entry and resetting its TTL.
func (c *TTLCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setLocked(key, value)
}

// GetOrLoad returns the cached value for key. On a miss it calls load to produce
// the value, stores it and returns it. Concurrent calls for the same key run load
// only once and share its result; the caller does not need to guard against a
// thundering herd. When load returns an error nothing is cached and the error is
// returned to every waiter.
func (c *TTLCache[K, V]) GetOrLoad(key K, load func() (V, error)) (V, error) {
	c.mu.Lock()
	if v, ok := c.getLocked(key); ok {
		c.mu.Unlock()
		return v, nil
	}
	if cl, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		cl.wg.Wait()
		return cl.val, cl.err
	}
	cl := new(call[V])
	cl.wg.Add(1)
	c.inflight[key] = cl
	c.mu.Unlock()

	cl.val, cl.err = load()

	c.mu.Lock()
	delete(c.inflight, key)
	if cl.err == nil {
		c.setLocked(key, cl.val)
	}
	c.mu.Unlock()
	cl.wg.Done()

	return cl.val, cl.err
}

// Delete removes key from the cache.
func (c *TTLCache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.removeElement(el)
	}
}

// Len returns the number of live entries, including ones that have expired but
// have not been reclaimed yet.
func (c *TTLCache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}

// Close stops the background sweep goroutine. It is safe to call more than once.
func (c *TTLCache[K, V]) Close() {
	c.closeOnce.Do(func() {
		close(c.closeCh)
	})
}

func (c *TTLCache[K, V]) getLocked(key K) (V, bool) {
	el, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	ent := el.Value.(*entry[K, V])
	if c.now().After(ent.expiresAt) {
		c.removeElement(el)
		var zero V
		return zero, false
	}
	c.ll.MoveToFront(el)
	return ent.value, true
}

func (c *TTLCache[K, V]) setLocked(key K, value V) {
	size := c.sizeOf(value)
	expiresAt := c.now().Add(c.ttl)
	if el, ok := c.items[key]; ok {
		ent := el.Value.(*entry[K, V])
		c.curBytes += size - ent.size
		ent.value = value
		ent.size = size
		ent.expiresAt = expiresAt
		c.ll.MoveToFront(el)
	} else {
		el := c.ll.PushFront(&entry[K, V]{key: key, value: value, size: size, expiresAt: expiresAt})
		c.items[key] = el
		c.curBytes += size
	}
	c.evictLocked()
}

func (c *TTLCache[K, V]) evictLocked() {
	if c.maxBytes <= 0 {
		return
	}
	for c.curBytes > c.maxBytes {
		el := c.ll.Back()
		if el == nil {
			return
		}
		c.removeElement(el)
	}
}

func (c *TTLCache[K, V]) removeElement(el *list.Element) {
	ent := el.Value.(*entry[K, V])
	c.ll.Remove(el)
	delete(c.items, ent.key)
	c.curBytes -= ent.size
}

func (c *TTLCache[K, V]) sweepLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-c.closeCh:
			return
		case <-ticker.C:
			c.sweep()
		}
	}
}

func (c *TTLCache[K, V]) sweep() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	for el := c.ll.Back(); el != nil; {
		prev := el.Prev()
		if now.After(el.Value.(*entry[K, V]).expiresAt) {
			c.removeElement(el)
		}
		el = prev
	}
}
