// Package webhook links the commits and the pull requests to the tickets by the webhook events of GitHub.
package webhook

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"

	buildapi "go.f110.dev/mono/go/build/api"
	ghwebhook "go.f110.dev/mono/go/github/webhook"
	"go.f110.dev/mono/go/issue"
	"go.f110.dev/mono/go/issue/database"
	"go.f110.dev/mono/go/issue/database/dao"
	"go.f110.dev/mono/go/logger/slogger"
)

// Repositories finds the repository registered in Build. The issue tracker handles only the repositories of Build.
type Repositories interface {
	// Find returns the id of the repository of url. ok is false if the repository is not registered.
	Find(ctx context.Context, url string) (id int32, ok bool, err error)
}

// BuildRepositories finds the repository by the API of Build.
type BuildRepositories struct {
	client buildapi.APIClient
}

var _ Repositories = (*BuildRepositories)(nil)

func NewBuildRepositories(client buildapi.APIClient) *BuildRepositories {
	return &BuildRepositories{client: client}
}

func (r *BuildRepositories) Find(ctx context.Context, url string) (int32, bool, error) {
	res, err := r.client.ListRepositories(ctx, &buildapi.RequestListRepositories{})
	if err != nil {
		return 0, false, xerrors.WithStack(err)
	}
	for _, v := range res.GetRepositories() {
		if v.GetUrl() == url {
			return v.GetId(), true, nil
		}
	}
	return 0, false, nil
}

// NewEventProcessor returns the EventProcessor that dispatches the events to the reconcilers of the issue tracker.
func NewEventProcessor(daos dao.Options, tracker *issue.Tracker, repositories Repositories, interval, maxProcessingDuration time.Duration) *ghwebhook.EventProcessor {
	p := ghwebhook.NewEventProcessor(newStore(daos.GithubEvent), interval, maxProcessingDuration)
	p.Register(NewPushReconciler(tracker, repositories))
	return p
}

// PushReconciler links the commits pushed to the default branch to the tickets which the trailers refer to. The ticket
// which a commit fixes is moved to Done.
type PushReconciler struct {
	tracker      *issue.Tracker
	repositories Repositories
}

var _ ghwebhook.Reconciler[github.PushEvent] = (*PushReconciler)(nil)

func NewPushReconciler(tracker *issue.Tracker, repositories Repositories) *PushReconciler {
	return &PushReconciler{tracker: tracker, repositories: repositories}
}

func (*PushReconciler) EventType() string { return "push" }

func (r *PushReconciler) Reconcile(ctx context.Context, ev *ghwebhook.Event, event *github.PushEvent) error {
	if event.GetRef() != "refs/heads/"+event.GetRepo().GetDefaultBranch() {
		ev.State = ghwebhook.StateSkipped
		return nil
	}
	repositoryID, ok, err := r.repositories.Find(ctx, event.GetRepo().GetHTMLURL())
	if err != nil {
		return err
	}
	if !ok {
		ev.State = ghwebhook.StateSkipped
		return nil
	}
	ctx, err = asSystem(ctx, r.tracker)
	if err != nil {
		return err
	}

	for _, c := range event.Commits {
		for _, ref := range issue.ParseTrailer(c.GetMessage()) {
			ticket, err := r.tracker.Ticket(ctx, ref.Number)
			if errors.Is(err, issue.ErrNotFound) {
				slogger.Log.Info("The ticket referred by the commit is not found", slog.Int("number", int(ref.Number)), slog.String("sha", c.GetID()))
				continue
			} else if err != nil {
				return err
			}
			if err := ticket.LinkCommit(ctx, repositoryID, c.GetID(), subject(c.GetMessage()), author(c.GetAuthor()), ref.Kind, c.GetTimestamp().Time); err != nil {
				return err
			}
			if ref.Kind == database.TrailerKindFixes && ticket.Status != database.TicketStatusDone {
				if err := ticket.Update(ctx, nil, nil, new(database.TicketStatusDone), nil, nil, nil); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func asSystem(ctx context.Context, tracker *issue.Tracker) (context.Context, error) {
	system, err := tracker.System(ctx)
	if err != nil {
		return nil, err
	}
	return issue.WithActor(ctx, system), nil
}

func subject(message string) string {
	s, _, _ := strings.Cut(message, "\n")
	return strings.TrimSpace(s)
}

// author returns the login of the author on GitHub, or the name if the author is not a user of GitHub.
func author(a *github.CommitAuthor) string {
	if a.GetLogin() != "" {
		return a.GetLogin()
	}
	return a.GetName()
}

const mysqlDuplicateEntry = 1062

// store stores the webhook events in the github_event table.
type store struct {
	dao dao.GithubEventInterface
}

var _ ghwebhook.Store = (*store)(nil)

func newStore(d dao.GithubEventInterface) *store {
	return &store{dao: d}
}

func (s *store) Create(ctx context.Context, ev *ghwebhook.Event) error {
	row, err := s.dao.Create(ctx, &database.GithubEvent{
		DeliveryId: ev.DeliveryId,
		EventType:  ev.EventType,
		Action:     ev.Action,
		Payload:    ev.Payload,
		State:      database.GithubEventState(ev.State),
		Status:     ev.Status,
		LastError:  ev.LastError,
	})
	if err != nil {
		if me, ok := errors.AsType[*mysql.MySQLError](err); ok && me.Number == mysqlDuplicateEntry {
			return ghwebhook.ErrDuplicateDelivery
		}
		return xerrors.WithStack(err)
	}
	ev.Id = row.Id
	return nil
}

func (s *store) ListByState(ctx context.Context, states ...ghwebhook.State) ([]*ghwebhook.Event, error) {
	var res []*ghwebhook.Event
	for _, st := range states {
		rows, err := s.dao.ListByState(ctx, uint32(st))
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		for _, r := range rows {
			res = append(res, toEvent(r))
		}
	}
	return res, nil
}

func (s *store) Claim(ctx context.Context, ev *ghwebhook.Event) (bool, error) {
	var claimed bool
	err := s.dao.Tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(
			ctx,
			"UPDATE `github_event` SET `state` = ?, `updated_at` = ? WHERE `id` = ? AND `state` IN (?, ?)",
			database.GithubEventStateProcessing, time.Now(), ev.Id, database.GithubEventStatePending, database.GithubEventStateFailed,
		)
		if err != nil {
			return xerrors.WithStack(err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return xerrors.WithStack(err)
		}
		claimed = n == 1
		return nil
	})
	if err != nil {
		return false, err
	}
	return claimed, nil
}

func (s *store) Update(ctx context.Context, ev *ghwebhook.Event) error {
	return s.dao.Tx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(
			ctx,
			"UPDATE `github_event` SET `state` = ?, `status` = ?, `last_error` = ?, `updated_at` = ? WHERE `id` = ?",
			database.GithubEventState(ev.State), ev.Status, ev.LastError, time.Now(), ev.Id,
		)
		if err != nil {
			return xerrors.WithStack(err)
		}
		return nil
	})
}

func toEvent(r *database.GithubEvent) *ghwebhook.Event {
	updatedAt := r.CreatedAt
	if r.UpdatedAt != nil {
		updatedAt = *r.UpdatedAt
	}
	return &ghwebhook.Event{
		Id:         r.Id,
		DeliveryId: r.DeliveryId,
		EventType:  r.EventType,
		Action:     r.Action,
		Payload:    r.Payload,
		State:      ghwebhook.State(r.State),
		Status:     r.Status,
		LastError:  r.LastError,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  updatedAt,
	}
}
