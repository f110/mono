package mcp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"go.f110.dev/mono/go/database/dbtestutil"
	"go.f110.dev/mono/go/http/authproxy"
	"go.f110.dev/mono/go/issue"
	"go.f110.dev/mono/go/issue/api"
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

type tokenTransport struct {
	token string
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set(authproxy.TokenHeaderName, t.token)
	return http.DefaultTransport.RoundTrip(req)
}

func newSession(t *testing.T, url string, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0.0.0"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:             url,
		HTTPClient:           &http.Client{Transport: &tokenTransport{token: token}},
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
	}
	session, err := client.Connect(context.Background(), transport, nil)
	assertion.MustNoError(t, err)
	t.Cleanup(func() { session.Close() })
	return session
}

func call(t *testing.T, session *mcp.ClientSession, name string, args any, out proto.Message) {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	assertion.MustNoError(t, err)
	text := res.Content[0].(*mcp.TextContent).Text
	if res.IsError {
		t.Fatalf("%s failed: %s", name, text)
	}
	assertion.MustNoError(t, protojson.Unmarshal([]byte(text), out))
}

func callError(t *testing.T, session *mcp.ClientSession, name string, args any) string {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	assertion.MustNoError(t, err)
	assertion.True(t, res.IsError)
	return res.Content[0].(*mcp.TextContent).Text
}

func TestNew(t *testing.T) {
	daos := dao.NewOptions(newConn(t))
	tracker := issue.NewTracker(daos)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assertion.MustNoError(t, err)
	s := httptest.NewServer(New(tracker, nil, func(h http.Handler) http.Handler { return authproxy.Authenticate(&key.PublicKey, h) }))
	t.Cleanup(s.Close)
	claims := &authproxy.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "alice@example.com", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodES256, claims).SignedString(key)
	assertion.MustNoError(t, err)
	session := newSession(t, s.URL, token)

	component, err := tracker.SaveComponent(issue.WithHuman(context.Background(), "bob@example.com"), 0, "build")
	assertion.MustNoError(t, err)

	t.Run("the agent which the human of the subject owns is the actor", func(t *testing.T) {
		res := &api.ResponseWhoAmI{}
		call(t, session, "who_am_i", map[string]any{}, res)
		assertion.Equal(t, res.GetActor().GetName(), "alice@example.com (agent)")
		assertion.Equal(t, res.GetActor().GetKind(), api.ActorKind_ACTOR_KIND_AGENT)
	})

	t.Run("create and update the ticket", func(t *testing.T) {
		me := &api.ResponseWhoAmI{}
		call(t, session, "who_am_i", map[string]any{}, me)

		created := &api.ResponseCreateTicket{}
		call(t, session, "create_ticket", map[string]any{"title": "title", "body": "body", "priority": "P1", "component_id": component.Id}, created)
		assertion.Equal(t, created.GetTicket().GetTitle(), "title")
		assertion.Equal(t, created.GetTicket().GetPriority(), api.Priority_PRIORITY_P1)
		assertion.Equal(t, created.GetTicket().GetComponentId(), component.Id)
		assertion.Equal(t, created.GetTicket().GetReporterId(), me.GetActor().GetId())

		number := created.GetTicket().GetNumber()
		updated := &api.ResponseUpdateTicket{}
		call(t, session, "update_ticket", map[string]any{"number": number, "status": "DONE", "priority": "NONE", "component_id": 0}, updated)
		assertion.Equal(t, updated.GetTicket().GetTitle(), "title")
		assertion.Equal(t, updated.GetTicket().GetStatus(), api.TicketStatus_TICKET_STATUS_DONE)
		assertion.Equal(t, updated.GetTicket().GetPriority(), api.Priority_PRIORITY_UNSPECIFIED)
		assertion.False(t, updated.GetTicket().HasComponentId())

		comment := &api.ResponseAddComment{}
		call(t, session, "add_comment", map[string]any{"ticket": number, "body": "comment"}, comment)
		assertion.Equal(t, comment.GetComment().GetActorId(), me.GetActor().GetId())

		other := &api.ResponseCreateTicket{}
		call(t, session, "create_ticket", map[string]any{"title": "other"}, other)
		relation := &api.ResponseAddRelation{}
		call(t, session, "add_relation", map[string]any{"from": number, "type": "BLOCKS", "to": other.GetTicket().GetNumber()}, relation)
		assertion.Equal(t, relation.GetRelation().GetType(), api.RelationType_RELATION_TYPE_BLOCKS)

		ticket := &api.ResponseGetTicket{}
		call(t, session, "get_ticket", map[string]any{"number": number}, ticket)
		assertion.Equal(t, len(ticket.GetComments()), 1)
		assertion.Equal(t, len(ticket.GetRelations()), 1)
	})

	t.Run("list the tickets by the pages", func(t *testing.T) {
		first := &api.ResponseListTickets{}
		call(t, session, "list_tickets", map[string]any{"order_by": "number", "page_size": 1}, first)
		assertion.MustLen(t, first.GetTickets(), 1)
		assertion.True(t, first.GetNextPageToken() != "")

		second := &api.ResponseListTickets{}
		call(t, session, "list_tickets", map[string]any{"order_by": "number", "page_size": 1, "page_token": first.GetNextPageToken()}, second)
		assertion.MustLen(t, second.GetTickets(), 1)
		assertion.True(t, second.GetTickets()[0].GetNumber() > first.GetTickets()[0].GetNumber())
	})

	t.Run("list the components", func(t *testing.T) {
		res := &api.ResponseListComponents{}
		call(t, session, "list_components", map[string]any{}, res)
		assertion.Equal(t, len(res.GetComponents()), 1)
		assertion.Equal(t, res.GetComponents()[0].GetName(), "build")
	})

	t.Run("the invalid input is the error of the tool", func(t *testing.T) {
		msg := callError(t, session, "update_ticket", map[string]any{"number": 1, "status": "CLOSED"})
		assertion.True(t, strings.Contains(msg, `invalid status: "CLOSED"`))

		msg = callError(t, session, "get_ticket", map[string]any{"number": 100})
		assertion.True(t, strings.Contains(msg, "not_found"))
	})

	t.Run("without the token", func(t *testing.T) {
		client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0.0.0"}, nil)
		_, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: s.URL, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
		assertion.MustError(t, err)
	})
}
