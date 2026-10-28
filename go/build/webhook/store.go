package webhook

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	ghwebhook "go.f110.dev/mono/go/github/webhook"
)

const mysqlDuplicateEntry = 1062

// store stores webhook events in the github_event table.
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
		if isDuplicateEntry(err) {
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

func isDuplicateEntry(err error) bool {
	if me, ok := errors.AsType[*mysql.MySQLError](err); ok {
		return me.Number == mysqlDuplicateEntry
	}
	return strings.Contains(err.Error(), "Duplicate entry")
}
