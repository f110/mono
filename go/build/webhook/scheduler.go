package webhook

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/logger/slogger"
)

// Scheduler polls the github_event table on the elected leader and dispatches
// PENDING and FAILED rows to the matching Reconciler.
//
// A row that has been waiting (state PENDING or FAILED) longer than
// MaxProcessingDuration since CreatedAt is moved to EXPIRED and excluded from
// further attempts. PROCESSING rows from a previous instance are reset to
// PENDING on startup so a crash mid-Reconcile does not strand them.
type Scheduler struct {
	dao                   dao.Options
	reconcilers           Reconcilers
	interval              time.Duration
	maxProcessingDuration time.Duration
	notifier              *Notifier

	kick chan struct{}
	now  func() time.Time
}

func NewScheduler(daos dao.Options, recs Reconcilers, notifier *Notifier, interval, maxProcessingDuration time.Duration) *Scheduler {
	return &Scheduler{
		dao:                   daos,
		reconcilers:           recs,
		interval:              interval,
		maxProcessingDuration: maxProcessingDuration,
		notifier:              notifier,
		kick:                  make(chan struct{}, 1),
		now:                   time.Now,
	}
}

// Run loops until ctx is cancelled. Intended to be started in its own
// goroutine once the leader role is acquired.
func (s *Scheduler) Run(ctx context.Context) {
	if s.notifier != nil {
		s.notifier.Register(s.kick)
	}
	slogger.Log.Info("Start webhook scheduler", slog.Duration("interval", s.interval), slog.Duration("max_processing", s.maxProcessingDuration))

	if err := s.recoverStuck(ctx); err != nil {
		slogger.Log.Warn("Failed to recover PROCESSING rows on startup", slogger.E(err))
	}

	t := time.NewTicker(s.interval)
	defer t.Stop()

	s.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-s.kick:
		}
		s.runOnce(ctx)
	}
}

// recoverStuck resets any rows left in PROCESSING by a previous instance.
// Safe to call only on the leader because non-leaders never write to
// github_event.state.
func (s *Scheduler) recoverStuck(ctx context.Context) error {
	rows, err := s.dao.GithubEvent.ListByState(ctx, uint32(database.GithubEventStateProcessing))
	if err != nil {
		return xerrors.WithStack(err)
	}
	for _, r := range rows {
		r.State = database.GithubEventStatePending
		if err := s.dao.GithubEvent.Update(ctx, r); err != nil {
			slogger.Log.Warn("Failed to reset stuck PROCESSING row",
				slog.Int("id", int(r.Id)), slogger.E(err))
		}
	}
	if len(rows) > 0 {
		slogger.Log.Info("Reset stuck PROCESSING rows", slog.Int("count", len(rows)))
	}
	return nil
}

func (s *Scheduler) runOnce(ctx context.Context) {
	rows, err := s.fetchTargets(ctx)
	if err != nil {
		slogger.Log.Warn("Failed to fetch reconcile targets", slogger.E(err))
		return
	}
	for _, ev := range rows {
		if ctx.Err() != nil {
			return
		}
		s.process(ctx, ev)
	}
}

// fetchTargets returns PENDING and FAILED rows in id order. PENDING is
// queried first so freshly inserted rows are processed before older retries.
func (s *Scheduler) fetchTargets(ctx context.Context) ([]*database.GithubEvent, error) {
	pending, err := s.dao.GithubEvent.ListByState(ctx, uint32(database.GithubEventStatePending))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	failed, err := s.dao.GithubEvent.ListByState(ctx, uint32(database.GithubEventStateFailed))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	all := append(pending, failed...)
	sort.Slice(all, func(i, j int) bool { return all[i].Id < all[j].Id })
	return all, nil
}

// process handles one row: expiry / unknown-event_type bookkeeping that
// happens *before* a reconciler is ever invoked stays here, since the
// scheduler is the only component that can observe those conditions. Once a
// reconciler is dispatched, all further state transitions are its
// responsibility.
func (s *Scheduler) process(ctx context.Context, ev *database.GithubEvent) {
	if s.maxProcessingDuration > 0 && s.now().Sub(ev.CreatedAt) > s.maxProcessingDuration {
		ev.State = database.GithubEventStateExpired
		if ev.LastError == "" {
			ev.LastError = "exceeded max processing duration"
		}
		if err := s.dao.GithubEvent.Update(ctx, ev); err != nil {
			slogger.Log.Warn("Failed to mark event as EXPIRED", slog.Int("id", int(ev.Id)), slogger.E(err))
		}
		return
	}

	rec, ok := s.reconcilers[ev.EventType]
	if !ok {
		ev.State = database.GithubEventStateSkipped
		ev.LastError = "no reconciler registered for event_type=" + ev.EventType
		if err := s.dao.GithubEvent.Update(ctx, ev); err != nil {
			slogger.Log.Warn("Failed to mark event as SKIPPED", slog.Int("id", int(ev.Id)), slogger.E(err))
		}
		return
	}

	if err := rec.Reconcile(ctx, ev); err != nil {
		slogger.Log.Warn("Reconcile failed",
			slog.Int("id", int(ev.Id)),
			slog.String("event_type", ev.EventType),
			slogger.E(err))
	}
}
