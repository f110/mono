package issue

import (
	"context"
	"database/sql"
	"slices"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/issue/database"
	"go.f110.dev/mono/go/issue/database/dao"
)

// Comment is a comment of the ticket. Any actor can edit and delete any comment because the revisions are kept.
type Comment struct {
	*database.Comment
	tracker *Tracker
}

// Revision is the body before the edit recorded by the revision Id. ActorId is the actor who made the edit.
type Revision struct {
	Id        int32
	Body      string
	ActorId   int32
	CreatedAt time.Time
}

func (c *Comment) Edit(ctx context.Context, body string) error {
	actor, err := c.tracker.ticketOperator(ctx)
	if err != nil {
		return err
	}
	if body == "" {
		return xerrors.WithMessage(ErrInvalidArgument, "body is required")
	}
	if c.Body == body {
		return nil
	}
	texts, err := c.mentionTexts(ctx)
	if err != nil {
		return err
	}

	updated := c.Copy()
	updated.ResetMark()
	updated.Body = body
	err = c.tracker.tx(ctx, func(tx *sql.Tx) error {
		if err := c.tracker.dao.Comment.Update(ctx, updated, dao.WithTx(tx)); err != nil {
			return xerrors.WithStack(err)
		}
		rev, err := c.tracker.dao.CommentRevision.Create(ctx, &database.CommentRevision{CommentId: c.Id, Patch: reversePatch(c.Body, body)}, dao.WithTx(tx))
		if err != nil {
			return xerrors.WithStack(err)
		}
		if err := c.tracker.record(ctx, tx, c.TicketId, actor, database.ActivityKindCommentEdited, "", "", &rev.Id); err != nil {
			return err
		}
		if err := c.tracker.syncMentions(ctx, tx, actor, c.TicketId, append(texts, body)...); err != nil {
			return err
		}
		return c.tracker.touch(ctx, tx, c.TicketId)
	})
	if err != nil {
		return err
	}
	return c.reload(ctx)
}

// Delete removes the comment logically.
func (c *Comment) Delete(ctx context.Context) error {
	actor, err := c.tracker.ticketOperator(ctx)
	if err != nil {
		return err
	}
	texts, err := c.mentionTexts(ctx)
	if err != nil {
		return err
	}

	updated := c.Copy()
	updated.ResetMark()
	updated.DeletedAt = new(time.Now())
	err = c.tracker.tx(ctx, func(tx *sql.Tx) error {
		if err := c.tracker.dao.Comment.Update(ctx, updated, dao.WithTx(tx)); err != nil {
			return xerrors.WithStack(err)
		}
		if err := c.tracker.record(ctx, tx, c.TicketId, actor, database.ActivityKindCommentDeleted, "", "", &c.Id); err != nil {
			return err
		}
		if err := c.tracker.syncMentions(ctx, tx, actor, c.TicketId, texts...); err != nil {
			return err
		}
		return c.tracker.touch(ctx, tx, c.TicketId)
	})
	if err != nil {
		return err
	}
	return c.reload(ctx)
}

// Revisions returns the body before each edit in the order of the edits.
func (c *Comment) Revisions(ctx context.Context) ([]*Revision, error) {
	revs, err := c.tracker.dao.CommentRevision.ListByCommentId(ctx, c.Id)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	editors, err := c.tracker.editors(ctx, c.TicketId, database.ActivityKindCommentEdited)
	if err != nil {
		return nil, err
	}
	return restoreRevisions(c.Body, revs, editors, func(r *database.CommentRevision) (int32, string, time.Time) { return r.Id, r.Patch, r.CreatedAt })
}

// mentionTexts returns the body of the ticket and the comments of the ticket which are not deleted except the comment.
func (c *Comment) mentionTexts(ctx context.Context) ([]string, error) {
	ticket, err := c.tracker.dao.Ticket.Select(ctx, c.TicketId)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	comments, err := c.tracker.commentBodies(ctx, c.TicketId, c.Id)
	if err != nil {
		return nil, err
	}
	return append(comments, ticket.Body), nil
}

func (c *Comment) reload(ctx context.Context) error {
	comment, err := c.tracker.dao.Comment.Select(ctx, c.Id)
	if err != nil {
		return xerrors.WithStack(err)
	}
	c.Comment = comment
	return nil
}

// reversePatch returns the patch which restores before from after.
func reversePatch(before, after string) string {
	dmp := diffmatchpatch.New()
	return dmp.PatchToText(dmp.PatchMake(after, before))
}

// restoreRevisions restores the body before each edit. revs are in the order of the edits, and latest is the body
// after the last edit. editors maps the id of the revision to the actor of the edit. fields returns the id, the reverse
// patch and the time of the revision.
func restoreRevisions[T any](latest string, revs []T, editors map[int32]int32, fields func(T) (int32, string, time.Time)) ([]*Revision, error) {
	dmp := diffmatchpatch.New()
	res := make([]*Revision, len(revs))
	body := latest
	for i := len(revs) - 1; i >= 0; i-- {
		id, patch, createdAt := fields(revs[i])
		p, err := dmp.PatchFromText(patch)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		restored, applied := dmp.PatchApply(p, body)
		if slices.Contains(applied, false) {
			return nil, xerrors.Define("failed to apply the patch of the revision").WithStack()
		}
		body = restored
		res[i] = &Revision{Id: id, Body: body, ActorId: editors[id], CreatedAt: createdAt}
	}
	return res, nil
}
