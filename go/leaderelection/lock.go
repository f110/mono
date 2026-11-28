package leaderelection

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

// Locker acquires the lock which is exclusive among processes.
type Locker interface {
	// Lock blocks until the lock is acquired or ctx is done.
	// The returned context is cancelled when the lock is lost or ctx is done.
	// The lock is released when ctx is done.
	Lock(ctx context.Context) (context.Context, error)
}

type objectStorageClient interface {
	Get(ctx context.Context, name string) (*storage.Object, error)
	Put(ctx context.Context, name string, data []byte) error
	Delete(ctx context.Context, name string) error
}

// ObjectStorage is the Locker which uses an object in the object storage as the lock.
// The lock has the expiration and is renewed periodically while it is held.
type ObjectStorage struct {
	client        objectStorageClient
	path          string
	id            string
	ttl           time.Duration
	renewInterval time.Duration
	retryInterval time.Duration
}

var _ Locker = &ObjectStorage{}

type record struct {
	Id     string
	Expire time.Time
}

func NewObjectStorage(client objectStorageClient, path string) (*ObjectStorage, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return nil, xerrors.WithStack(err)
	}
	return &ObjectStorage{
		client:        client,
		path:          path,
		id:            hex.EncodeToString(buf),
		ttl:           10 * time.Minute,
		renewInterval: 9 * time.Minute,
		retryInterval: 9 * time.Minute,
	}, nil
}

func (l *ObjectStorage) Lock(ctx context.Context) (context.Context, error) {
	logger.Log.Info("Acquiring the lock...", slog.String("id", l.id), slog.String("path", l.path))
	t := time.NewTicker(l.retryInterval)
	defer t.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil, xerrors.WithStack(err)
		}
		expire, ok, err := l.tryLock(ctx)
		if err != nil {
			logger.Log.Warn("Failed to acquire the lock", logger.E(err), slog.String("id", l.id))
		} else if ok {
			logger.Log.Info("Got lock", slog.String("id", l.id))
			lockCtx, cancel := context.WithCancel(ctx)
			go l.keep(ctx, cancel, expire)
			return lockCtx, nil
		}

		select {
		case <-ctx.Done():
			return nil, xerrors.WithStack(ctx.Err())
		case <-t.C:
		}
	}
}

func (l *ObjectStorage) tryLock(ctx context.Context) (time.Time, bool, error) {
	obj, err := l.client.Get(ctx, l.path)
	if err != nil && !errors.Is(err, storage.ErrObjectNotFound) {
		logger.Log.Warn("Failed to get the lock", logger.E(err))
		return time.Time{}, false, nil
	}
	if obj != nil {
		cur, err := decodeRecord(obj)
		if err != nil {
			return time.Time{}, false, err
		}
		if cur.Id != l.id && time.Now().Before(cur.Expire) {
			logger.Log.Debug("Other process is holding the lock", slog.String("id", cur.Id), slog.Time("expire", cur.Expire))
			return time.Time{}, false, nil
		}
	}

	expire, err := l.put(ctx)
	if err != nil {
		return time.Time{}, false, err
	}
	return expire, true, nil
}

func (l *ObjectStorage) keep(ctx context.Context, cancel context.CancelFunc, expire time.Time) {
	defer cancel()

	t := time.NewTicker(l.renewInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			l.release(context.WithoutCancel(ctx))
			return
		case <-t.C:
			if time.Now().After(expire) {
				logger.Log.Error("Lost the lock because it is expired", slog.String("id", l.id), slog.Time("expire", expire))
				return
			}
			cur, err := l.get(ctx)
			if err != nil && !errors.Is(err, storage.ErrObjectNotFound) {
				logger.Log.Warn("Failed to get the lock", logger.E(err), slog.String("id", l.id))
				continue
			}
			if cur == nil || cur.Id != l.id {
				logger.Log.Error("Lost the lock because other process has taken it", slog.String("id", l.id))
				return
			}
			newExpire, err := l.put(ctx)
			if err != nil {
				logger.Log.Warn("Failed to renew the lock", logger.E(err), slog.String("id", l.id))
				continue
			}
			expire = newExpire
		}
	}
}

func (l *ObjectStorage) release(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cur, err := l.get(ctx)
	if err != nil {
		if !errors.Is(err, storage.ErrObjectNotFound) {
			logger.Log.Warn("Failed to get the lock", logger.E(err), slog.String("id", l.id))
		}
		return
	}
	if cur.Id != l.id {
		return
	}
	if err := l.client.Delete(ctx, l.path); err != nil {
		logger.Log.Warn("Failed to release the lock", logger.E(err), slog.String("id", l.id))
		return
	}
	logger.Log.Info("Released the lock", slog.String("id", l.id))
}

func (l *ObjectStorage) get(ctx context.Context) (*record, error) {
	obj, err := l.client.Get(ctx, l.path)
	if err != nil {
		return nil, err
	}
	return decodeRecord(obj)
}

func decodeRecord(obj *storage.Object) (*record, error) {
	defer obj.Body.Close()

	cur := &record{}
	if err := json.NewDecoder(obj.Body).Decode(cur); err != nil {
		return nil, xerrors.WithStack(err)
	}
	return cur, nil
}

func (l *ObjectStorage) put(ctx context.Context) (time.Time, error) {
	expire := time.Now().Add(l.ttl)
	buf, err := json.Marshal(record{Id: l.id, Expire: expire})
	if err != nil {
		return time.Time{}, xerrors.WithStack(err)
	}
	if err := l.client.Put(ctx, l.path, buf); err != nil {
		return time.Time{}, xerrors.WithStack(err)
	}
	return expire, nil
}
