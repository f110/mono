package issue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"go.f110.dev/mono/go/database/dbtestutil"
	"go.f110.dev/mono/go/issue/database"
	"go.f110.dev/mono/go/issue/database/dao"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/testing/assertion"
)

var (
	mysqlPort  int
	databaseNo atomic.Int32
)

func TestMain(m *testing.M) {
	slogger.Init()
	os.Exit(runWithMySQL(m))
}

func runWithMySQL(m *testing.M) int {
	if !dbtestutil.CanUseTemporaryMySQL() {
		return m.Run()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	mysql, err := dbtestutil.NewTemporaryMySQL(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer mysql.Close()
	if err := mysql.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	mysqlPort = mysql.Port
	return m.Run()
}

type fixture struct {
	tracker *Tracker
	dao     dao.Options
	human   *database.Actor
	agent   *database.Actor
	system  *database.Actor
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	if mysqlPort == 0 {
		t.Skip("mysqld is not found")
	}
	dbName := fmt.Sprintf("issue_test_%d", databaseNo.Add(1))
	root, err := sql.Open("mysql", fmt.Sprintf("root@tcp(127.0.0.1:%d)/", mysqlPort))
	assertion.MustNoError(t, err)
	defer root.Close()
	_, err = root.Exec("CREATE DATABASE " + dbName)
	assertion.MustNoError(t, err)
	conn, err := sql.Open("mysql", fmt.Sprintf("root@tcp(127.0.0.1:%d)/%s?parseTime=true&multiStatements=true", mysqlPort, dbName))
	assertion.MustNoError(t, err)
	t.Cleanup(func() { conn.Close() })
	_, err = conn.Exec(database.Schema)
	assertion.MustNoError(t, err)

	daos := dao.NewOptions(conn)
	ctx := context.Background()
	human, err := daos.Actor.Create(ctx, &database.Actor{Kind: database.ActorKindHuman, Name: "human", Subject: new("human@example.com")})
	assertion.MustNoError(t, err)
	agent, err := daos.Actor.Create(ctx, &database.Actor{Kind: database.ActorKindAgent, Name: "agent", OwnerId: &human.Id})
	assertion.MustNoError(t, err)
	system, err := daos.Actor.Create(ctx, &database.Actor{Kind: database.ActorKindSystem, Name: "system"})
	assertion.MustNoError(t, err)
	return &fixture{tracker: NewTracker(daos), dao: daos, human: human, agent: agent, system: system}
}

func (f *fixture) as(actor *database.Actor) context.Context {
	return WithActor(context.Background(), actor)
}

func (f *fixture) create(t *testing.T, title string) *Ticket {
	t.Helper()
	ticket, err := f.tracker.Create(f.as(f.human), title, "body of "+title, database.PriorityNone, nil, nil)
	assertion.MustNoError(t, err)
	return ticket
}

func (f *fixture) component(t *testing.T, name string) *database.Component {
	t.Helper()
	c, err := f.tracker.SaveComponent(f.as(f.human), 0, name)
	assertion.MustNoError(t, err)
	return c
}

func activities(t *testing.T, ticket *Ticket) []*database.Activity {
	t.Helper()
	a, err := ticket.Activities(context.Background())
	assertion.MustNoError(t, err)
	return a
}

func mustErrorIs(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("expected %v but got %v", target, err)
	}
}

func TestCreate(t *testing.T) {
	t.Run("created ticket is open and has the history", func(t *testing.T) {
		f := newFixture(t)
		component := f.component(t, "build")

		ticket, err := f.tracker.Create(f.as(f.agent), "title", "body", database.PriorityP1, &component.Id, &f.human.Id)
		assertion.MustNoError(t, err)
		assertion.Equal(t, ticket.Title, "title")
		assertion.Equal(t, ticket.Status, database.TicketStatusOpen)
		assertion.Equal(t, ticket.Priority, database.PriorityP1)
		assertion.Equal(t, *ticket.ComponentId, component.Id)
		assertion.Equal(t, *ticket.AssigneeId, f.human.Id)
		assertion.Equal(t, ticket.ReporterId, f.agent.Id)
		assertion.False(t, ticket.CreatedAt.IsZero())
		assertion.False(t, ticket.LastActivityAt.IsZero())

		a := activities(t, ticket)
		assertion.MustLen(t, a, 1)
		assertion.Equal(t, a[0].Kind, database.ActivityKindCreate)
		assertion.Equal(t, a[0].ActorId, f.agent.Id)
		revs, err := ticket.Revisions(context.Background())
		assertion.MustNoError(t, err)
		assertion.Len(t, revs, 0)
	})

	t.Run("rejected", func(t *testing.T) {
		f := newFixture(t)
		_, err := f.tracker.Create(context.Background(), "title", "", database.PriorityNone, nil, nil)
		mustErrorIs(t, err, ErrUnauthenticated)
		_, err = f.tracker.Create(f.as(f.system), "title", "", database.PriorityNone, nil, nil)
		mustErrorIs(t, err, ErrPermissionDenied)
		_, err = f.tracker.Create(f.as(f.human), "", "", database.PriorityNone, nil, nil)
		mustErrorIs(t, err, ErrInvalidArgument)
		_, err = f.tracker.Create(f.as(f.human), strings.Repeat("あ", 256), "", database.PriorityNone, nil, nil)
		mustErrorIs(t, err, ErrInvalidArgument)
		_, err = f.tracker.Create(f.as(f.human), strings.Repeat("あ", 255), "", database.PriorityNone, nil, nil)
		assertion.MustNoError(t, err)
		_, err = f.tracker.Create(f.as(f.human), "title", "", database.PriorityNone, new(int32(100)), nil)
		mustErrorIs(t, err, ErrInvalidArgument)
		_, err = f.tracker.Create(f.as(f.human), "title", "", database.PriorityNone, nil, new(int32(100)))
		mustErrorIs(t, err, ErrInvalidArgument)
		_, err = f.tracker.Ticket(f.as(f.human), 100)
		mustErrorIs(t, err, ErrNotFound)
	})
}

func TestTicketUpdate(t *testing.T) {
	t.Run("each change is recorded", func(t *testing.T) {
		f := newFixture(t)
		component := f.component(t, "build")
		ticket := f.create(t, "before")

		err := ticket.Update(f.as(f.agent), new("after"), new("new body"), new(database.TicketStatusInProgress), new(database.PriorityP0), &component.Id, &f.agent.Id)
		assertion.MustNoError(t, err)
		assertion.Equal(t, ticket.Title, "after")
		assertion.Equal(t, ticket.Body, "new body")
		assertion.Equal(t, ticket.Status, database.TicketStatusInProgress)
		assertion.Equal(t, ticket.Priority, database.PriorityP0)
		assertion.Equal(t, *ticket.ComponentId, component.Id)
		assertion.Equal(t, *ticket.AssigneeId, f.agent.Id)

		type value struct {
			Kind     database.ActivityKind
			From, To string
		}
		var got []value
		for _, v := range activities(t, ticket)[1:] {
			assertion.Equal(t, v.ActorId, f.agent.Id)
			got = append(got, value{Kind: v.Kind, From: v.FromValue, To: v.ToValue})
		}
		assertion.Equal(t, got, []value{
			{Kind: database.ActivityKindTitle, From: "before", To: "after"},
			{Kind: database.ActivityKindStatus, From: "open", To: "in_progress"},
			{Kind: database.ActivityKindPriority, From: "", To: "P0"},
			{Kind: database.ActivityKindComponent, From: "", To: fmt.Sprint(component.Id)},
			{Kind: database.ActivityKindAssignee, From: "", To: fmt.Sprint(f.agent.Id)},
			{Kind: database.ActivityKindBody},
		})

		assertion.MustNoError(t, ticket.Update(f.as(f.human), nil, new("third body"), nil, nil, nil, nil))
		a := activities(t, ticket)
		revs, err := ticket.Revisions(context.Background())
		assertion.MustNoError(t, err)
		assertion.MustLen(t, revs, 2)
		assertion.Equal(t, revs[0].Body, "body of before")
		assertion.Equal(t, revs[1].Body, "new body")
		assertion.Equal(t, *a[6].RefId, revs[0].Id)
		assertion.Equal(t, *a[7].RefId, revs[1].Id)
	})

	t.Run("0 clears the component and the assignee", func(t *testing.T) {
		f := newFixture(t)
		component := f.component(t, "build")
		ticket, err := f.tracker.Create(f.as(f.human), "title", "", database.PriorityNone, &component.Id, &f.human.Id)
		assertion.MustNoError(t, err)

		assertion.MustNoError(t, ticket.Update(f.as(f.human), nil, nil, nil, nil, new(int32(0)), new(int32(0))))
		assertion.True(t, ticket.ComponentId == nil)
		assertion.True(t, ticket.AssigneeId == nil)
	})

	t.Run("no change records nothing", func(t *testing.T) {
		f := newFixture(t)
		ticket := f.create(t, "title")
		assertion.MustNoError(t, ticket.Update(f.as(f.human), new("title"), nil, new(database.TicketStatusOpen), nil, nil, nil))
		assertion.Len(t, activities(t, ticket), 1)
	})

	t.Run("system can change only the status", func(t *testing.T) {
		f := newFixture(t)
		ticket := f.create(t, "title")
		assertion.MustNoError(t, ticket.Update(f.as(f.system), nil, nil, new(database.TicketStatusDone), nil, nil, nil))
		assertion.Equal(t, ticket.Status, database.TicketStatusDone)
		a := activities(t, ticket)
		assertion.Equal(t, a[len(a)-1].ActorId, f.system.Id)

		err := ticket.Update(f.as(f.system), new("changed"), nil, nil, nil, nil, nil)
		mustErrorIs(t, err, ErrPermissionDenied)
	})

	t.Run("too long title", func(t *testing.T) {
		f := newFixture(t)
		ticket := f.create(t, "title")
		err := ticket.Update(f.as(f.human), new(strings.Repeat("a", 256)), nil, nil, nil, nil, nil)
		mustErrorIs(t, err, ErrInvalidArgument)
		assertion.Equal(t, ticket.Title, "title")
	})
}

func TestTicketRelate(t *testing.T) {
	t.Run("added relation is recorded to both tickets", func(t *testing.T) {
		f := newFixture(t)
		t1, t2 := f.create(t, "t1"), f.create(t, "t2")

		r, err := t1.Relate(f.as(f.human), database.RelationTypeBlocks, t2.Id)
		assertion.MustNoError(t, err)
		assertion.Equal(t, r.FromId, t1.Id)
		assertion.Equal(t, r.ToId, t2.Id)

		a1, a2 := activities(t, t1), activities(t, t2)
		assertion.Equal(t, a1[len(a1)-1].Kind, database.ActivityKindRelationAdded)
		assertion.Equal(t, a1[len(a1)-1].ToValue, fmt.Sprintf("blocks:%d", t2.Id))
		assertion.Equal(t, a2[len(a2)-1].ToValue, fmt.Sprintf("blocked_by:%d", t1.Id))

		for _, ticket := range []*Ticket{t1, t2} {
			relations, err := ticket.Relations(context.Background())
			assertion.MustNoError(t, err)
			assertion.MustLen(t, relations, 1)
			assertion.Equal(t, relations[0].Id, r.Id)
		}

		assertion.MustNoError(t, t1.Unrelate(f.as(f.human), database.RelationTypeBlocks, t2.Id))
		relations, err := t1.Relations(context.Background())
		assertion.MustNoError(t, err)
		assertion.Len(t, relations, 0)
		a1 = activities(t, t1)
		assertion.Equal(t, a1[len(a1)-1].Kind, database.ActivityKindRelationRemoved)
	})

	t.Run("relates_to is normalized", func(t *testing.T) {
		f := newFixture(t)
		t1, t2 := f.create(t, "t1"), f.create(t, "t2")

		r, err := t2.Relate(f.as(f.human), database.RelationTypeRelatesTo, t1.Id)
		assertion.MustNoError(t, err)
		assertion.Equal(t, r.FromId, t1.Id)
		assertion.Equal(t, r.ToId, t2.Id)
		_, err = t1.Relate(f.as(f.human), database.RelationTypeRelatesTo, t2.Id)
		mustErrorIs(t, err, ErrInvalidArgument)
		assertion.MustNoError(t, t2.Unrelate(f.as(f.human), database.RelationTypeRelatesTo, t1.Id))
	})

	t.Run("rejected", func(t *testing.T) {
		f := newFixture(t)
		t1 := f.create(t, "t1")
		_, err := t1.Relate(f.as(f.human), database.RelationTypeBlocks, t1.Id)
		mustErrorIs(t, err, ErrInvalidArgument)
		_, err = t1.Relate(f.as(f.human), database.RelationTypeBlocks, 100)
		mustErrorIs(t, err, ErrNotFound)
		_, err = t1.Relate(f.as(f.human), database.RelationType(100), 100)
		mustErrorIs(t, err, ErrInvalidArgument)
		mustErrorIs(t, t1.Unrelate(f.as(f.human), database.RelationTypeBlocks, 100), ErrNotFound)
	})
}

func TestComment(t *testing.T) {
	t.Run("anyone can edit and delete the comment", func(t *testing.T) {
		f := newFixture(t)
		ticket := f.create(t, "title")

		c, err := ticket.AddComment(f.as(f.human), "first")
		assertion.MustNoError(t, err)
		assertion.Equal(t, c.ActorId, f.human.Id)

		assertion.MustNoError(t, c.Edit(f.as(f.agent), "second"))
		assertion.Equal(t, c.Body, "second")
		assertion.Equal(t, c.ActorId, f.human.Id)
		assertion.MustNoError(t, c.Edit(f.as(f.human), "third"))
		revs, err := c.Revisions(context.Background())
		assertion.MustNoError(t, err)
		assertion.MustLen(t, revs, 2)
		assertion.Equal(t, revs[0].Body, "first")
		assertion.Equal(t, revs[1].Body, "second")

		assertion.MustNoError(t, c.Delete(f.as(f.agent)))
		comments, err := ticket.Comments(context.Background())
		assertion.MustNoError(t, err)
		assertion.Len(t, comments, 0)
		_, err = f.tracker.Comment(f.as(f.human), c.Id)
		mustErrorIs(t, err, ErrNotFound)

		var kinds []database.ActivityKind
		for _, v := range activities(t, ticket) {
			kinds = append(kinds, v.Kind)
		}
		assertion.Equal(t, kinds, []database.ActivityKind{
			database.ActivityKindCreate,
			database.ActivityKindComment,
			database.ActivityKindCommentEdited,
			database.ActivityKindCommentEdited,
			database.ActivityKindCommentDeleted,
		})
	})

	t.Run("comment updates the last activity", func(t *testing.T) {
		f := newFixture(t)
		ticket := f.create(t, "title")
		past := time.Now().Add(-time.Hour).Truncate(time.Second)
		ticket.LastActivityAt = past
		assertion.MustNoError(t, f.dao.Ticket.Update(context.Background(), ticket.Ticket))

		_, err := ticket.AddComment(f.as(f.human), "comment")
		assertion.MustNoError(t, err)
		assertion.True(t, ticket.LastActivityAt.After(past))
	})

	t.Run("rejected", func(t *testing.T) {
		f := newFixture(t)
		ticket := f.create(t, "title")
		_, err := ticket.AddComment(f.as(f.human), "")
		mustErrorIs(t, err, ErrInvalidArgument)
		_, err = ticket.AddComment(f.as(f.system), "comment")
		mustErrorIs(t, err, ErrPermissionDenied)
	})
}

func TestFind(t *testing.T) {
	f := newFixture(t)
	component := f.component(t, "build")
	t1, err := f.tracker.Create(f.as(f.human), "fix the crash", "", database.PriorityP0, &component.Id, &f.agent.Id)
	assertion.MustNoError(t, err)
	t2, err := f.tracker.Create(f.as(f.human), "add the feature", "100% done", database.PriorityP2, nil, nil)
	assertion.MustNoError(t, err)
	t3, err := f.tracker.Create(f.as(f.human), "refactor", "", database.PriorityNone, nil, &f.human.Id)
	assertion.MustNoError(t, err)
	assertion.MustNoError(t, t3.Update(f.as(f.human), nil, nil, new(database.TicketStatusDone), nil, nil, nil))
	_, err = t3.AddComment(f.as(f.human), "the crash is also fixed")
	assertion.MustNoError(t, err)
	deleted, err := t2.AddComment(f.as(f.human), "crash")
	assertion.MustNoError(t, err)
	assertion.MustNoError(t, deleted.Delete(f.as(f.human)))
	for i, ticket := range []*Ticket{t1, t2, t3} {
		ticket.LastActivityAt = time.Now().Add(time.Duration(i-3) * time.Hour).Truncate(time.Second)
		ticket.CreatedAt = time.Now().Add(time.Duration(i-10) * time.Hour).Truncate(time.Second)
		assertion.MustNoError(t, f.dao.Ticket.Update(context.Background(), ticket.Ticket))
	}

	cases := []struct {
		filter string
		want   []int32
	}{
		{filter: "", want: []int32{t3.Id, t2.Id, t1.Id}},
		{filter: "crash", want: []int32{t3.Id, t1.Id}},
		{filter: `"%"`, want: []int32{t2.Id}},
		{filter: "status = OPEN OR status = IN_PROGRESS", want: []int32{t2.Id, t1.Id}},
		{filter: "status != DONE", want: []int32{t2.Id, t1.Id}},
		{filter: "priority = P0", want: []int32{t1.Id}},
		{filter: "priority <= P2", want: []int32{t2.Id, t1.Id}},
		{filter: "NOT priority:*", want: []int32{t3.Id}},
		{filter: "component = build", want: []int32{t1.Id}},
		{filter: "component != build", want: []int32{t3.Id, t2.Id}},
		{filter: "component = unknown", want: nil},
		{filter: "assignee = human OR assignee = agent", want: []int32{t3.Id, t1.Id}},
		{filter: "-assignee:*", want: []int32{t2.Id}},
		{filter: "reporter = human", want: []int32{t3.Id, t2.Id, t1.Id}},
		{filter: "status = OPEN crash", want: []int32{t1.Id}},
		{filter: `"the crash"`, want: []int32{t3.Id, t1.Id}},
		{filter: "title:feature", want: []int32{t2.Id}},
		{filter: "nothing", want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.filter, func(t *testing.T) {
			tickets, err := f.tracker.Find(f.as(f.human), tc.filter, "")
			assertion.MustNoError(t, err)
			var got []int32
			for _, v := range tickets {
				got = append(got, v.Id)
			}
			assertion.Equal(t, got, tc.want)
		})
	}

	_, err = f.tracker.Find(f.as(f.human), "status = CLOSED", "")
	mustErrorIs(t, err, ErrInvalidArgument)

	orders := []struct {
		orderBy string
		want    []int32
	}{
		{orderBy: "number", want: []int32{t1.Id, t2.Id, t3.Id}},
		{orderBy: "number desc", want: []int32{t3.Id, t2.Id, t1.Id}},
		{orderBy: "status, number", want: []int32{t1.Id, t2.Id, t3.Id}},
		{orderBy: "priority", want: []int32{t1.Id, t2.Id, t3.Id}},
		{orderBy: "priority desc", want: []int32{t2.Id, t1.Id, t3.Id}},
		{orderBy: "component", want: []int32{t1.Id, t3.Id, t2.Id}},
		{orderBy: "assignee", want: []int32{t1.Id, t3.Id, t2.Id}},
		{orderBy: "assignee desc", want: []int32{t3.Id, t1.Id, t2.Id}},
		{orderBy: "created_at", want: []int32{t1.Id, t2.Id, t3.Id}},
		{orderBy: "created_at desc", want: []int32{t3.Id, t2.Id, t1.Id}},
		{orderBy: "last_activity_at", want: []int32{t1.Id, t2.Id, t3.Id}},
	}
	for _, tc := range orders {
		t.Run(tc.orderBy, func(t *testing.T) {
			tickets, err := f.tracker.Find(f.as(f.human), "", tc.orderBy)
			assertion.MustNoError(t, err)
			var got []int32
			for _, v := range tickets {
				got = append(got, v.Id)
			}
			assertion.Equal(t, got, tc.want)
		})
	}
	for _, orderBy := range []string{"title", "number up", "number desc desc"} {
		_, err = f.tracker.Find(f.as(f.human), "", orderBy)
		mustErrorIs(t, err, ErrInvalidArgument)
	}
}

func TestSaveComponent(t *testing.T) {
	f := newFixture(t)
	c := f.component(t, "build")
	renamed, err := f.tracker.SaveComponent(f.as(f.human), c.Id, "ci")
	assertion.MustNoError(t, err)
	assertion.Equal(t, renamed.Name, "ci")
	components, err := f.tracker.Components(f.as(f.agent))
	assertion.MustNoError(t, err)
	assertion.MustLen(t, components, 1)
	assertion.Equal(t, components[0].Name, "ci")

	_, err = f.tracker.SaveComponent(f.as(f.agent), 0, "agent")
	mustErrorIs(t, err, ErrPermissionDenied)
	_, err = f.tracker.SaveComponent(f.as(f.human), 100, "unknown")
	mustErrorIs(t, err, ErrNotFound)
	_, err = f.tracker.SaveComponent(f.as(f.human), 0, strings.Repeat("a", 256))
	mustErrorIs(t, err, ErrInvalidArgument)
}

func TestWithHuman(t *testing.T) {
	f := newFixture(t)
	ctx := WithHuman(context.Background(), "alice@example.com")

	_, err := f.tracker.Find(ctx, "", "")
	assertion.MustNoError(t, err)
	_, err = f.dao.Actor.SelectBySubject(context.Background(), "alice@example.com")
	mustErrorIs(t, err, sql.ErrNoRows)

	ticket, err := f.tracker.Create(ctx, "title", "body", database.PriorityNone, nil, nil)
	assertion.MustNoError(t, err)
	alice, err := f.tracker.Me(ctx)
	assertion.MustNoError(t, err)
	assertion.Equal(t, alice.Kind, database.ActorKindHuman)
	assertion.Equal(t, alice.Name, "alice@example.com")
	assertion.Equal(t, *alice.Subject, "alice@example.com")
	assertion.Equal(t, ticket.ReporterId, alice.Id)

	again, err := f.tracker.Me(ctx)
	assertion.MustNoError(t, err)
	assertion.Equal(t, again.Id, alice.Id)

	existing, err := f.tracker.Me(WithHuman(context.Background(), "human@example.com"))
	assertion.MustNoError(t, err)
	assertion.Equal(t, existing.Id, f.human.Id)

	_, err = f.tracker.Find(WithHuman(context.Background(), ""), "", "")
	mustErrorIs(t, err, ErrUnauthenticated)
}
