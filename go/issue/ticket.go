package issue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/issue/database"
	"go.f110.dev/mono/go/issue/database/dao"
)

// Ticket is a ticket of the tracker. Id is the ticket number.
type Ticket struct {
	*database.Ticket
	tracker *Tracker
}

// Update updates the fields which are not nil. 0 for componentID or assigneeID clears the field. The system actor can
// change only the status.
func (t *Ticket) Update(ctx context.Context, title, body *string, status *database.TicketStatus, priority *database.Priority, componentID, assigneeID *int32) error {
	actor, err := t.tracker.actor(ctx)
	if err != nil {
		return err
	}
	if actor.Kind == database.ActorKindSystem && (title != nil || body != nil || priority != nil || componentID != nil || assigneeID != nil) {
		return ErrPermissionDenied.WithStack()
	}
	if title != nil {
		if err := validateName("title", *title); err != nil {
			return err
		}
	}
	newComponentID, newAssigneeID := nonZero(componentID), nonZero(assigneeID)
	if err := t.tracker.validateReferences(ctx, newComponentID, newAssigneeID); err != nil {
		return err
	}

	type change struct {
		kind     database.ActivityKind
		from, to string
	}
	var changes []change
	updated := t.Copy()
	updated.ResetMark()
	if title != nil && *title != updated.Title {
		changes = append(changes, change{kind: database.ActivityKindTitle, from: updated.Title, to: *title})
		updated.Title = *title
	}
	if status != nil && *status != updated.Status {
		changes = append(changes, change{kind: database.ActivityKindStatus, from: statusName(updated.Status), to: statusName(*status)})
		updated.Status = *status
	}
	if priority != nil && *priority != updated.Priority {
		changes = append(changes, change{kind: database.ActivityKindPriority, from: priorityName(updated.Priority), to: priorityName(*priority)})
		updated.Priority = *priority
	}
	if componentID != nil && !equalID(updated.ComponentId, newComponentID) {
		changes = append(changes, change{kind: database.ActivityKindComponent, from: idString(updated.ComponentId), to: idString(newComponentID)})
		updated.ComponentId = newComponentID
	}
	if assigneeID != nil && !equalID(updated.AssigneeId, newAssigneeID) {
		changes = append(changes, change{kind: database.ActivityKindAssignee, from: idString(updated.AssigneeId), to: idString(newAssigneeID)})
		updated.AssigneeId = newAssigneeID
	}
	var bodyPatch *string
	if body != nil && *body != updated.Body {
		bodyPatch = new(reversePatch(updated.Body, *body))
		updated.Body = *body
	}
	if len(changes) == 0 && bodyPatch == nil {
		return nil
	}
	updated.LastActivityAt = time.Now()

	err = t.tracker.tx(ctx, func(tx *sql.Tx) error {
		if err := t.tracker.dao.Ticket.Update(ctx, updated, dao.WithTx(tx)); err != nil {
			return xerrors.WithStack(err)
		}
		for _, c := range changes {
			if err := t.tracker.record(ctx, tx, t.Id, actor, c.kind, c.from, c.to, nil); err != nil {
				return err
			}
		}
		if bodyPatch != nil {
			rev, err := t.tracker.dao.TicketBodyRevision.Create(ctx, &database.TicketBodyRevision{TicketId: t.Id, Patch: *bodyPatch}, dao.WithTx(tx))
			if err != nil {
				return xerrors.WithStack(err)
			}
			if err := t.tracker.record(ctx, tx, t.Id, actor, database.ActivityKindBody, "", "", &rev.Id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return t.reload(ctx)
}

// Relate adds the relation from the ticket to the ticket of the number to. relates_to is stored with the smaller
// number as from because it is symmetric.
func (t *Ticket) Relate(ctx context.Context, relationType database.RelationType, to int32) (*database.Relation, error) {
	actor, err := t.tracker.ticketOperator(ctx)
	if err != nil {
		return nil, err
	}
	if _, ok := relationNames[relationType]; !ok {
		return nil, xerrors.WithMessage(ErrInvalidArgument, "unknown relation type")
	}
	if t.Id == to {
		return nil, xerrors.WithMessage(ErrInvalidArgument, "the ticket can not relate to itself")
	}
	if _, err := t.tracker.Ticket(ctx, to); err != nil {
		return nil, err
	}
	from, to := normalizeRelation(t.Id, relationType, to)
	if _, err := t.tracker.dao.Relation.SelectByFromIdAndRelationTypeAndToId(ctx, from, uint32(relationType), to); err == nil {
		return nil, xerrors.WithMessage(ErrInvalidArgument, "the relation already exists")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, xerrors.WithStack(err)
	}

	var id int32
	err = t.tracker.tx(ctx, func(tx *sql.Tx) error {
		r, err := t.tracker.dao.Relation.Create(ctx, &database.Relation{FromId: from, RelationType: relationType, ToId: to}, dao.WithTx(tx))
		if err != nil {
			return xerrors.WithStack(err)
		}
		id = r.Id
		return t.recordRelation(ctx, tx, actor, database.ActivityKindRelationAdded, r)
	})
	if err != nil {
		return nil, err
	}
	if err := t.reload(ctx); err != nil {
		return nil, err
	}
	r, err := t.tracker.dao.Relation.Select(ctx, id)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return r, nil
}

// Unrelate removes the relation from the ticket to the ticket of the number to.
func (t *Ticket) Unrelate(ctx context.Context, relationType database.RelationType, to int32) error {
	actor, err := t.tracker.ticketOperator(ctx)
	if err != nil {
		return err
	}
	from, to := normalizeRelation(t.Id, relationType, to)
	r, err := t.tracker.dao.Relation.SelectByFromIdAndRelationTypeAndToId(ctx, from, uint32(relationType), to)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound.WithStack()
	} else if err != nil {
		return xerrors.WithStack(err)
	}

	err = t.tracker.tx(ctx, func(tx *sql.Tx) error {
		if err := t.tracker.dao.Relation.Delete(ctx, r.Id, dao.WithTx(tx)); err != nil {
			return xerrors.WithStack(err)
		}
		return t.recordRelation(ctx, tx, actor, database.ActivityKindRelationRemoved, r)
	})
	if err != nil {
		return err
	}
	return t.reload(ctx)
}

// Relations returns the relations which the ticket is on either side of.
func (t *Ticket) Relations(ctx context.Context) ([]*database.Relation, error) {
	from, err := t.tracker.dao.Relation.ListByFromId(ctx, t.Id)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	to, err := t.tracker.dao.Relation.ListByToId(ctx, t.Id)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return append(from, to...), nil
}

func (t *Ticket) AddComment(ctx context.Context, body string) (*Comment, error) {
	actor, err := t.tracker.ticketOperator(ctx)
	if err != nil {
		return nil, err
	}
	if body == "" {
		return nil, xerrors.WithMessage(ErrInvalidArgument, "body is required")
	}

	var id int32
	err = t.tracker.tx(ctx, func(tx *sql.Tx) error {
		c, err := t.tracker.dao.Comment.Create(ctx, &database.Comment{TicketId: t.Id, ActorId: actor.Id, Body: body}, dao.WithTx(tx))
		if err != nil {
			return xerrors.WithStack(err)
		}
		id = c.Id
		if err := t.tracker.record(ctx, tx, t.Id, actor, database.ActivityKindComment, "", "", &c.Id); err != nil {
			return err
		}
		return t.tracker.touch(ctx, tx, t.Id)
	})
	if err != nil {
		return nil, err
	}
	if err := t.reload(ctx); err != nil {
		return nil, err
	}
	return t.tracker.Comment(ctx, id)
}

// Comments returns the comments which are not deleted.
func (t *Ticket) Comments(ctx context.Context) ([]*Comment, error) {
	comments, err := t.tracker.dao.Comment.ListByTicketId(ctx, t.Id)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	comments = slices.DeleteFunc(comments, func(c *database.Comment) bool { return c.DeletedAt != nil })
	return enumerable.Map(comments, func(c *database.Comment) *Comment { return &Comment{Comment: c, tracker: t.tracker} }), nil
}

// Activities returns the activities in the order of the operations.
func (t *Ticket) Activities(ctx context.Context) ([]*database.Activity, error) {
	activities, err := t.tracker.dao.Activity.ListByTicketId(ctx, t.Id)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return activities, nil
}

// Revisions returns the body before each edit in the order of the edits.
func (t *Ticket) Revisions(ctx context.Context) ([]*Revision, error) {
	revs, err := t.tracker.dao.TicketBodyRevision.ListByTicketId(ctx, t.Id)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	editors, err := t.tracker.editors(ctx, t.Id, database.ActivityKindBody)
	if err != nil {
		return nil, err
	}
	return restoreRevisions(t.Body, revs, editors, func(r *database.TicketBodyRevision) (int32, string, time.Time) { return r.Id, r.Patch, r.CreatedAt })
}

func (t *Ticket) reload(ctx context.Context) error {
	ticket, err := t.tracker.dao.Ticket.Select(ctx, t.Id)
	if err != nil {
		return xerrors.WithStack(err)
	}
	t.Ticket = ticket
	return nil
}

// recordRelation records the activity to both tickets of the relation. The value is "<type>:<the other ticket>" from
// the point of view of each ticket.
func (t *Ticket) recordRelation(ctx context.Context, tx *sql.Tx, actor *database.Actor, kind database.ActivityKind, r *database.Relation) error {
	names := relationNames[r.RelationType]
	for _, v := range []struct {
		ticket, other int32
		name          string
	}{
		{ticket: r.FromId, other: r.ToId, name: names[0]},
		{ticket: r.ToId, other: r.FromId, name: names[1]},
	} {
		if err := t.tracker.record(ctx, tx, v.ticket, actor, kind, "", fmt.Sprintf("%s:%d", v.name, v.other), nil); err != nil {
			return err
		}
		if err := t.tracker.touch(ctx, tx, v.ticket); err != nil {
			return err
		}
	}
	return nil
}

func normalizeRelation(from int32, relationType database.RelationType, to int32) (int32, int32) {
	if relationType == database.RelationTypeRelatesTo && from > to {
		return to, from
	}
	return from, to
}

var statusNames = map[database.TicketStatus]string{
	database.TicketStatusOpen:       "open",
	database.TicketStatusInProgress: "in_progress",
	database.TicketStatusDone:       "done",
	database.TicketStatusWontFix:    "wont_fix",
}

func statusName(s database.TicketStatus) string {
	return statusNames[s]
}

func priorityName(p database.Priority) string {
	if p == database.PriorityNone {
		return ""
	}
	return fmt.Sprintf("P%d", p-database.PriorityP0)
}

// relationNames is the name of the relation from the point of view of the from ticket and the to ticket.
var relationNames = map[database.RelationType][2]string{
	database.RelationTypeParentOf:    {"parent_of", "child_of"},
	database.RelationTypeBlocks:      {"blocks", "blocked_by"},
	database.RelationTypeDuplicateOf: {"duplicate_of", "duplicated_by"},
	database.RelationTypeRelatesTo:   {"relates_to", "relates_to"},
}

func nonZero(id *int32) *int32 {
	if id == nil || *id == 0 {
		return nil
	}
	return id
}

func equalID(a, b *int32) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func idString(id *int32) string {
	if id == nil {
		return ""
	}
	return strconv.Itoa(int(*id))
}
