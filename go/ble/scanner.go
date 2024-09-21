package ble

import (
	"context"
	"sync"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/ctxutil"
)

var DefaultScanner = &Scanner{}

type Scanner struct {
	Error error

	mu sync.Mutex
	ch []chan Peripheral

	cancel context.CancelFunc
}

func (s *Scanner) Start(ctx context.Context) error {
	if s.cancel != nil {
		return nil
	}

	sCtx, cancel := ctxutil.WithCancel(ctx)
	s.cancel = cancel
	return s.start(sCtx)
}

func (s *Scanner) Stop() error {
	if s.cancel == nil {
		return xerrors.Define("ble: Scanner is not started").WithStack()
	}
	s.cancel()

	for _, ch := range s.ch {
		close(ch)
	}

	s.cancel = nil
	err := s.stop()
	if err != nil {
		return xerrors.WithStack(err)
	}
	return s.Error
}

func (s *Scanner) Scan() <-chan Peripheral {
	ch := make(chan Peripheral)
	s.mu.Lock()
	s.ch = append(s.ch, ch)
	s.mu.Unlock()

	return ch
}
