package parallel

import (
	"context"
	"log/slog"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/logger/slogger"
)

type supervisorState int

const (
	supervisorStateRunning supervisorState = iota
	supervisorStateShuttingDown
	supervisorStateShutDowned
)

const (
	restartBackoff       = 100 * time.Millisecond
	backoffResetDuration = 30 * time.Second
)

type Supervisor struct {
	Log *slog.Logger

	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup

	mu       sync.Mutex
	state    supervisorState
	children []*childProcess
}

func NewSupervisor(ctx context.Context) *Supervisor {
	c, cancel := ctxutil.WithCancel(ctx)
	return &Supervisor{Log: slogger.Log, ctx: c, cancelFunc: cancel, state: supervisorStateRunning}
}

func (s *Supervisor) Add(f func(ctx context.Context)) {
	child := newChildProcess(s.Len()+1, f)
	child.Log = s.Log
	s.mu.Lock()
	s.children = append(s.children, child)
	s.mu.Unlock()

	s.Log.Info("Add new process", slog.Int("num", s.Len()))

	s.wg.Add(1)
	go child.Run(s.ctx, s.wg.Done)
}

func (s *Supervisor) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.children)
}

func (s *Supervisor) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if s.state != supervisorStateRunning {
		s.mu.Unlock()
		return xerrors.New("parallel: Terminating or terminated")
	}
	s.state = supervisorStateShuttingDown
	s.mu.Unlock()

	// Stop all child processes
	s.cancelFunc()

	doneCh := make(chan struct{})
	go func() {
		s.wg.Wait()

		close(doneCh)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-doneCh:
		s.mu.Lock()
		s.state = supervisorStateShutDowned
		s.mu.Unlock()
		return nil
	}
}

type childProcess struct {
	Id       int
	Interval time.Duration
	Log      *slog.Logger

	restart         int
	c               int
	rand            *rand.Rand
	lastRestartTime time.Time
	exited          bool

	fn func(ctx context.Context)
}

func newChildProcess(id int, fn func(ctx context.Context)) *childProcess {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &childProcess{Id: id, c: 1, Interval: restartBackoff, Log: slogger.Log, rand: r, fn: fn}
}

func (c *childProcess) Run(ctx context.Context, done func()) {
	defer func() {
		c.exited = true
		done()
	}()

	for {
		c.run(ctx)

		select {
		case <-ctx.Done():
			return
		default:
		}

		if time.Since(c.lastRestartTime) > backoffResetDuration {
			c.resetBackoff()
		}

		backoff := c.calculateNextBackoff()
		c.Log.Info("Wait restart", slog.Duration("backoff", backoff), slog.Int("id", c.Id), slog.Int("count", c.restart))
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		c.lastRestartTime = time.Now()
	}
}

func (c *childProcess) run(ctx context.Context) {
	defer func() {
		c.restart++
		if r := recover(); r != nil {
			const size = 64 << 10
			stacktrace := make([]byte, size)
			stacktrace = stacktrace[:runtime.Stack(stacktrace, false)]
			c.Log.Warn("Panic", slog.String("stacktrace", string(stacktrace)))
		}
	}()

	c.fn(ctx)
}

func (c *childProcess) calculateNextBackoff() time.Duration {
	c.c *= 2

	if c.restart < 4 {
		return time.Duration(c.restart) * c.Interval
	}

	k := c.rand.Intn(c.c-1) + 1
	return time.Duration(k) * c.Interval
}

func (c *childProcess) resetBackoff() {
	c.c = 1
	c.restart = 0
}
