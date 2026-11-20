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
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"connectrpc.com/connect"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/codexel/config"
	"go.f110.dev/mono/go/codexel/pullrequest"
	"go.f110.dev/mono/go/codexel/review"
	"go.f110.dev/mono/go/herdr"
	"go.f110.dev/mono/go/testing/assertion"
)

// startHerdr starts the fake herdr server. responses maps the method to the result or the error of the response.
func startHerdr(t *testing.T, responses map[string]string) *herdr.Client {
	t.Helper()

	sequences := make(map[string][]string, len(responses))
	for k, v := range responses {
		sequences[k] = []string{v}
	}
	c, _ := startHerdrWithSequence(t, sequences)
	return c
}

type herdrRequest struct {
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

// startHerdrWithSequence starts the fake herdr server which returns the responses of the method in order. The last
// response is repeated. It returns the requests the server received.
func startHerdrWithSequence(t *testing.T, responses map[string][]string) (*herdr.Client, func() []*herdrRequest) {
	t.Helper()

	var mu sync.Mutex
	var received []*herdrRequest

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
					ID string `json:"id"`
					herdrRequest
				}
				if err := json.Unmarshal(line, &req); err != nil {
					return
				}

				mu.Lock()
				received = append(received, &req.herdrRequest)
				res := `"error":{"code":"unknown_method","message":"unknown"}`
				if seq := responses[req.Method]; len(seq) > 0 {
					res = seq[0]
					if len(seq) > 1 {
						responses[req.Method] = seq[1:]
					}
				}
				mu.Unlock()
				conn.Write([]byte(`{"id":"` + req.ID + `",` + res + "}\n"))
			}()
		}
	}()
	return herdr.NewClient(sock), func() []*herdrRequest {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(received)
	}
}

func methods(reqs []*herdrRequest) []string {
	res := make([]string, 0, len(reqs))
	for _, v := range reqs {
		res = append(res, v.Method)
	}
	return res
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
	srv := httptest.NewServer(New(c, config.NewFile(filepath.Join(t.TempDir(), "config.json")), review.NewStore(t.TempDir()), pullrequest.NewFile(filepath.Join(t.TempDir(), "pullrequests.json")), fstest.MapFS{}, "", nil))
	t.Cleanup(srv.Close)

	client := api.NewAgentServiceClient(srv.Client(), srv.URL)
	res, err := client.List(t.Context(), connect.NewRequest(&api.RequestListAgents{}))
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
	h := NewAgentHandler(c, nil, "")

	_, err := h.ReadOutput(context.Background(), connect.NewRequest(api.RequestReadOutput_builder{PaneId: new("w9:p9")}.Build()))
	assertion.MustError(t, err)
	connectErr, ok := errors.AsType[*connect.Error](err)
	assertion.True(t, ok)
	assertion.Equal(t, connectErr.Code(), connect.CodeNotFound)
}

func TestStartAgent(t *testing.T) {
	tabCreated := `"result":{"type":"tab_created","tab":{"tab_id":"w1:t2"},"root_pane":{"pane_id":"w1:p3","tab_id":"w1:t2","workspace_id":"w1"}}`
	agentStarted := `"result":{"type":"agent_started","argv":["claude"],"agent":{"agent":"claude","agent_status":"idle","pane_id":"w1:p3","tab_id":"w1:t2","workspace_id":"w1"}}`
	workspaces := `"result":{"type":"workspace_list","workspaces":[{"workspace_id":"w1","label":"mono"}]}`
	paneBusy := `"error":{"code":"agent_pane_busy","message":"agent target pane w1:p3 is not an available shell"}`

	t.Run("Retry while the pane is busy", func(t *testing.T) {
		c, received := startHerdrWithSequence(t, map[string][]string{
			"tab.create":     {tabCreated},
			"agent.start":    {paneBusy, agentStarted},
			"workspace.list": {workspaces},
		})
		h := NewAgentHandler(c, nil, "")

		res, err := h.Start(t.Context(), connect.NewRequest(api.RequestStartAgent_builder{WorkspaceId: new("w1"), Kind: new("claude")}.Build()))
		assertion.MustNoError(t, err)
		assertion.Equal(t, res.Msg.GetAgent().GetPaneId(), "w1:p3")
		assertion.Equal(t, methods(received()), []string{"tab.create", "agent.start", "agent.start", "workspace.list"})
	})

	t.Run("Unique name", func(t *testing.T) {
		c, received := startHerdrWithSequence(t, map[string][]string{
			"tab.create":     {tabCreated},
			"agent.start":    {agentStarted},
			"workspace.list": {workspaces},
		})
		h := NewAgentHandler(c, nil, "")

		for range 2 {
			_, err := h.Start(t.Context(), connect.NewRequest(api.RequestStartAgent_builder{WorkspaceId: new("w1"), Kind: new("claude")}.Build()))
			assertion.MustNoError(t, err)
		}
		var names []string
		for _, v := range received() {
			if v.Method == "agent.start" {
				names = append(names, v.Params["name"].(string))
			}
		}
		assertion.MustLen(t, names, 2)
		assertion.True(t, strings.HasPrefix(names[0], "claude-"))
		assertion.NotEqual(t, names[0], names[1])
	})

	t.Run("Close the tab on failure", func(t *testing.T) {
		c, received := startHerdrWithSequence(t, map[string][]string{
			"tab.create":  {tabCreated},
			"agent.start": {`"error":{"code":"agent_not_ready","message":"not ready"}`},
			"tab.close":   {`"result":{"type":"ok"}`},
		})
		h := NewAgentHandler(c, nil, "")

		_, err := h.Start(t.Context(), connect.NewRequest(api.RequestStartAgent_builder{WorkspaceId: new("w1"), Kind: new("claude")}.Build()))
		assertion.MustError(t, err)
		reqs := received()
		assertion.Equal(t, methods(reqs), []string{"tab.create", "agent.start", "tab.close"})
		assertion.Equal(t, reqs[2].Params["tab_id"], any("w1:t2"))
	})

	workspaceCreated := `"result":{"type":"workspace_created","workspace":{"workspace_id":"w2"},"tab":{"tab_id":"w2:t1"},"root_pane":{"pane_id":"w2:p1","tab_id":"w2:t1","workspace_id":"w2"}}`

	t.Run("New workspace", func(t *testing.T) {
		c, received := startHerdrWithSequence(t, map[string][]string{
			"workspace.create": {workspaceCreated},
			"agent.start":      {agentStarted},
			"workspace.list":   {workspaces},
		})
		h := NewAgentHandler(c, nil, "")

		_, err := h.Start(t.Context(), connect.NewRequest(api.RequestStartAgent_builder{Cwd: new("/repo"), Kind: new("claude")}.Build()))
		assertion.MustNoError(t, err)
		reqs := received()
		assertion.Equal(t, methods(reqs), []string{"workspace.create", "agent.start", "workspace.list"})
		assertion.Equal(t, reqs[0].Params["cwd"], any("/repo"))
		assertion.Equal(t, reqs[1].Params["pane_id"], any("w2:p1"))
	})

	t.Run("Close the new workspace on failure", func(t *testing.T) {
		c, received := startHerdrWithSequence(t, map[string][]string{
			"workspace.create": {workspaceCreated},
			"agent.start":      {`"error":{"code":"agent_not_ready","message":"not ready"}`},
			"workspace.close":  {`"result":{"type":"ok"}`},
		})
		h := NewAgentHandler(c, nil, "")

		_, err := h.Start(t.Context(), connect.NewRequest(api.RequestStartAgent_builder{Kind: new("claude")}.Build()))
		assertion.MustError(t, err)
		reqs := received()
		assertion.Equal(t, methods(reqs), []string{"workspace.create", "agent.start", "workspace.close"})
		assertion.Equal(t, reqs[2].Params["workspace_id"], any("w2"))
	})
}

func TestStopAgent(t *testing.T) {
	t.Run("Close the pane", func(t *testing.T) {
		c, received := startHerdrWithSequence(t, map[string][]string{
			"agent.get":  {`"result":{"type":"agent_info","agent":{"agent":"claude","agent_status":"working","pane_id":"w1:p3","tab_id":"w1:t2","workspace_id":"w1"}}`},
			"pane.close": {`"result":{"type":"ok"}`},
		})
		h := NewAgentHandler(c, nil, "")

		_, err := h.Stop(t.Context(), connect.NewRequest(api.RequestStopAgent_builder{PaneId: new("w1:p3")}.Build()))
		assertion.MustNoError(t, err)
		reqs := received()
		assertion.Equal(t, methods(reqs), []string{"agent.get", "pane.close"})
		assertion.Equal(t, reqs[1].Params["pane_id"], any("w1:p3"))
	})

	t.Run("Don't close the pane without the agent", func(t *testing.T) {
		c, received := startHerdrWithSequence(t, map[string][]string{
			"agent.get": {`"error":{"code":"agent_not_found","message":"agent target w1:p3 not found"}`},
		})
		h := NewAgentHandler(c, nil, "")

		_, err := h.Stop(t.Context(), connect.NewRequest(api.RequestStopAgent_builder{PaneId: new("w1:p3")}.Build()))
		assertion.MustError(t, err)
		connectErr, ok := errors.AsType[*connect.Error](err)
		assertion.True(t, ok)
		assertion.Equal(t, connectErr.Code(), connect.CodeNotFound)
		assertion.Equal(t, methods(received()), []string{"agent.get"})
	})
}

func TestReadOutputWhileWorking(t *testing.T) {
	c, received := startHerdrWithSequence(t, map[string][]string{
		"agent.read": {
			`"error":{"code":"agent_not_idle","message":"cannot read 2000 lines while w1:p1 is working"}`,
			`"result":{"type":"pane_read","read":{"pane_id":"w1:p1","text":"screen"}}`,
		},
	})
	h := NewAgentHandler(c, nil, "")

	res, err := h.ReadOutput(t.Context(), connect.NewRequest(api.RequestReadOutput_builder{PaneId: new("w1:p1"), Lines: new(int32(2000))}.Build()))
	assertion.MustNoError(t, err)
	assertion.Equal(t, res.Msg.GetText(), "screen")
	reqs := received()
	assertion.MustLen(t, reqs, 2)
	assertion.Equal(t, reqs[0].Params["lines"], any(float64(2000)))
	_, ok := reqs[1].Params["lines"]
	assertion.False(t, ok)
}

func TestWatchTranscript(t *testing.T) {
	dir := t.TempDir()
	assertion.MustNoError(t, os.MkdirAll(filepath.Join(dir, "-repo"), 0755))
	p := filepath.Join(dir, "-repo", "sess.jsonl")
	assertion.MustNoError(t, os.WriteFile(p, []byte(`{"type":"user","origin":{"kind":"human"},"message":{"content":"Fix it"},"timestamp":"2026-11-18T10:00:00Z"}`+"\n"), 0644))
	srv := httptest.NewServer(New(nil, config.NewFile(filepath.Join(t.TempDir(), "config.json")), review.NewStore(t.TempDir()), pullrequest.NewFile(filepath.Join(t.TempDir(), "pullrequests.json")), fstest.MapFS{}, dir, nil))
	t.Cleanup(srv.Close)

	client := api.NewAgentServiceClient(srv.Client(), srv.URL)
	stream, err := client.WatchTranscript(t.Context(), connect.NewRequest(api.RequestWatchTranscript_builder{SessionId: new("sess")}.Build()))
	assertion.MustNoError(t, err)
	t.Cleanup(func() { stream.Close() })

	assertion.True(t, stream.Receive())
	assertion.Equal(t, stream.Msg().GetTitle(), "Fix it")
	assertion.MustLen(t, stream.Msg().GetEntries(), 1)
	assertion.Equal(t, stream.Msg().GetEntries()[0].GetKind(), api.TranscriptEntryKind_TRANSCRIPT_ENTRY_KIND_USER_PROMPT)

	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
	assertion.MustNoError(t, err)
	_, err = f.WriteString(`{"type":"assistant","message":{"content":[{"type":"text","text":"Done"}],"usage":{"output_tokens":5}},"timestamp":"2026-11-18T10:00:01Z"}` + "\n")
	assertion.MustNoError(t, err)
	assertion.MustNoError(t, f.Close())

	assertion.True(t, stream.Receive())
	assertion.MustLen(t, stream.Msg().GetEntries(), 1)
	assertion.Equal(t, stream.Msg().GetEntries()[0].GetText(), "Done")
	assertion.Equal(t, stream.Msg().GetEntries()[0].GetUsage().GetOutputTokens(), int64(5))
}

func TestGetStatus(t *testing.T) {
	srv := httptest.NewServer(New(nil, config.NewFile(filepath.Join(t.TempDir(), "config.json")), review.NewStore(t.TempDir()), pullrequest.NewFile(filepath.Join(t.TempDir(), "pullrequests.json")), fstest.MapFS{}, "", []string{"jj"}))
	t.Cleanup(srv.Close)

	client := api.NewSystemServiceClient(srv.Client(), srv.URL)
	res, err := client.GetStatus(t.Context(), connect.NewRequest(&api.RequestGetStatus{}))
	assertion.MustNoError(t, err)
	assertion.Equal(t, res.Msg.GetMissingBinaries(), []string{"jj"})
}
