package ctxutil

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

type sourceCtx struct {
	context.Context

	file string
	line int
}

func (s *sourceCtx) Err() error {
	if s.Context.Err() != nil {
		return fmt.Errorf("%s of %s:%d", s.Context.Err(), s.file, s.line)
	}
	return nil
}

func WithTimeout(parent context.Context, duration time.Duration) (context.Context, context.CancelFunc) {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		return context.WithTimeout(parent, duration)
	}
	timerCtx, cancel := context.WithTimeout(parent, duration)
	return &sourceCtx{Context: timerCtx, file: file, line: line}, cancel
}

func WithCancel(parent context.Context) (context.Context, context.CancelFunc) {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		return context.WithCancel(parent)
	}
	cCtx, cancel := context.WithCancel(parent)
	return &sourceCtx{Context: cCtx, file: file, line: line}, cancel
}

func Source(ctx context.Context) (file string, line int) {
	sCtx, ok := ctx.(*sourceCtx)
	if !ok {
		return "", -1
	}
	return sCtx.file, sCtx.line
}
