package webhook

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/go-github/v85/github"
	"go.f110.dev/githubmock"
	"go.f110.dev/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.f110.dev/mono/go/database/dbtestutil"
	"go.f110.dev/mono/go/git"
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

// Name returns the last element of the URL as the name of the repository.
func (r fakeRepositories) Name(_ context.Context, id int32) (string, bool, error) {
	for url, v := range r {
		if v == id {
			return path.Base(url), true, nil
		}
	}
	return "", false, nil
}

const repositoryURL = "https://github.com/f110/mono"

func createTicket(t *testing.T, tracker *issue.Tracker, daos dao.Options, title string) *issue.Ticket {
	t.Helper()
	human, err := daos.Actor.Create(context.Background(), &database.Actor{Kind: database.ActorKindHuman, Name: title, Subject: new(title)})
	assertion.MustNoError(t, err)
	ticket, err := tracker.Create(issue.WithActor(context.Background(), human), title, "", database.TicketTypeNone, database.PriorityNone, nil, nil)
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

func pullRequestEvent(action string, number int, title, head string, merged bool) *github.PullRequestEvent {
	return &github.PullRequestEvent{
		Action: new(action),
		Repo:   &github.Repository{HTMLURL: new(repositoryURL), Name: new("mono"), Owner: &github.User{Login: new("f110")}},
		PullRequest: &github.PullRequest{
			Number: new(number),
			Title:  new(title),
			Merged: new(merged),
			Head:   &github.PullRequestBranch{SHA: new(head)},
			User:   &github.User{Login: new("alice")},
		},
	}
}

func TestPullRequestReconciler(t *testing.T) {
	setup := func(t *testing.T) (*issue.Tracker, dao.Options, *githubmock.Repository, *github.Client) {
		tracker, daos := newTracker(t)
		mock := githubmock.NewMock()
		repo := mock.Repository("f110/mono")
		return tracker, daos, repo, github.NewClient(&http.Client{Transport: mock.Transport()})
	}
	reconcile := func(t *testing.T, r *PullRequestReconciler, ev *github.PullRequestEvent) *ghwebhook.Event {
		t.Helper()
		e := &ghwebhook.Event{State: ghwebhook.StateProcessing}
		assertion.MustNoError(t, r.Reconcile(context.Background(), e, ev))
		return e
	}
	linksOf := func(t *testing.T, ticket *issue.Ticket) []*database.PullRequestLink {
		t.Helper()
		links, err := ticket.PullRequestLinks(context.Background())
		assertion.MustNoError(t, err)
		return links
	}

	t.Run("link the pull request and follow the state", func(t *testing.T) {
		tracker, daos, repo, gh := setup(t)
		refs := createTicket(t, tracker, daos, "refs")
		fixes := createTicket(t, tracker, daos, "fixes")
		root := githubmock.NewCommit().Message(fmt.Sprintf("subject 1\n\nRefs: %d", refs.Id))
		child := githubmock.NewCommit().Message(fmt.Sprintf("subject 2\n\nFixes: #%d\nRefs: 1000", fixes.Id)).Parents(root)
		assertion.MustNoError(t, repo.Commits(root, child))
		pr := githubmock.NewPullRequest().Number(5).Commits(root, child)
		repo.PullRequests(pr)
		r := NewPullRequestReconciler(tracker, fakeRepositories{repositoryURL: 10}, gh)

		for range 2 {
			e := reconcile(t, r, pullRequestEvent("opened", 5, "title", child.GetSHA(), false))
			assertion.Equal(t, e.State, ghwebhook.StateProcessing)
		}
		system, err := tracker.System(context.Background())
		assertion.MustNoError(t, err)
		for _, ticket := range []*issue.Ticket{refs, fixes} {
			links := linksOf(t, ticket)
			assertion.MustLen(t, links, 1)
			assertion.Equal(t, links[0].RepositoryId, int32(10))
			assertion.Equal(t, links[0].Number, int32(5))
			assertion.Equal(t, links[0].Title, "title")
			assertion.Equal(t, links[0].Author, "alice")
			assertion.Equal(t, links[0].State, database.PullRequestStateOpen)
			assertion.Equal(t, links[0].HeadSha, child.GetSHA())
			activities, err := ticket.Activities(context.Background())
			assertion.MustNoError(t, err)
			assertion.MustLen(t, activities, 2)
			assertion.Equal(t, activities[1].Kind, database.ActivityKindPullRequestLinked)
			assertion.Equal(t, activities[1].ActorId, system.Id)
		}
		got, err := tracker.Ticket(issue.WithActor(context.Background(), system), fixes.Id)
		assertion.MustNoError(t, err)
		assertion.Equal(t, got.Status, database.TicketStatusOpen)

		reconcile(t, r, pullRequestEvent("synchronize", 5, "new title", "3333333333333333333333333333333333333333", false))
		for _, ticket := range []*issue.Ticket{refs, fixes} {
			links := linksOf(t, ticket)
			assertion.MustLen(t, links, 1)
			assertion.Equal(t, links[0].Title, "new title")
			assertion.Equal(t, links[0].HeadSha, "3333333333333333333333333333333333333333")
		}

		reconcile(t, r, pullRequestEvent("closed", 5, "new title", "3333333333333333333333333333333333333333", true))
		for _, ticket := range []*issue.Ticket{refs, fixes} {
			assertion.Equal(t, linksOf(t, ticket)[0].State, database.PullRequestStateMerged)
		}
	})

	t.Run("closed without merging", func(t *testing.T) {
		tracker, daos, repo, gh := setup(t)
		ticket := createTicket(t, tracker, daos, "ticket")
		c := githubmock.NewCommit().Message(fmt.Sprintf("subject\n\nRefs: %d", ticket.Id))
		assertion.MustNoError(t, repo.Commits(c))
		repo.PullRequests(githubmock.NewPullRequest().Number(1).Commits(c))
		r := NewPullRequestReconciler(tracker, fakeRepositories{repositoryURL: 10}, gh)

		reconcile(t, r, pullRequestEvent("opened", 1, "title", c.GetSHA(), false))
		reconcile(t, r, pullRequestEvent("closed", 1, "title", c.GetSHA(), false))
		assertion.Equal(t, linksOf(t, ticket)[0].State, database.PullRequestStateClosed)
	})

	t.Run("skipped", func(t *testing.T) {
		tracker, daos, repo, gh := setup(t)
		ticket := createTicket(t, tracker, daos, "ticket")
		c := githubmock.NewCommit().Message(fmt.Sprintf("subject\n\nRefs: %d", ticket.Id))
		assertion.MustNoError(t, repo.Commits(c))
		repo.PullRequests(githubmock.NewPullRequest().Number(1).Commits(c))

		e := reconcile(t, NewPullRequestReconciler(tracker, fakeRepositories{repositoryURL: 10}, gh), pullRequestEvent("labeled", 1, "title", c.GetSHA(), false))
		assertion.Equal(t, e.State, ghwebhook.StateSkipped)
		e = reconcile(t, NewPullRequestReconciler(tracker, fakeRepositories{}, gh), pullRequestEvent("opened", 1, "title", c.GetSHA(), false))
		assertion.Equal(t, e.State, ghwebhook.StateSkipped)
		assertion.Len(t, linksOf(t, ticket), 0)
	})
}

type fakeGitData struct {
	git.GitDataClient
	changeIDs map[string]string
}

func (g *fakeGitData) GetCommit(_ context.Context, req *git.RequestGetCommit, _ ...grpc.CallOption) (*git.ResponseGetCommit, error) {
	changeID, ok := g.changeIDs[req.Repo+"/"+req.Sha]
	if !ok {
		return nil, status.Error(codes.NotFound, "commit not found")
	}
	return &git.ResponseGetCommit{Commit: &git.Commit{Sha: req.Sha, ChangeId: changeID}}, nil
}

func TestChangeIDFiller(t *testing.T) {
	tracker, daos := newTracker(t)
	ticket := createTicket(t, tracker, daos, "ticket")
	ctx, err := asSystem(context.Background(), tracker)
	assertion.MustNoError(t, err)
	committedAt := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for _, v := range []struct {
		repositoryID int32
		sha          string
	}{{10, "jj"}, {10, "git"}, {10, "not-fetched"}, {20, "unregistered"}} {
		assertion.MustNoError(t, ticket.LinkCommit(ctx, v.repositoryID, v.sha, "subject", "alice", database.TrailerKindRefs, committedAt))
	}
	gitData := &fakeGitData{changeIDs: map[string]string{"mono/jj": "zzzzzzzz", "mono/git": ""}}
	f := NewChangeIDFiller(tracker, fakeRepositories{repositoryURL: 10}, gitData)

	assertion.MustNoError(t, f.fill(context.Background()))
	links, err := ticket.CommitLinks(context.Background())
	assertion.MustNoError(t, err)
	got := make(map[string]*string)
	for _, v := range links {
		got[v.Sha] = v.ChangeId
	}
	assertion.Equal(t, *got["jj"], "zzzzzzzz")
	assertion.Equal(t, *got["git"], "")
	assertion.True(t, got["not-fetched"] == nil)
	assertion.True(t, got["unregistered"] == nil)

	gitData.changeIDs["mono/not-fetched"] = "yyyyyyyy"
	assertion.MustNoError(t, f.fill(context.Background()))
	links, err = ticket.CommitLinks(context.Background())
	assertion.MustNoError(t, err)
	for _, v := range links {
		if v.Sha == "not-fetched" {
			assertion.Equal(t, *v.ChangeId, "yyyyyyyy")
		}
	}
}
