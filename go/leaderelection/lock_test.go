package leaderelection

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"go.f110.dev/mono/go/database/dbtestutil"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
	"go.f110.dev/mono/go/testing/assertion"
)

var mysqlPort int

func TestMain(m *testing.M) {
	os.Exit(runWithMySQL(m))
}

func TestMySQL_Lock(t *testing.T) {
	logger.Init()

	t.Run("NotHeld", func(t *testing.T) {
		db := newTestDatabase(t)
		l := NewMySQL(db, t.Name())

		_, err := l.Lock(t.Context())
		assertion.MustNoError(t, err)
		assertion.False(t, isFreeLock(t, db, t.Name()))
	})

	t.Run("Exclusive", func(t *testing.T) {
		db := newTestDatabase(t)
		l1 := NewMySQL(db, t.Name())
		l1.retryInterval = 10 * time.Millisecond
		l2 := NewMySQL(db, t.Name())
		l2.retryInterval = 10 * time.Millisecond

		_, err := l1.Lock(t.Context())
		assertion.MustNoError(t, err)
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		_, err = l2.Lock(ctx)
		assertion.MustError(t, err)
	})

	t.Run("ReleasedByOther", func(t *testing.T) {
		db := newTestDatabase(t)
		other, err := db.Conn(t.Context())
		assertion.MustNoError(t, err)
		defer other.Close()
		var got int
		assertion.MustNoError(t, other.QueryRowContext(t.Context(), "SELECT GET_LOCK(?, 0)", t.Name()).Scan(&got))
		assertion.Equal(t, got, 1)
		l := NewMySQL(db, t.Name())
		l.retryInterval = 10 * time.Millisecond

		go func() {
			time.Sleep(50 * time.Millisecond)
			other.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", t.Name())
		}()
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		_, err = l.Lock(ctx)
		assertion.MustNoError(t, err)
	})

	t.Run("LostConnection", func(t *testing.T) {
		db := newTestDatabase(t)
		l := NewMySQL(db, t.Name())
		l.checkInterval = 10 * time.Millisecond

		lockCtx, err := l.Lock(t.Context())
		assertion.MustNoError(t, err)
		var connID int64
		assertion.MustNoError(t, db.QueryRowContext(t.Context(), "SELECT IS_USED_LOCK(?)", t.Name()).Scan(&connID))
		_, err = db.ExecContext(t.Context(), fmt.Sprintf("KILL %d", connID))
		assertion.MustNoError(t, err)
		waitFor(t, func() bool { return lockCtx.Err() != nil })
	})

	t.Run("CancelledContext", func(t *testing.T) {
		db := newTestDatabase(t)
		l := NewMySQL(db, t.Name())

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := l.Lock(ctx)
		assertion.MustError(t, err)
	})

	t.Run("ReleaseWhenDone", func(t *testing.T) {
		db := newTestDatabase(t)
		l := NewMySQL(db, t.Name())

		ctx, cancel := context.WithCancel(t.Context())
		lockCtx, err := l.Lock(ctx)
		assertion.MustNoError(t, err)
		cancel()
		waitFor(t, func() bool { return isFreeLock(t, db, t.Name()) })
		assertion.MustError(t, lockCtx.Err())
	})
}

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

	t.Run("CancelledContext", func(t *testing.T) {
		st := newSyncStorage()
		l, err := NewObjectStorage(st, "lock")
		assertion.MustNoError(t, err)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err = l.Lock(ctx)
		assertion.MustError(t, err)
	})

	t.Run("RetryWhenFailedToPut", func(t *testing.T) {
		st := &failingStorage{syncStorage: newSyncStorage()}
		st.failPut.Store(true)
		l, err := NewObjectStorage(st, "lock")
		assertion.MustNoError(t, err)
		l.retryInterval = 10 * time.Millisecond

		go func() {
			time.Sleep(50 * time.Millisecond)
			st.failPut.Store(false)
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err = l.Lock(ctx)
		assertion.MustNoError(t, err)
		assertion.Equal(t, readRecord(t, st.syncStorage).Id, l.id)
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
	fail    atomic.Bool
	failPut atomic.Bool
}

func (s *failingStorage) Get(ctx context.Context, name string) (*storage.Object, error) {
	if s.fail.Load() {
		return nil, errors.New("failed")
	}
	return s.syncStorage.Get(ctx, name)
}

func (s *failingStorage) Put(ctx context.Context, name string, data []byte) error {
	if s.fail.Load() || s.failPut.Load() {
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

func runWithMySQL(m *testing.M) int {
	if !dbtestutil.CanUseTemporaryMySQL() {
		return m.Run()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	mysql, err := dbtestutil.NewTemporaryMySQL(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer mysql.Close()
	if err := mysql.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	mysqlPort = mysql.Port
	return m.Run()
}

func newTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	if mysqlPort == 0 {
		t.Skip("mysqld is not found")
	}
	db, err := sql.Open("mysql", fmt.Sprintf("root@tcp(127.0.0.1:%d)/", mysqlPort))
	assertion.MustNoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func isFreeLock(t *testing.T, db *sql.DB, name string) bool {
	var free int
	assertion.MustNoError(t, db.QueryRowContext(context.Background(), "SELECT IS_FREE_LOCK(?)", name).Scan(&free))
	return free == 1
}
