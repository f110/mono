package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/database/dbtestutil"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/http/authproxy"
	"go.f110.dev/mono/go/issue"
	"go.f110.dev/mono/go/issue/api"
	"go.f110.dev/mono/go/issue/database"
	"go.f110.dev/mono/go/issue/database/dao"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/testing/assertion"
)

var (
	mysqlPort  int
	databaseNo atomic.Int32
)

func TestMain(m *testing.M) {
	logger.Init()
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

func newConn(t *testing.T) *sql.DB {
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
	return conn
}

func TestEnumConversion(t *testing.T) {
	statuses := map[database.TicketStatus]api.TicketStatus{
		database.TicketStatusOpen:              api.TicketStatus_TICKET_STATUS_OPEN,
		database.TicketStatusInProgress:        api.TicketStatus_TICKET_STATUS_IN_PROGRESS,
		database.TicketStatusDone:              api.TicketStatus_TICKET_STATUS_DONE,
		database.TicketStatusWontFix:           api.TicketStatus_TICKET_STATUS_WONT_FIX,
		database.TicketStatusNeedInvestigation: api.TicketStatus_TICKET_STATUS_NEED_INVESTIGATION,
	}
	for db, a := range statuses {
		assertion.Equal(t, toTicket(&issue.Ticket{Ticket: &database.Ticket{Status: db}}).GetStatus(), a)
		got, err := toDBStatus(a)
		assertion.MustNoError(t, err)
		assertion.Equal(t, got, db)
	}

	priorities := map[database.Priority]api.Priority{
		database.PriorityNone: api.Priority_PRIORITY_UNSPECIFIED,
		database.PriorityP0:   api.Priority_PRIORITY_P0,
		database.PriorityP4:   api.Priority_PRIORITY_P4,
	}
	for db, a := range priorities {
		assertion.Equal(t, toTicket(&issue.Ticket{Ticket: &database.Ticket{Priority: db}}).GetPriority(), a)
	}

	types := map[database.TicketType]api.TicketType{
		database.TicketTypeNone:      api.TicketType_TICKET_TYPE_UNSPECIFIED,
		database.TicketTypeBug:       api.TicketType_TICKET_TYPE_BUG,
		database.TicketTypeOperation: api.TicketType_TICKET_TYPE_OPERATION,
	}
	for db, a := range types {
		assertion.Equal(t, toTicket(&issue.Ticket{Ticket: &database.Ticket{Type: db}}).GetType(), a)
	}

	relations := map[database.RelationType]api.RelationType{
		database.RelationTypeParentOf:    api.RelationType_RELATION_TYPE_PARENT_OF,
		database.RelationTypeBlocks:      api.RelationType_RELATION_TYPE_BLOCKS,
		database.RelationTypeDuplicateOf: api.RelationType_RELATION_TYPE_DUPLICATE_OF,
		database.RelationTypeRelatesTo:   api.RelationType_RELATION_TYPE_RELATES_TO,
	}
	for db, a := range relations {
		assertion.Equal(t, toRelation(&database.Relation{RelationType: db}).GetType(), a)
		got, err := toDBRelationType(a)
		assertion.MustNoError(t, err)
		assertion.Equal(t, got, db)
	}

	kinds := map[database.ActorKind]api.ActorKind{
		database.ActorKindHuman:  api.ActorKind_ACTOR_KIND_HUMAN,
		database.ActorKindAgent:  api.ActorKind_ACTOR_KIND_AGENT,
		database.ActorKindSystem: api.ActorKind_ACTOR_KIND_SYSTEM,
	}
	for db, a := range kinds {
		assertion.Equal(t, toActor(&database.Actor{Kind: db}).GetKind(), a)
	}

	trailers := map[database.TrailerKind]api.TrailerKind{
		database.TrailerKindRefs:  api.TrailerKind_TRAILER_KIND_REFS,
		database.TrailerKindFixes: api.TrailerKind_TRAILER_KIND_FIXES,
	}
	for db, a := range trailers {
		assertion.Equal(t, toCommitLink(&database.CommitLink{Trailer: db}, "").GetTrailer(), a)
	}

	states := map[database.PullRequestState]api.PullRequestState{
		database.PullRequestStateOpen:   api.PullRequestState_PULL_REQUEST_STATE_OPEN,
		database.PullRequestStateClosed: api.PullRequestState_PULL_REQUEST_STATE_CLOSED,
		database.PullRequestStateMerged: api.PullRequestState_PULL_REQUEST_STATE_MERGED,
	}
	for db, a := range states {
		assertion.Equal(t, toPullRequestLink(&database.PullRequestLink{State: db}, "").GetState(), a)
	}

	assertion.Equal(t, toActivity(&database.Activity{Kind: database.ActivityKindCreate}).GetKind(), api.ActivityKind_ACTIVITY_KIND_CREATE)
	assertion.Equal(t, toActivity(&database.Activity{Kind: database.ActivityKindCommentDeleted}).GetKind(), api.ActivityKind_ACTIVITY_KIND_COMMENT_DELETED)
	assertion.Equal(t, toActivity(&database.Activity{Kind: database.ActivityKindMentionRemoved}).GetKind(), api.ActivityKind_ACTIVITY_KIND_MENTION_REMOVED)

	_, err := toDBStatus(api.TicketStatus_TICKET_STATUS_UNSPECIFIED)
	assertion.Equal(t, connect.CodeOf(err), connect.CodeInvalidArgument)
	_, err = toDBRelationType(api.RelationType_RELATION_TYPE_UNSPECIFIED)
	assertion.Equal(t, connect.CodeOf(err), connect.CodeInvalidArgument)
}

func TestToConnectError(t *testing.T) {
	cases := []struct {
		err  error
		want connect.Code
	}{
		{err: issue.ErrUnauthenticated.WithStack(), want: connect.CodeUnauthenticated},
		{err: issue.ErrPermissionDenied.WithStack(), want: connect.CodePermissionDenied},
		{err: issue.ErrNotFound.WithStack(), want: connect.CodeNotFound},
		{err: xerrors.WithMessage(issue.ErrInvalidArgument, "title is required"), want: connect.CodeInvalidArgument},
		{err: xerrors.WithStack(context.Canceled), want: connect.CodeCanceled},
		{err: xerrors.WithStack(context.DeadlineExceeded), want: connect.CodeDeadlineExceeded},
		{err: xerrors.New("unexpected"), want: connect.CodeInternal},
	}
	for _, tc := range cases {
		assertion.Equal(t, connect.CodeOf(toConnectError(tc.err)), tc.want)
	}
}

func newClient(t *testing.T) (api.IssueClient, *database.Actor) {
	t.Helper()
	daos := dao.NewOptions(newConn(t))
	actor, err := daos.Actor.Create(context.Background(), &database.Actor{Kind: database.ActorKindHuman, Name: "human", Subject: new("human@example.com")})
	assertion.MustNoError(t, err)

	path, handler := api.NewIssueHandler(NewIssueHandler(issue.NewTracker(daos), nil))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		handler.ServeHTTP(w, req.WithContext(issue.WithActor(req.Context(), actor)))
	}))
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return api.NewIssueClient(http.DefaultClient, s.URL), actor
}

func TestIssueHandler(t *testing.T) {
	client, actor := newClient(t)
	ctx := context.Background()

	whoami, err := client.WhoAmI(ctx, connect.NewRequest(&api.RequestWhoAmI{}))
	assertion.MustNoError(t, err)
	assertion.Equal(t, whoami.Msg.GetActor().GetId(), actor.Id)

	component, err := client.SaveComponent(ctx, connect.NewRequest(api.RequestSaveComponent_builder{Name: new("build")}.Build()))
	assertion.MustNoError(t, err)
	created, err := client.CreateTicket(ctx, connect.NewRequest(api.RequestCreateTicket_builder{
		Title:       new("title"),
		Body:        new("body"),
		Priority:    new(api.Priority_PRIORITY_P1),
		Type:        new(api.TicketType_TICKET_TYPE_BUG),
		ComponentId: new(component.Msg.GetComponent().GetId()),
	}.Build()))
	assertion.MustNoError(t, err)
	ticket := created.Msg.GetTicket()
	assertion.Equal(t, ticket.GetStatus(), api.TicketStatus_TICKET_STATUS_OPEN)
	assertion.Equal(t, ticket.GetType(), api.TicketType_TICKET_TYPE_BUG)
	assertion.True(t, ticket.HasComponentId())
	assertion.False(t, ticket.HasAssigneeId())
	assertion.Equal(t, ticket.GetReporterId(), actor.Id)

	t.Run("update only the specified fields", func(t *testing.T) {
		res, err := client.UpdateTicket(ctx, connect.NewRequest(api.RequestUpdateTicket_builder{
			Number:      new(ticket.GetNumber()),
			Status:      new(api.TicketStatus_TICKET_STATUS_IN_PROGRESS),
			Priority:    new(api.Priority_PRIORITY_UNSPECIFIED),
			Type:        new(api.TicketType_TICKET_TYPE_UNSPECIFIED),
			ComponentId: new(int32(0)),
		}.Build()))
		assertion.MustNoError(t, err)
		updated := res.Msg.GetTicket()
		assertion.Equal(t, updated.GetTitle(), "title")
		assertion.Equal(t, updated.GetBody(), "body")
		assertion.Equal(t, updated.GetStatus(), api.TicketStatus_TICKET_STATUS_IN_PROGRESS)
		assertion.Equal(t, updated.GetPriority(), api.Priority_PRIORITY_UNSPECIFIED)
		assertion.Equal(t, updated.GetType(), api.TicketType_TICKET_TYPE_UNSPECIFIED)
		assertion.False(t, updated.HasComponentId())
	})

	t.Run("get the ticket with the relations and the comments", func(t *testing.T) {
		other, err := client.CreateTicket(ctx, connect.NewRequest(api.RequestCreateTicket_builder{Title: new("other")}.Build()))
		assertion.MustNoError(t, err)
		_, err = client.AddRelation(ctx, connect.NewRequest(api.RequestAddRelation_builder{Relation: api.Relation_builder{
			From: new(other.Msg.GetTicket().GetNumber()),
			Type: new(api.RelationType_RELATION_TYPE_BLOCKS),
			To:   new(ticket.GetNumber()),
		}.Build()}.Build()))
		assertion.MustNoError(t, err)
		_, err = client.AddComment(ctx, connect.NewRequest(api.RequestAddComment_builder{
			Ticket: new(other.Msg.GetTicket().GetNumber()),
			Body:   new(fmt.Sprintf("#%d", ticket.GetNumber())),
		}.Build()))
		assertion.MustNoError(t, err)
		_, err = client.AddComment(ctx, connect.NewRequest(api.RequestAddComment_builder{Ticket: new(ticket.GetNumber()), Body: new("comment")}.Build()))
		assertion.MustNoError(t, err)

		res, err := client.GetTicket(ctx, connect.NewRequest(api.RequestGetTicket_builder{Number: new(ticket.GetNumber())}.Build()))
		assertion.MustNoError(t, err)
		assertion.MustLen(t, res.Msg.GetRelations(), 1)
		assertion.Equal(t, res.Msg.GetRelations()[0].GetRelation().GetFrom(), other.Msg.GetTicket().GetNumber())
		assertion.Equal(t, res.Msg.GetRelations()[0].GetTitle(), "other")
		assertion.MustLen(t, res.Msg.GetComments(), 1)
		assertion.Equal(t, res.Msg.GetComments()[0].GetBody(), "comment")
		assertion.MustLen(t, res.Msg.GetBacklinks(), 1)
		assertion.Equal(t, res.Msg.GetBacklinks()[0].GetTicket(), other.Msg.GetTicket().GetNumber())
		assertion.Equal(t, res.Msg.GetBacklinks()[0].GetTitle(), "other")
		assertion.True(t, res.Msg.GetBacklinks()[0].HasCreatedAt())

		activities, err := client.ListActivities(ctx, connect.NewRequest(api.RequestListActivities_builder{Ticket: new(ticket.GetNumber())}.Build()))
		assertion.MustNoError(t, err)
		last := activities.Msg.GetActivities()[len(activities.Msg.GetActivities())-1]
		assertion.Equal(t, last.GetKind(), api.ActivityKind_ACTIVITY_KIND_COMMENT)
		assertion.Equal(t, last.GetActorId(), actor.Id)
	})

	t.Run("list the tickets by the filter", func(t *testing.T) {
		res, err := client.ListTickets(ctx, connect.NewRequest(api.RequestListTickets_builder{Filter: new("status = IN_PROGRESS title")}.Build()))
		assertion.MustNoError(t, err)
		assertion.MustLen(t, res.Msg.GetTickets(), 1)
		assertion.Equal(t, res.Msg.GetTickets()[0].GetNumber(), ticket.GetNumber())
	})

	t.Run("list the tickets by the pages", func(t *testing.T) {
		all, err := client.ListTickets(ctx, connect.NewRequest(api.RequestListTickets_builder{OrderBy: new("number")}.Build()))
		assertion.MustNoError(t, err)
		assertion.Equal(t, all.Msg.GetNextPageToken(), "")
		assertion.True(t, len(all.Msg.GetTickets()) > 1)

		var got []int32
		token := ""
		for {
			res, err := client.ListTickets(ctx, connect.NewRequest(api.RequestListTickets_builder{OrderBy: new("number"), PageSize: new(int32(1)), PageToken: new(token)}.Build()))
			assertion.MustNoError(t, err)
			assertion.MustLen(t, res.Msg.GetTickets(), 1)
			got = append(got, res.Msg.GetTickets()[0].GetNumber())
			token = res.Msg.GetNextPageToken()
			if token == "" {
				break
			}
		}
		assertion.Equal(t, got, enumerable.Map(all.Msg.GetTickets(), (*api.Ticket).GetNumber))

		first, err := client.ListTickets(ctx, connect.NewRequest(api.RequestListTickets_builder{OrderBy: new("number"), PageSize: new(int32(1))}.Build()))
		assertion.MustNoError(t, err)
		_, err = client.ListTickets(ctx, connect.NewRequest(api.RequestListTickets_builder{PageSize: new(int32(1)), PageToken: new(first.Msg.GetNextPageToken())}.Build()))
		assertion.Equal(t, connect.CodeOf(err), connect.CodeInvalidArgument)
		_, err = client.ListTickets(ctx, connect.NewRequest(api.RequestListTickets_builder{PageToken: new("invalid")}.Build()))
		assertion.Equal(t, connect.CodeOf(err), connect.CodeInvalidArgument)
		_, err = client.ListTickets(ctx, connect.NewRequest(api.RequestListTickets_builder{PageSize: new(int32(-1))}.Build()))
		assertion.Equal(t, connect.CodeOf(err), connect.CodeInvalidArgument)
	})

	t.Run("list the revisions of the body and the comment", func(t *testing.T) {
		created, err := client.CreateTicket(ctx, connect.NewRequest(api.RequestCreateTicket_builder{Title: new("revisions"), Body: new("first")}.Build()))
		assertion.MustNoError(t, err)
		number := created.Msg.GetTicket().GetNumber()
		_, err = client.UpdateTicket(ctx, connect.NewRequest(api.RequestUpdateTicket_builder{Number: new(number), Body: new("second")}.Build()))
		assertion.MustNoError(t, err)
		res, err := client.ListTicketRevisions(ctx, connect.NewRequest(api.RequestListTicketRevisions_builder{Ticket: new(number)}.Build()))
		assertion.MustNoError(t, err)
		assertion.MustLen(t, res.Msg.GetRevisions(), 1)
		assertion.Equal(t, res.Msg.GetRevisions()[0].GetBody(), "first")
		assertion.Equal(t, res.Msg.GetRevisions()[0].GetActorId(), actor.Id)

		comment, err := client.AddComment(ctx, connect.NewRequest(api.RequestAddComment_builder{Ticket: new(number), Body: new("comment")}.Build()))
		assertion.MustNoError(t, err)
		id := comment.Msg.GetComment().GetId()
		_, err = client.EditComment(ctx, connect.NewRequest(api.RequestEditComment_builder{Id: new(id), Body: new("edited")}.Build()))
		assertion.MustNoError(t, err)
		commentRevs, err := client.ListCommentRevisions(ctx, connect.NewRequest(api.RequestListCommentRevisions_builder{Id: new(id)}.Build()))
		assertion.MustNoError(t, err)
		assertion.MustLen(t, commentRevs.Msg.GetRevisions(), 1)
		assertion.Equal(t, commentRevs.Msg.GetRevisions()[0].GetBody(), "comment")
		assertion.Equal(t, commentRevs.Msg.GetRevisions()[0].GetActorId(), actor.Id)

		_, err = client.ListTicketRevisions(ctx, connect.NewRequest(api.RequestListTicketRevisions_builder{Ticket: new(int32(100))}.Build()))
		assertion.Equal(t, connect.CodeOf(err), connect.CodeNotFound)
		_, err = client.ListCommentRevisions(ctx, connect.NewRequest(api.RequestListCommentRevisions_builder{Id: new(int32(100))}.Build()))
		assertion.Equal(t, connect.CodeOf(err), connect.CodeNotFound)
	})

	t.Run("manage the templates", func(t *testing.T) {
		created, err := client.SaveTemplate(ctx, connect.NewRequest(api.RequestSaveTemplate_builder{
			Title:       new("bug"),
			Body:        new("## Steps"),
			ComponentId: new(component.Msg.GetComponent().GetId()),
		}.Build()))
		assertion.MustNoError(t, err)
		tmpl := created.Msg.GetTemplate()
		assertion.Equal(t, tmpl.GetTitle(), "bug")
		assertion.Equal(t, tmpl.GetBody(), "## Steps")
		assertion.Equal(t, tmpl.GetComponentId(), component.Msg.GetComponent().GetId())

		updated, err := client.SaveTemplate(ctx, connect.NewRequest(api.RequestSaveTemplate_builder{
			Id:          new(tmpl.GetId()),
			Title:       new("task"),
			ComponentId: new(int32(0)),
		}.Build()))
		assertion.MustNoError(t, err)
		assertion.Equal(t, updated.Msg.GetTemplate().GetTitle(), "task")
		assertion.Equal(t, updated.Msg.GetTemplate().GetBody(), "")
		assertion.False(t, updated.Msg.GetTemplate().HasComponentId())

		list, err := client.ListTemplates(ctx, connect.NewRequest(&api.RequestListTemplates{}))
		assertion.MustNoError(t, err)
		assertion.MustLen(t, list.Msg.GetTemplates(), 1)
		assertion.Equal(t, list.Msg.GetTemplates()[0].GetTitle(), "task")

		_, err = client.DeleteTemplate(ctx, connect.NewRequest(api.RequestDeleteTemplate_builder{Id: new(tmpl.GetId())}.Build()))
		assertion.MustNoError(t, err)
		_, err = client.DeleteTemplate(ctx, connect.NewRequest(api.RequestDeleteTemplate_builder{Id: new(tmpl.GetId())}.Build()))
		assertion.Equal(t, connect.CodeOf(err), connect.CodeNotFound)
	})

	t.Run("errors are mapped to the codes", func(t *testing.T) {
		_, err := client.ListTickets(ctx, connect.NewRequest(api.RequestListTickets_builder{Filter: new("status = CLOSED")}.Build()))
		assertion.Equal(t, connect.CodeOf(err), connect.CodeInvalidArgument)
		_, err = client.ListTickets(ctx, connect.NewRequest(api.RequestListTickets_builder{OrderBy: new("title")}.Build()))
		assertion.Equal(t, connect.CodeOf(err), connect.CodeInvalidArgument)
		_, err = client.GetTicket(ctx, connect.NewRequest(api.RequestGetTicket_builder{Number: new(int32(100))}.Build()))
		assertion.Equal(t, connect.CodeOf(err), connect.CodeNotFound)
		_, err = client.CreateTicket(ctx, connect.NewRequest(&api.RequestCreateTicket{}))
		assertion.Equal(t, connect.CodeOf(err), connect.CodeInvalidArgument)
		_, err = client.UpdateTicket(ctx, connect.NewRequest(api.RequestUpdateTicket_builder{
			Number: new(ticket.GetNumber()),
			Status: new(api.TicketStatus_TICKET_STATUS_UNSPECIFIED),
		}.Build()))
		assertion.Equal(t, connect.CodeOf(err), connect.CodeInvalidArgument)
		var connectErr *connect.Error
		assertion.True(t, errors.As(err, &connectErr))
	})
}

type fakeRepositories struct {
	urls map[int32]string
	err  error
}

func (r *fakeRepositories) URLs(_ context.Context, ids []int32) (map[int32]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	urls := make(map[int32]string)
	for _, id := range ids {
		if u, ok := r.urls[id]; ok {
			urls[id] = u
		}
	}
	return urls, nil
}

func TestGetTicketLinks(t *testing.T) {
	daos := dao.NewOptions(newConn(t))
	actor, err := daos.Actor.Create(context.Background(), &database.Actor{Kind: database.ActorKindHuman, Name: "human", Subject: new("human@example.com")})
	assertion.MustNoError(t, err)
	tracker := issue.NewTracker(daos)
	ctx := issue.WithActor(context.Background(), actor)
	ticket, err := tracker.Create(ctx, "title", "", database.TicketTypeNone, database.PriorityNone, nil, nil)
	assertion.MustNoError(t, err)
	committedAt := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	assertion.MustNoError(t, ticket.LinkCommit(ctx, 1, "4b825dc642cb6eb9a060e54bf8d69288fbee4904", "subject", "alice", database.TrailerKindFixes, committedAt))
	assertion.MustNoError(t, ticket.LinkCommit(ctx, 2, "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391", "unknown", "bob", database.TrailerKindRefs, committedAt))
	assertion.MustNoError(t, ticket.LinkPullRequest(ctx, 1, 3, "pull request", "alice", database.PullRequestStateMerged, "4b825dc642cb6eb9a060e54bf8d69288fbee4904"))
	req := connect.NewRequest(api.RequestGetTicket_builder{Number: new(ticket.Id)}.Build())

	t.Run("the links have the urls of the repositories of Build", func(t *testing.T) {
		res, err := NewIssueHandler(tracker, &fakeRepositories{urls: map[int32]string{1: "https://github.com/f110/mono"}}).GetTicket(ctx, req)
		assertion.MustNoError(t, err)
		commits := res.Msg.GetCommits()
		assertion.MustLen(t, commits, 2)
		assertion.Equal(t, commits[0].GetSha(), "4b825dc642cb6eb9a060e54bf8d69288fbee4904")
		assertion.Equal(t, commits[0].GetSubject(), "subject")
		assertion.Equal(t, commits[0].GetAuthor(), "alice")
		assertion.Equal(t, commits[0].GetTrailer(), api.TrailerKind_TRAILER_KIND_FIXES)
		assertion.Equal(t, commits[0].GetCommittedAt().AsTime(), committedAt)
		assertion.Equal(t, commits[0].GetUrl(), "https://github.com/f110/mono/commit/4b825dc642cb6eb9a060e54bf8d69288fbee4904")
		assertion.Equal(t, commits[1].GetUrl(), "")
		pullRequests := res.Msg.GetPullRequests()
		assertion.MustLen(t, pullRequests, 1)
		assertion.Equal(t, pullRequests[0].GetNumber(), int32(3))
		assertion.Equal(t, pullRequests[0].GetTitle(), "pull request")
		assertion.Equal(t, pullRequests[0].GetState(), api.PullRequestState_PULL_REQUEST_STATE_MERGED)
		assertion.Equal(t, pullRequests[0].GetUrl(), "https://github.com/f110/mono/pull/3")
	})

	t.Run("the links have no url without Build", func(t *testing.T) {
		res, err := NewIssueHandler(tracker, nil).GetTicket(ctx, req)
		assertion.MustNoError(t, err)
		assertion.MustLen(t, res.Msg.GetCommits(), 2)
		assertion.Equal(t, res.Msg.GetCommits()[0].GetUrl(), "")
		assertion.MustLen(t, res.Msg.GetPullRequests(), 1)
		assertion.Equal(t, res.Msg.GetPullRequests()[0].GetUrl(), "")
	})

	t.Run("the ticket is returned even if Build is unavailable", func(t *testing.T) {
		res, err := NewIssueHandler(tracker, &fakeRepositories{err: errors.New("unavailable")}).GetTicket(ctx, req)
		assertion.MustNoError(t, err)
		assertion.MustLen(t, res.Msg.GetCommits(), 2)
		assertion.Equal(t, res.Msg.GetCommits()[0].GetUrl(), "")
	})
}

func TestNew(t *testing.T) {
	daos := dao.NewOptions(newConn(t))
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assertion.MustNoError(t, err)
	s := httptest.NewServer(New(issue.NewTracker(daos), nil, func(h http.Handler) http.Handler { return authproxy.Authenticate(&key.PublicKey, h) }))
	t.Cleanup(s.Close)
	client := api.NewIssueClient(http.DefaultClient, s.URL)

	t.Run("the human of the subject is the actor", func(t *testing.T) {
		claims := &authproxy.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "alice@example.com", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}}
		token, err := jwt.NewWithClaims(jwt.SigningMethodES256, claims).SignedString(key)
		assertion.MustNoError(t, err)
		req := connect.NewRequest(&api.RequestWhoAmI{})
		req.Header().Set(authproxy.TokenHeaderName, token)
		res, err := client.WhoAmI(context.Background(), req)
		assertion.MustNoError(t, err)
		assertion.Equal(t, res.Msg.GetActor().GetName(), "alice@example.com")
		assertion.Equal(t, res.Msg.GetActor().GetKind(), api.ActorKind_ACTOR_KIND_HUMAN)
	})

	t.Run("without the token", func(t *testing.T) {
		_, err := client.WhoAmI(context.Background(), connect.NewRequest(&api.RequestWhoAmI{}))
		assertion.Equal(t, connect.CodeOf(err), connect.CodeUnauthenticated)
	})
}
