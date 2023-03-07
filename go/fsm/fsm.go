package fsm

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"go.f110.dev/xerrors"
)

type State int
type StateFunc func(context.Context) (State, error)

const (
	UnknownState State = -255
	WaitState    State = -254
	CloseState   State = -253
)

var (
	ErrUnrecognizedState = xerrors.New("unrecognized state")
)

type FSM struct {
	// CloseContext allows to specify the context when entering the close state.
	// If not specify CloseContext, then entering the close state will be used same context.
	CloseContext func() (context.Context, context.CancelFunc)

	ch         chan State
	funcs      map[State]StateFunc
	initState  State
	closeState State
	ctx        context.Context
	cancel     context.CancelFunc
	beClosing  bool
	lastErr    error
}

func NewFSM(funcs map[State]StateFunc, initState, closeState State) *FSM {
	return &FSM{
		ch:         make(chan State),
		funcs:      funcs,
		initState:  initState,
		closeState: closeState,
	}
}

func Next(state State) (State, error) {
	return state, nil
}

func Error(err error) (State, error) {
	return UnknownState, err
}

func Finish() (State, error) {
	return CloseState, nil
}

func Wait() (State, error) {
	return WaitState, nil
}

// SignalHandling handles specifying signal
// Deprecated.
func (f *FSM) SignalHandling(signals ...os.Signal) {
	signalCh := make(chan os.Signal)
	signal.Notify(signalCh, signals...)

	go func() {
		for sig := range signalCh {
			for _, v := range signals {
				if v == sig {
					f.nextState(f.closeState)
					return
				}
			}
		}
	}()
}

func (f *FSM) Shutdown() {
	f.nextState(f.closeState)
}

func (f *FSM) context() context.Context {
	if f.ctx == nil {
		return context.Background()
	}
	return f.ctx
}

func (f *FSM) setContext(ctx context.Context) {
	f.ctx = ctx
	go func() {
		<-ctx.Done()
		f.nextState(f.closeState)
	}()
}

func (f *FSM) LoopContext(ctx context.Context) error {
	f.setContext(ctx)
	return f.Loop()
}

func (f *FSM) Loop() error {
	go func() {
		f.nextState(f.initState)
	}()
	defer func() {
		if f.cancel != nil {
			f.cancel()
		}
	}()

	for {
		s, open := <-f.ch
		if !open {
			return f.lastErr
		}
		if s == f.closeState {
			f.beClosing = true
			if f.CloseContext != nil {
				c, cancel := f.CloseContext()
				f.ctx = c
				f.cancel = cancel
			}
		}

		var fn StateFunc
		if v, ok := f.funcs[s]; ok {
			fn = v
		} else {
			return ErrUnrecognizedState
		}

		go func() {
			if nxt, err := fn(f.context()); err != nil {
				fmt.Fprintf(os.Stderr, "%+v\n", err)
				f.lastErr = err

				// When the function for close state is returning an error, we should stop the main loop ASAP.
				if s == f.closeState {
					ch := f.ch
					f.ch = nil
					close(ch)
					return
				}
				// When one of a function for close state is returning an error, we also should stop the main loop immediately.
				if f.beClosing {
					ch := f.ch
					f.ch = nil
					if ch != nil {
						close(ch)
					}
					return
				}

				f.nextState(f.closeState)
			} else if nxt == CloseState {
				ch := f.ch
				f.ch = nil
				close(ch)
			} else if nxt > 0 {
				f.nextState(nxt)
			}
		}()
	}
}

func (f *FSM) nextState(s State) {
	ch := f.ch
	if ch != nil {
		ch <- s
	}
}
