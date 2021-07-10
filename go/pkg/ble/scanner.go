package ble

import (
	"context"
	"sync"

	"golang.org/x/xerrors"
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

	sCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	return s.start(sCtx)
}

func (s *Scanner) Stop() error {
	if s.cancel == nil {
		return xerrors.New("ble: Scanner is not started")
	}
	s.cancel()

	for _, ch := range s.ch {
		close(ch)
	}

	s.cancel = nil
	err := s.stop()
	if err != nil {
		return xerrors.Errorf(": %w", err)
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
