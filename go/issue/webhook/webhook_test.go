package webhook

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/database/dbtestutil"
	ghwebhook "go.f110.dev/mono/go/github/webhook"
	"go.f110.dev/mono/go/issue"
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

func newTracker(t *testing.T) (*issue.Tracker, dao.Options) {
	t.Helper()
	if mysqlPort == 0 {
		t.Skip("mysqld is not found")
	}
	dbName := fmt.Sprintf("issue_webhook_test_%d", databaseNo.Add(1))
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
	return issue.NewTracker(daos), daos
}

type fakeRepositories map[string]int32

func (r fakeRepositories) Find(_ context.Context, url string) (int32, bool, error) {
	if url == "error" {
		return 0, false, xerrors.New("build is not available")
	}
	id, ok := r[url]
	return id, ok, nil
}

const repositoryURL = "https://github.com/f110/mono"

func createTicket(t *testing.T, tracker *issue.Tracker, daos dao.Options, title string) *issue.Ticket {
	t.Helper()
	human, err := daos.Actor.Create(context.Background(), &database.Actor{Kind: database.ActorKindHuman, Name: title, Subject: new(title)})
	assertion.MustNoError(t, err)
	ticket, err := tracker.Create(issue.WithActor(context.Background(), human), title, "", database.PriorityNone, nil, nil)
	assertion.MustNoError(t, err)
	return ticket
}

func pushEvent(url, ref string, commits ...*github.HeadCommit) *github.PushEvent {
	return &github.PushEvent{
		Ref:     new(ref),
		Repo:    &github.PushEventRepository{HTMLURL: new(url), DefaultBranch: new("main")},
		Commits: commits,
	}
}

func TestPushReconciler(t *testing.T) {
	committedAt := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	t.Run("link the commits to the tickets and fix the tickets", func(t *testing.T) {
		tracker, daos := newTracker(t)
		refs := createTicket(t, tracker, daos, "refs")
		fixes := createTicket(t, tracker, daos, "fixes")
		r := NewPushReconciler(tracker, fakeRepositories{repositoryURL: 10})
		ev := pushEvent(repositoryURL, "refs/heads/main",
			&github.HeadCommit{
				ID:        new("1111111111111111111111111111111111111111"),
				Message:   new(fmt.Sprintf("subject 1\n\nbody\n\nRefs: #%d\nFixes: %d\nFixes: 1000", refs.Id, fixes.Id)),
				Timestamp: &github.Timestamp{Time: committedAt},
				Author:    &github.CommitAuthor{Name: new("Alice"), Login: new("alice")},
			},
			&github.HeadCommit{
				ID:        new("2222222222222222222222222222222222222222"),
				Message:   new("subject 2"),
				Timestamp: &github.Timestamp{Time: committedAt},
				Author:    &github.CommitAuthor{Name: new("Bob")},
			},
		)

		for range 2 {
			e := &ghwebhook.Event{State: ghwebhook.StateProcessing}
			assertion.MustNoError(t, r.Reconcile(context.Background(), e, ev))
			assertion.Equal(t, e.State, ghwebhook.StateProcessing)
		}

		system, err := tracker.System(context.Background())
		assertion.MustNoError(t, err)
		for _, tc := range []struct {
			ticket  *issue.Ticket
			trailer database.TrailerKind
			status  database.TicketStatus
		}{
			{ticket: refs, trailer: database.TrailerKindRefs, status: database.TicketStatusOpen},
			{ticket: fixes, trailer: database.TrailerKindFixes, status: database.TicketStatusDone},
		} {
			ticket, err := tracker.Ticket(issue.WithActor(context.Background(), system), tc.ticket.Id)
			assertion.MustNoError(t, err)
			assertion.Equal(t, ticket.Status, tc.status)
			links, err := ticket.CommitLinks(context.Background())
			assertion.MustNoError(t, err)
			assertion.MustLen(t, links, 1)
			assertion.Equal(t, links[0].RepositoryId, int32(10))
			assertion.Equal(t, links[0].Sha, "1111111111111111111111111111111111111111")
			assertion.Equal(t, links[0].Subject, "subject 1")
			assertion.Equal(t, links[0].Author, "alice")
			assertion.Equal(t, links[0].Trailer, tc.trailer)
			activities, err := ticket.Activities(context.Background())
			assertion.MustNoError(t, err)
			assertion.Equal(t, activities[1].Kind, database.ActivityKindCommitLinked)
			assertion.Equal(t, activities[1].ActorId, system.Id)
		}
	})

	cases := []struct {
		name string
		url  string
		ref  string
	}{
		{name: "the push to the other branch is skipped", url: repositoryURL, ref: "refs/heads/feature"},
		{name: "the repository which is not registered in Build is skipped", url: "https://github.com/f110/other", ref: "refs/heads/main"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tracker, daos := newTracker(t)
			ticket := createTicket(t, tracker, daos, "ticket")
			r := NewPushReconciler(tracker, fakeRepositories{repositoryURL: 10})
			e := &ghwebhook.Event{State: ghwebhook.StateProcessing}
			ev := pushEvent(tc.url, tc.ref, &github.HeadCommit{
				ID:        new("1111111111111111111111111111111111111111"),
				Message:   new(fmt.Sprintf("subject\n\nFixes: %d", ticket.Id)),
				Timestamp: &github.Timestamp{Time: committedAt},
			})
			assertion.MustNoError(t, r.Reconcile(context.Background(), e, ev))
			assertion.Equal(t, e.State, ghwebhook.StateSkipped)
			links, err := ticket.CommitLinks(context.Background())
			assertion.MustNoError(t, err)
			assertion.Len(t, links, 0)
		})
	}

	t.Run("the error of Build is returned to retry", func(t *testing.T) {
		tracker, _ := newTracker(t)
		r := NewPushReconciler(tracker, fakeRepositories{})
		e := &ghwebhook.Event{State: ghwebhook.StateProcessing}
		assertion.MustError(t, r.Reconcile(context.Background(), e, pushEvent("error", "refs/heads/main")))
	})
}
