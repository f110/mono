package leaderelection

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestObjectStorage_Lock(t *testing.T) {
	logger.Init()

	t.Run("NotExist", func(t *testing.T) {
		st := newSyncStorage()
		l, err := NewObjectStorage(st, "lock")
		assertion.MustNoError(t, err)

		_, err = l.Lock(t.Context())
		assertion.MustNoError(t, err)
		assertion.Equal(t, readRecord(t, st).Id, l.id)
	})

	t.Run("Expired", func(t *testing.T) {
		st := newSyncStorage()
		writeRecord(t, st, &record{Id: "other", Expire: time.Now().Add(-time.Minute)})
		l, err := NewObjectStorage(st, "lock")
		assertion.MustNoError(t, err)

		_, err = l.Lock(t.Context())
		assertion.MustNoError(t, err)
		assertion.Equal(t, readRecord(t, st).Id, l.id)
	})

	t.Run("HeldByOther", func(t *testing.T) {
		st := newSyncStorage()
		writeRecord(t, st, &record{Id: "other", Expire: time.Now().Add(time.Hour)})
		l, err := NewObjectStorage(st, "lock")
		assertion.MustNoError(t, err)
		l.retryInterval = 10 * time.Millisecond

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_, err = l.Lock(ctx)
		assertion.MustError(t, err)
		assertion.Equal(t, readRecord(t, st).Id, "other")
	})

	t.Run("ReleasedByOther", func(t *testing.T) {
		st := newSyncStorage()
		writeRecord(t, st, &record{Id: "other", Expire: time.Now().Add(50 * time.Millisecond)})
		l, err := NewObjectStorage(st, "lock")
		assertion.MustNoError(t, err)
		l.retryInterval = 10 * time.Millisecond

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err = l.Lock(ctx)
		assertion.MustNoError(t, err)
		assertion.Equal(t, readRecord(t, st).Id, l.id)
	})

	t.Run("Renew", func(t *testing.T) {
		st := newSyncStorage()
		l, err := NewObjectStorage(st, "lock")
		assertion.MustNoError(t, err)
		l.renewInterval = 10 * time.Millisecond

		lockCtx, err := l.Lock(t.Context())
		assertion.MustNoError(t, err)
		first := readRecord(t, st).Expire
		waitFor(t, func() bool { return readRecord(t, st).Expire.After(first) })
		assertion.Nil(t, lockCtx.Err())
	})

	t.Run("TakenByOther", func(t *testing.T) {
		st := newSyncStorage()
		l, err := NewObjectStorage(st, "lock")
		assertion.MustNoError(t, err)
		l.renewInterval = 10 * time.Millisecond

		lockCtx, err := l.Lock(t.Context())
		assertion.MustNoError(t, err)
		writeRecord(t, st, &record{Id: "other", Expire: time.Now().Add(time.Hour)})
		waitFor(t, func() bool { return lockCtx.Err() != nil })
		time.Sleep(50 * time.Millisecond)
		assertion.Equal(t, readRecord(t, st).Id, "other")
	})

	t.Run("FailedToRenewUntilExpire", func(t *testing.T) {
		st := &failingStorage{syncStorage: newSyncStorage()}
		l, err := NewObjectStorage(st, "lock")
		assertion.MustNoError(t, err)
		l.ttl = 50 * time.Millisecond
		l.renewInterval = 10 * time.Millisecond

		lockCtx, err := l.Lock(t.Context())
		assertion.MustNoError(t, err)
		st.fail.Store(true)
		waitFor(t, func() bool { return lockCtx.Err() != nil })
	})

	t.Run("ReleaseWhenDone", func(t *testing.T) {
		st := newSyncStorage()
		l, err := NewObjectStorage(st, "lock")
		assertion.MustNoError(t, err)

		ctx, cancel := context.WithCancel(t.Context())
		lockCtx, err := l.Lock(ctx)
		assertion.MustNoError(t, err)
		cancel()
		waitFor(t, func() bool {
			_, err := st.Get(context.Background(), "lock")
			return errors.Is(err, storage.ErrObjectNotFound)
		})
		assertion.MustError(t, lockCtx.Err())
	})

	t.Run("NotReleaseOthersLock", func(t *testing.T) {
		st := newSyncStorage()
		l, err := NewObjectStorage(st, "lock")
		assertion.MustNoError(t, err)

		ctx, cancel := context.WithCancel(t.Context())
		lockCtx, err := l.Lock(ctx)
		assertion.MustNoError(t, err)
		writeRecord(t, st, &record{Id: "other", Expire: time.Now().Add(time.Hour)})
		cancel()
		waitFor(t, func() bool { return lockCtx.Err() != nil })
		time.Sleep(50 * time.Millisecond)
		assertion.Equal(t, readRecord(t, st).Id, "other")
	})
}

type syncStorage struct {
	mu   sync.Mutex
	mock *storage.Mock
}

func newSyncStorage() *syncStorage {
	return &syncStorage{mock: storage.NewMock()}
}

func (s *syncStorage) Get(ctx context.Context, name string) (*storage.Object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	obj, err := s.mock.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	buf, err := io.ReadAll(obj.Body)
	if err != nil {
		return nil, err
	}
	obj.Body = io.NopCloser(bytes.NewReader(buf))
	return obj, nil
}

func (s *syncStorage) Put(ctx context.Context, name string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mock.Put(ctx, name, data)
}

func (s *syncStorage) Delete(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mock.Delete(ctx, name)
}

type failingStorage struct {
	*syncStorage
	fail atomic.Bool
}

func (s *failingStorage) Get(ctx context.Context, name string) (*storage.Object, error) {
	if s.fail.Load() {
		return nil, errors.New("failed")
	}
	return s.syncStorage.Get(ctx, name)
}

func (s *failingStorage) Put(ctx context.Context, name string, data []byte) error {
	if s.fail.Load() {
		return errors.New("failed")
	}
	return s.syncStorage.Put(ctx, name, data)
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func readRecord(t *testing.T, st *syncStorage) *record {
	obj, err := st.Get(context.Background(), "lock")
	assertion.MustNoError(t, err)
	buf, err := io.ReadAll(obj.Body)
	assertion.MustNoError(t, err)
	r := &record{}
	assertion.MustNoError(t, json.Unmarshal(buf, r))
	return r
}

func writeRecord(t *testing.T, st *syncStorage, r *record) {
	buf, err := json.Marshal(r)
	assertion.MustNoError(t, err)
	assertion.MustNoError(t, st.Put(context.Background(), "lock", buf))
}
