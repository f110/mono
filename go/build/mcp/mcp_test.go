package mcp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"go.f110.dev/mono/go/build/api"
	"go.f110.dev/mono/go/build/model"
	"go.f110.dev/mono/go/http/authproxy"
	"go.f110.dev/mono/go/testing/assertion"
)

type fakeAPIServer struct {
	api.UnimplementedAPIServer
	listTasks *api.RequestListTasks
	getLogs   *api.RequestGetLogs
}

func (f *fakeAPIServer) ListTasks(_ context.Context, req *api.RequestListTasks) (*api.ResponseListTasks, error) {
	f.listTasks = req
	return api.ResponseListTasks_builder{Tasks: []*model.Task{model.Task_builder{Id: new(int32(1))}.Build()}}.Build(), nil
}

func (f *fakeAPIServer) GetLogs(_ context.Context, req *api.RequestGetLogs) (*api.ResponseGetLogs, error) {
	f.getLogs = req
	if req.GetTaskId() != 1 {
		return nil, status.Error(codes.NotFound, "task not found")
	}
	return api.ResponseGetLogs_builder{Body: new("ERROR: failed\n")}.Build(), nil
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

func TestNew(t *testing.T) {
	fake := &fakeAPIServer{}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assertion.MustNoError(t, err)
	s := httptest.NewServer(New(fake, func(h http.Handler) http.Handler { return authproxy.Authenticate(&key.PublicKey, h) }))
	t.Cleanup(s.Close)
	claims := &authproxy.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "alice@example.com", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodES256, claims).SignedString(key)
	assertion.MustNoError(t, err)
	session := newSession(t, s.URL, token)

	t.Run("only the read operations are exposed", func(t *testing.T) {
		res, err := session.ListTools(context.Background(), nil)
		assertion.MustNoError(t, err)
		var names []string
		for _, v := range res.Tools {
			names = append(names, v.Name)
		}
		slices.Sort(names)
		assertion.Equal(t, []string{
			"get_logs", "get_server_info", "list_artifacts", "list_external_release_triggers", "list_github_events", "list_jobs", "list_repositories", "list_tasks",
		}, names)
	})

	t.Run("list the tasks", func(t *testing.T) {
		res := &api.ResponseListTasks{}
		call(t, session, "list_tasks", map[string]any{"repository_ids": []int32{1}, "revisions": []string{"abc"}}, res)
		assertion.Len(t, res.GetTasks(), 1)
		assertion.Equal(t, []int32{1}, fake.listTasks.GetRepositoryIds())
		assertion.Equal(t, []string{"abc"}, fake.listTasks.GetRevisions())
		assertion.False(t, fake.listTasks.HasPageSize())
	})

	t.Run("get the logs with the filter", func(t *testing.T) {
		res := &api.ResponseGetLogs{}
		call(t, session, "get_logs", map[string]any{"task_id": 1, "filter": "^ERROR"}, res)
		assertion.Equal(t, "ERROR: failed\n", res.GetBody())
		assertion.Equal(t, "^ERROR", fake.getLogs.GetFilter())
	})

	t.Run("the error of the API is the error of the tool", func(t *testing.T) {
		res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_logs", Arguments: map[string]any{"task_id": 2}})
		assertion.MustNoError(t, err)
		assertion.True(t, res.IsError)
		assertion.True(t, strings.Contains(res.Content[0].(*mcp.TextContent).Text, "task not found"))
	})

	t.Run("without the token", func(t *testing.T) {
		client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0.0.0"}, nil)
		_, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: s.URL, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
		assertion.MustError(t, err)
	})
}
