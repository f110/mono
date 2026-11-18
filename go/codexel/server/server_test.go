package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"connectrpc.com/connect"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/herdr"
	"go.f110.dev/mono/go/testing/assertion"
)

// startHerdr starts the fake herdr server. responses maps the method to the result or the error of the response.
func startHerdr(t *testing.T, responses map[string]string) *herdr.Client {
	t.Helper()

	// The path of the unix socket is limited to about 100 bytes. t.TempDir() may be too long.
	dir, err := os.MkdirTemp("/tmp", "herdr")
	assertion.MustNoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "s")
	l, err := net.Listen("unix", sock)
	assertion.MustNoError(t, err)
	t.Cleanup(func() { l.Close() })

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				line, err := bufio.NewReader(conn).ReadBytes('\n')
				if err != nil {
					return
				}
				var req struct {
					ID     string `json:"id"`
					Method string `json:"method"`
				}
				if err := json.Unmarshal(line, &req); err != nil {
					return
				}
				res, ok := responses[req.Method]
				if !ok {
					res = `"error":{"code":"unknown_method","message":"unknown"}`
				}
				conn.Write([]byte(`{"id":"` + req.ID + `",` + res + "}\n"))
			}()
		}
	}()
	return herdr.NewClient(sock)
}

func TestFrontendHandler(t *testing.T) {
	frontend := fstest.MapFS{
		"index.html":    {Data: []byte("index")},
		"assets/app.js": {Data: []byte("app")},
	}
	cases := []struct {
		Path string
		Body string
	}{
		{Path: "/", Body: "index"},
		{Path: "/assets/app.js", Body: "app"},
		{Path: "/agents/w1:p1", Body: "index"},
	}
	for _, tc := range cases {
		t.Run(tc.Path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newFrontendHandler(frontend).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.Path, nil))
			assertion.Equal(t, rec.Code, http.StatusOK)
			body, err := io.ReadAll(rec.Body)
			assertion.MustNoError(t, err)
			assertion.Equal(t, string(body), tc.Body)
		})
	}
}

func TestListAgents(t *testing.T) {
	c := startHerdr(t, map[string]string{
		"agent.list":     `"result":{"type":"agent_list","agents":[{"agent":"claude","agent_session":{"agent":"claude","kind":"id","source":"herdr:claude","value":"sess"},"agent_status":"working","cwd":"/repo","pane_id":"w1:p1","tab_id":"w1:t1","workspace_id":"w1","terminal_title_stripped":"Claude Code"}]}`,
		"workspace.list": `"result":{"type":"workspace_list","workspaces":[{"workspace_id":"w1","label":"mono"}]}`,
	})
	srv := httptest.NewServer(New(c, fstest.MapFS{}))
	t.Cleanup(srv.Close)

	client := api.NewAgentServiceClient(srv.Client(), srv.URL)
	res, err := client.ListAgents(t.Context(), connect.NewRequest(&api.RequestListAgents{}))
	assertion.MustNoError(t, err)
	agents := res.Msg.GetAgents()
	assertion.MustLen(t, agents, 1)
	assertion.Equal(t, agents[0].GetPaneId(), "w1:p1")
	assertion.Equal(t, agents[0].GetWorkspaceLabel(), "mono")
	assertion.Equal(t, agents[0].GetKind(), "claude")
	assertion.Equal(t, agents[0].GetSessionId(), "sess")
	assertion.Equal(t, agents[0].GetStatus(), api.AgentStatus_AGENT_STATUS_WORKING)
	assertion.Equal(t, agents[0].GetTitle(), "Claude Code")
}

func TestReadOutputNotFound(t *testing.T) {
	c := startHerdr(t, map[string]string{
		"agent.read": `"error":{"code":"agent_not_found","message":"agent target w9:p9 not found"}`,
	})
	h := NewAgentHandler(c)

	_, err := h.ReadOutput(context.Background(), connect.NewRequest(api.RequestReadOutput_builder{PaneId: new("w9:p9")}.Build()))
	assertion.MustError(t, err)
	connectErr, ok := errors.AsType[*connect.Error](err)
	assertion.True(t, ok)
	assertion.Equal(t, connectErr.Code(), connect.CodeNotFound)
}
