package main

import (
	"context"
	"testing"
	"time"

	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestUpdaterCommand_Run(t *testing.T) {
	logger.Init()

	t.Run("LostTheLock", func(t *testing.T) {
		c := newUpdaterCommand()
		c.updater = git.NewUpdater(nil, nil, git.NewStaticRepositoryProvider(nil), 1).SetInterval(time.Hour)
		c.locker = &fakeLocker{lockDuration: 10 * time.Millisecond}

		_, err := c.run(t.Context())
		assertion.MustError(t, err)
	})

	t.Run("Shutdown", func(t *testing.T) {
		c := newUpdaterCommand()
		c.updater = git.NewUpdater(nil, nil, git.NewStaticRepositoryProvider(nil), 1).SetInterval(time.Hour)
		c.locker = &fakeLocker{lockDuration: time.Hour}

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
		defer cancel()
		s, err := c.run(ctx)
		assertion.MustNoError(t, err)
		assertion.Equal(t, s, fsm.WaitState)
	})
}

type fakeLocker struct {
	lockDuration time.Duration
}

func (l *fakeLocker) Lock(ctx context.Context) (context.Context, error) {
	lockCtx, cancel := context.WithTimeout(ctx, l.lockDuration)
	go func() {
		<-lockCtx.Done()
		cancel()
	}()
	return lockCtx, nil
}
