package fsm

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFSM(t *testing.T) {
	const (
		initState State = iota
		shuttingDownState
	)

	t.Run("Normal", func(t *testing.T) {
		l := NewFSM(map[State]StateFunc{
			initState: func() (State, error) {
				return Next(shuttingDownState)
			},
			shuttingDownState: func() (State, error) {
				return Finish()
			},
		}, initState, shuttingDownState)
		err := l.Loop()
		require.NoError(t, err)
	})

	t.Run("Wait", func(t *testing.T) {
		ch := make(chan struct{})
		shutDown := false

		l := NewFSM(map[State]StateFunc{
			initState: func() (State, error) {
				defer func() { close(ch) }()
				return Wait()
			},
			shuttingDownState: func() (State, error) {
				shutDown = true
				return Finish()
			},
		}, initState, shuttingDownState)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			select {
			case <-ch:
				cancel()
			}
		}()
		err := l.LoopContext(ctx)
		require.NoError(t, err)
		assert.True(t, shutDown)
	})

	t.Run("Error", func(t *testing.T) {
		l := NewFSM(map[State]StateFunc{
			initState: func() (State, error) {
				return Error(errors.New("init error"))
			},
			shuttingDownState: func() (State, error) {
				t.Log("shutting down")
				return Finish()
			},
		}, initState, shuttingDownState)
		err := l.Loop()
		require.Error(t, err)
		assert.EqualError(t, err, "init error")
	})

	t.Run("ErrorAtCloseState", func(t *testing.T) {
		l := NewFSM(map[State]StateFunc{
			initState: func() (State, error) {
				return Error(errors.New("init error"))
			},
			shuttingDownState: func() (State, error) {
				return Error(errors.New("shutting down error"))
			},
		}, initState, shuttingDownState)

		err := l.Loop()
		require.Error(t, err)
		assert.EqualError(t, err, "shutting down error")
	})
}
