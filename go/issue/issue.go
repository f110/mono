// Package issue implements the issue tracker.
//
// Tracker is the entry point. Ticket and Comment have the operations to themselves. Every operation records the
// activity to the ticket in the same transaction. The actor of the operations is passed by context.Context with
// WithActor or WithHuman.
package issue

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/issue/database"
	"go.f110.dev/mono/go/issue/database/dao"
	"go.f110.dev/mono/go/issue/filter"
)

var (
	ErrUnauthenticated  = xerrors.Define("unauthenticated")
	ErrPermissionDenied = xerrors.Define("permission denied")
	ErrNotFound         = xerrors.Define("not found")
	ErrInvalidArgument  = xerrors.Define("invalid argument")
)

// maxNameLength is the length of VARCHAR(255) which stores the title and the name of the component.
const maxNameLength = 255

type actorKey struct{}

type subjectKey struct{}

type agentSubjectKey struct{}

// WithActor returns the context that carries the actor of the operations.
func WithActor(ctx context.Context, actor *database.Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}

// WithHuman returns the context that carries the subject which is the user id of heimdallr. The human actor of the
// subject is looked up only when the operation needs it, and it is created at the first time.
func WithHuman(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, subjectKey{}, subject)
}

// WithAgent returns the context that carries the subject of the owner of the agent. The agent actor of the owner is
// looked up only when the operation needs it, and it is created with the owner at the first time.
func WithAgent(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, agentSubjectKey{}, subject)
}

func actorFrom(ctx context.Context) (*database.Actor, bool) {
	a, ok := ctx.Value(actorKey{}).(*database.Actor)
	return a, ok && a != nil
}

func subjectFrom(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(subjectKey{}).(string)
	return s, ok && s != ""
}

func agentSubjectFrom(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(agentSubjectKey{}).(string)
	return s, ok && s != ""
}

type Tracker struct {
	dao  dao.Options
	conn *sql.DB
}

func NewTracker(daos dao.Options) *Tracker {
	return &Tracker{dao: daos, conn: daos.RawConnection}
}

// Create creates the ticket in OPEN. componentID and assigneeID are optional.
func (t *Tracker) Create(ctx context.Context, title, body string, priority database.Priority, componentID, assigneeID *int32) (*Ticket, error) {
	actor, err := t.ticketOperator(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateName("title", title); err != nil {
		return nil, err
	}
	if err := t.validateReferences(ctx, componentID, assigneeID); err != nil {
		return nil, err
	}

	var id int32
	err = t.tx(ctx, func(tx *sql.Tx) error {
		ticket, err := t.dao.Ticket.Create(ctx, &database.Ticket{
			Title:          title,
			Body:           body,
			Status:         database.TicketStatusOpen,
			Priority:       priority,
			ComponentId:    componentID,
			AssigneeId:     assigneeID,
			ReporterId:     actor.Id,
			LastActivityAt: time.Now(),
		}, dao.WithTx(tx))
		if err != nil {
			return xerrors.WithStack(err)
		}
		id = ticket.Id
		if err := t.record(ctx, tx, ticket.Id, actor, database.ActivityKindCreate, "", "", nil); err != nil {
			return err
		}
		return t.syncMentions(ctx, tx, actor, ticket.Id, body)
	})
	if err != nil {
		return nil, err
	}
	return t.Ticket(ctx, id)
}

// Ticket returns the ticket of the number.
func (t *Tracker) Ticket(ctx context.Context, number int32) (*Ticket, error) {
	if err := authenticated(ctx); err != nil {
		return nil, err
	}
	ticket, err := t.dao.Ticket.Select(ctx, number)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound.WithStack()
	} else if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return &Ticket{Ticket: ticket, tracker: t}, nil
}

// Find returns the tickets which match the filter in the order of orderBy. The filter is written in AIP-160. See the
// package filter for the fields. orderBy is written in AIP-132, and the fields are the keys of orderFields. The tickets
// are ordered by the last activity in the descending order if orderBy is empty.
func (t *Tracker) Find(ctx context.Context, f, orderBy string) ([]*Ticket, error) {
	if err := authenticated(ctx); err != nil {
		return nil, err
	}
	parsed, err := filter.Parse(f)
	if err != nil {
		return nil, xerrors.WithMessage(ErrInvalidArgument, err.Error())
	}
	order, err := orderClause(orderBy)
	if err != nil {
		return nil, err
	}

	query := "SELECT `id` FROM `ticket`"
	where, args := parsed.Where()
	if where != "" {
		query += " WHERE " + where
	}
	query += " ORDER BY " + order
	rows, err := t.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	defer rows.Close()
	var ids []int32
	for rows.Next() {
		var id int32
		if err := rows.Scan(&id); err != nil {
			return nil, xerrors.WithStack(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, xerrors.WithStack(err)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	tickets, err := t.dao.Ticket.SelectMulti(ctx, ids...)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	byID := make(map[int32]*database.Ticket, len(tickets))
	for _, v := range tickets {
		byID[v.Id] = v
	}
	res := make([]*Ticket, 0, len(ids))
	for _, id := range ids {
		if v, ok := byID[id]; ok {
			res = append(res, &Ticket{Ticket: v, tracker: t})
		}
	}
	return res, nil
}

// Comment returns the comment which is not deleted.
func (t *Tracker) Comment(ctx context.Context, id int32) (*Comment, error) {
	if err := authenticated(ctx); err != nil {
		return nil, err
	}
	c, err := t.dao.Comment.Select(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound.WithStack()
	} else if err != nil {
		return nil, xerrors.WithStack(err)
	}
	if c.DeletedAt != nil {
		return nil, ErrNotFound.WithStack()
	}
	return &Comment{Comment: c, tracker: t}, nil
}

func (t *Tracker) Components(ctx context.Context) ([]*database.Component, error) {
	if err := authenticated(ctx); err != nil {
		return nil, err
	}
	components, err := t.dao.Component.ListAll(ctx)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return components, nil
}

// SaveComponent creates the component if id is 0, otherwise renames it. Only the human can manage the components.
func (t *Tracker) SaveComponent(ctx context.Context, id int32, name string) (*database.Component, error) {
	actor, err := t.actor(ctx)
	if err != nil {
		return nil, err
	}
	if actor.Kind != database.ActorKindHuman {
		return nil, ErrPermissionDenied.WithStack()
	}
	if err := validateName("name", name); err != nil {
		return nil, err
	}

	if id == 0 {
		c, err := t.dao.Component.Create(ctx, &database.Component{Name: name})
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		return c, nil
	}
	c, err := t.dao.Component.Select(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound.WithStack()
	} else if err != nil {
		return nil, xerrors.WithStack(err)
	}
	c.Name = name
	if err := t.dao.Component.Update(ctx, c); err != nil {
		return nil, xerrors.WithStack(err)
	}
	return c, nil
}

// Me returns the actor of the operations.
func (t *Tracker) Me(ctx context.Context) (*database.Actor, error) {
	return t.actor(ctx)
}

// human returns the human actor of subject. The actor is created at the first access, so the human does not have to
// be registered in advance.
func (t *Tracker) human(ctx context.Context, subject string) (*database.Actor, error) {
	a, err := t.dao.Actor.SelectBySubject(ctx, subject)
	if err == nil {
		return a, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, xerrors.WithStack(err)
	}

	created, err := t.dao.Actor.Create(ctx, &database.Actor{Kind: database.ActorKindHuman, Name: subject, Subject: &subject})
	if err == nil {
		return t.dao.Actor.Select(ctx, created.Id)
	}
	a, selectErr := t.dao.Actor.SelectBySubject(ctx, subject)
	if selectErr != nil {
		return nil, xerrors.WithStack(err)
	}
	return a, nil
}

// agent returns the agent actor which the human of subject owns. The agent and the owner are created at the first
// access.
func (t *Tracker) agent(ctx context.Context, subject string) (*database.Actor, error) {
	owner, err := t.human(ctx, subject)
	if err != nil {
		return nil, err
	}
	a, err := t.dao.Actor.SelectByOwner(ctx, owner.Id)
	if err == nil {
		return a, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, xerrors.WithStack(err)
	}

	created, err := t.dao.Actor.Create(ctx, &database.Actor{Kind: database.ActorKindAgent, Name: owner.Name + " (agent)", OwnerId: &owner.Id})
	if err == nil {
		return t.dao.Actor.Select(ctx, created.Id)
	}
	a, selectErr := t.dao.Actor.SelectByOwner(ctx, owner.Id)
	if selectErr != nil {
		return nil, xerrors.WithStack(err)
	}
	return a, nil
}

// System returns the system actor which operates the tickets by the events from GitHub. The actor is created at the
// first access. If the actors are created concurrently, the actor of the smallest id is the system actor.
func (t *Tracker) System(ctx context.Context) (*database.Actor, error) {
	actors, err := t.dao.Actor.ListByKind(ctx, uint32(database.ActorKindSystem))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	if len(actors) > 0 {
		return slices.MinFunc(actors, func(a, b *database.Actor) int { return cmp.Compare(a.Id, b.Id) }), nil
	}
	a, err := t.dao.Actor.Create(ctx, &database.Actor{Kind: database.ActorKindSystem, Name: "system"})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return a, nil
}

// UpdatePullRequest updates the title, the state and the head of the pull request of all tickets which the pull request
// is linked to.
func (t *Tracker) UpdatePullRequest(ctx context.Context, repositoryID, number int32, title string, state database.PullRequestState, headSHA string) error {
	if _, err := t.actor(ctx); err != nil {
		return err
	}
	links, err := t.dao.PullRequestLink.ListByRepositoryIdAndNumber(ctx, repositoryID, number)
	if err != nil {
		return xerrors.WithStack(err)
	}
	for _, v := range links {
		if v.Title == title && v.State == state && v.HeadSha == headSHA {
			continue
		}
		v.Title, v.State, v.HeadSha = title, state, headSHA
		if err := t.dao.PullRequestLink.Update(ctx, v); err != nil {
			return xerrors.WithStack(err)
		}
	}
	return nil
}

// CommitLinksWithoutChangeID returns the commit links whose change id is not looked up yet.
func (t *Tracker) CommitLinksWithoutChangeID(ctx context.Context) ([]*database.CommitLink, error) {
	if err := authenticated(ctx); err != nil {
		return nil, err
	}
	links, err := t.dao.CommitLink.ListWithoutChangeId(ctx)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return links, nil
}

// SetChangeID sets the change id of the commit link of id. changeID is empty if the commit has no change id.
func (t *Tracker) SetChangeID(ctx context.Context, id int32, changeID string) error {
	if _, err := t.actor(ctx); err != nil {
		return err
	}
	link, err := t.dao.CommitLink.Select(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound.WithStack()
	} else if err != nil {
		return xerrors.WithStack(err)
	}
	link.ChangeId = &changeID
	if err := t.dao.CommitLink.Update(ctx, link); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (t *Tracker) Actors(ctx context.Context) ([]*database.Actor, error) {
	if err := authenticated(ctx); err != nil {
		return nil, err
	}
	actors, err := t.dao.Actor.ListAll(ctx)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return actors, nil
}

func (t *Tracker) tx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := t.conn.BeginTx(ctx, nil)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if err := fn(tx); err != nil {
		if rErr := tx.Rollback(); rErr != nil {
			return xerrors.WithStack(rErr)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (t *Tracker) record(ctx context.Context, tx *sql.Tx, ticketID int32, actor *database.Actor, kind database.ActivityKind, from, to string, refID *int32) error {
	_, err := t.dao.Activity.Create(ctx, &database.Activity{
		TicketId:  ticketID,
		ActorId:   actor.Id,
		Kind:      kind,
		FromValue: from,
		ToValue:   to,
		RefId:     refID,
	}, dao.WithTx(tx))
	if err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

// editors returns the actors of the edits of the ticket by the id of the revision which the activity of kind refers to.
func (t *Tracker) editors(ctx context.Context, ticketID int32, kind database.ActivityKind) (map[int32]int32, error) {
	activities, err := t.dao.Activity.ListByTicketId(ctx, ticketID)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	editors := make(map[int32]int32)
	for _, v := range activities {
		if v.Kind == kind && v.RefId != nil {
			editors[*v.RefId] = v.ActorId
		}
	}
	return editors, nil
}

// touch updates last_activity_at of the ticket. Update reports sql.ErrNoRows if the time is not changed within the
// precision of the column, and it is not an error here because the ticket exists.
func (t *Tracker) touch(ctx context.Context, tx *sql.Tx, ticketID int32) error {
	ticket, err := t.dao.Ticket.Select(ctx, ticketID)
	if err != nil {
		return xerrors.WithStack(err)
	}
	ticket.LastActivityAt = time.Now()
	if err := t.dao.Ticket.Update(ctx, ticket, dao.WithTx(tx)); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return xerrors.WithStack(err)
	}
	return nil
}

// syncMentions makes the mentions from the ticket of from match the tickets mentioned in texts, which are the body and
// the comments of the ticket after the operation. The mentioned ticket records the activity and its last activity is
// updated when the mention is added or removed.
func (t *Tracker) syncMentions(ctx context.Context, tx *sql.Tx, actor *database.Actor, from int32, texts ...string) error {
	var numbers []int32
	for _, v := range texts {
		numbers = append(numbers, parseMentions(v)...)
	}
	numbers = slices.DeleteFunc(numbers, func(n int32) bool { return n == from })
	var targets []int32
	if len(numbers) > 0 {
		tickets, err := t.dao.Ticket.SelectMulti(ctx, numbers...)
		if err != nil {
			return xerrors.WithStack(err)
		}
		for _, v := range tickets {
			targets = append(targets, v.Id)
		}
	}
	existing, err := t.dao.Mention.ListByFromId(ctx, from)
	if err != nil {
		return xerrors.WithStack(err)
	}

	value := strconv.Itoa(int(from))
	for _, v := range existing {
		if slices.Contains(targets, v.ToId) {
			continue
		}
		if err := t.dao.Mention.Delete(ctx, v.Id, dao.WithTx(tx)); err != nil {
			return xerrors.WithStack(err)
		}
		if err := t.record(ctx, tx, v.ToId, actor, database.ActivityKindMentionRemoved, "", value, nil); err != nil {
			return err
		}
		if err := t.touch(ctx, tx, v.ToId); err != nil {
			return err
		}
	}
	for _, to := range targets {
		if slices.ContainsFunc(existing, func(m *database.Mention) bool { return m.ToId == to }) {
			continue
		}
		if _, err := t.dao.Mention.Create(ctx, &database.Mention{FromId: from, ToId: to}, dao.WithTx(tx)); err != nil {
			return xerrors.WithStack(err)
		}
		if err := t.record(ctx, tx, to, actor, database.ActivityKindMentioned, "", value, nil); err != nil {
			return err
		}
		if err := t.touch(ctx, tx, to); err != nil {
			return err
		}
	}
	return nil
}

// commentBodies returns the bodies of the comments of the ticket which are not deleted except the comment of exclude.
func (t *Tracker) commentBodies(ctx context.Context, ticketID, exclude int32) ([]string, error) {
	comments, err := t.dao.Comment.ListByTicketId(ctx, ticketID)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	var res []string
	for _, v := range comments {
		if v.DeletedAt == nil && v.Id != exclude {
			res = append(res, v.Body)
		}
	}
	return res, nil
}

func (t *Tracker) validateReferences(ctx context.Context, componentID, assigneeID *int32) error {
	if componentID != nil {
		if _, err := t.dao.Component.Select(ctx, *componentID); errors.Is(err, sql.ErrNoRows) {
			return xerrors.WithMessage(ErrInvalidArgument, "component not found")
		} else if err != nil {
			return xerrors.WithStack(err)
		}
	}
	if assigneeID != nil {
		if _, err := t.dao.Actor.Select(ctx, *assigneeID); errors.Is(err, sql.ErrNoRows) {
			return xerrors.WithMessage(ErrInvalidArgument, "assignee not found")
		} else if err != nil {
			return xerrors.WithStack(err)
		}
	}
	return nil
}

// authenticated reports whether ctx carries the actor or the subject without looking up the actor.
func authenticated(ctx context.Context) error {
	if _, ok := actorFrom(ctx); ok {
		return nil
	}
	if _, ok := subjectFrom(ctx); ok {
		return nil
	}
	if _, ok := agentSubjectFrom(ctx); ok {
		return nil
	}
	return ErrUnauthenticated.WithStack()
}

func (t *Tracker) actor(ctx context.Context) (*database.Actor, error) {
	if actor, ok := actorFrom(ctx); ok {
		return actor, nil
	}
	if subject, ok := subjectFrom(ctx); ok {
		return t.human(ctx, subject)
	}
	if subject, ok := agentSubjectFrom(ctx); ok {
		return t.agent(ctx, subject)
	}
	return nil, ErrUnauthenticated.WithStack()
}

// ticketOperator returns the actor who can operate the tickets. The system actor can not.
func (t *Tracker) ticketOperator(ctx context.Context) (*database.Actor, error) {
	actor, err := t.actor(ctx)
	if err != nil {
		return nil, err
	}
	if actor.Kind == database.ActorKindSystem {
		return nil, ErrPermissionDenied.WithStack()
	}
	return actor, nil
}

func validateName(field, value string) error {
	if value == "" {
		return xerrors.WithMessagef(ErrInvalidArgument, "%s is required", field)
	}
	if utf8.RuneCountInString(value) > maxNameLength {
		return xerrors.WithMessagef(ErrInvalidArgument, "%s must be at most %d characters", field, maxNameLength)
	}
	return nil
}

// orderFields is the fields which the tickets can be ordered by. absent is the expression which is true if the field is
// not set, so that the ticket without the field comes last in both directions.
var orderFields = map[string]struct {
	expr   string
	absent string
}{
	"number":           {expr: "`id`"},
	"status":           {expr: "`status`"},
	"priority":         {expr: "`priority`", absent: "`priority` = 0"},
	"component":        {expr: "(SELECT `name` FROM `component` WHERE `component`.`id` = `ticket`.`component_id`)", absent: "`component_id` IS NULL"},
	"assignee":         {expr: "(SELECT `name` FROM `actor` WHERE `actor`.`id` = `ticket`.`assignee_id`)", absent: "`assignee_id` IS NULL"},
	"created_at":       {expr: "`created_at`"},
	"last_activity_at": {expr: "`last_activity_at`"},
}

// orderClause returns ORDER BY clause of orderBy. The ticket number in the descending order breaks the tie.
func orderClause(orderBy string) (string, error) {
	if strings.TrimSpace(orderBy) == "" {
		orderBy = "last_activity_at desc"
	}
	var exprs []string
	for _, v := range strings.Split(orderBy, ",") {
		terms := strings.Fields(v)
		if len(terms) == 0 || len(terms) > 2 || (len(terms) == 2 && terms[1] != "desc") {
			return "", xerrors.WithMessagef(ErrInvalidArgument, "invalid order_by: %s", orderBy)
		}
		field, ok := orderFields[terms[0]]
		if !ok {
			return "", xerrors.WithMessagef(ErrInvalidArgument, "unknown field of order_by: %s", terms[0])
		}
		direction := "ASC"
		if len(terms) == 2 {
			direction = "DESC"
		}
		if field.absent != "" {
			exprs = append(exprs, field.absent)
		}
		exprs = append(exprs, field.expr+" "+direction)
	}
	return strings.Join(append(exprs, "`id` DESC"), ", "), nil
}
