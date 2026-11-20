package herdr

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.f110.dev/mono/go/testing/assertion"
)

type request struct {
	ID     string         `json:"id"`
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

// startServer starts the fake herdr server. handle returns the lines written after reading the request.
func startServer(t *testing.T, handle func(req *request) []string) (*Client, <-chan *request) {
	t.Helper()

	// The path of the unix socket is limited to about 100 bytes. t.TempDir() may be too long.
	dir, err := os.MkdirTemp("/tmp", "herdr")
	assertion.MustNoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "s")
	l, err := net.Listen("unix", sock)
	assertion.MustNoError(t, err)
	t.Cleanup(func() { l.Close() })

	reqs := make(chan *request, 10)
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
				req := &request{}
				if err := json.Unmarshal(line, req); err != nil {
					return
				}
				reqs <- req
				for _, v := range handle(req) {
					if _, err := conn.Write([]byte(v + "\n")); err != nil {
						return
					}
				}
			}()
		}
	}()
	return NewClient(sock), reqs
}

func TestListAgents(t *testing.T) {
	c, reqs := startServer(t, func(req *request) []string {
		return []string{`{"id":"` + req.ID + `","result":{"type":"agent_list","agents":[{"agent":"claude","agent_session":{"agent":"claude","kind":"id","source":"herdr:claude","value":"sess"},"agent_status":"idle","cwd":"/repo","pane_id":"w1:p1","tab_id":"w1:t1","workspace_id":"w1"}]}}`}
	})

	agents, err := c.ListAgents(t.Context())
	assertion.MustNoError(t, err)
	assertion.MustLen(t, agents, 1)
	assertion.Equal(t, agents[0].PaneID, "w1:p1")
	assertion.Equal(t, agents[0].AgentStatus, AgentStatusIdle)
	assertion.Equal(t, agents[0].AgentSession.Agent, "claude")
	assertion.Equal(t, agents[0].AgentSession.Value, "sess")
	assertion.Equal(t, (<-reqs).Method, "agent.list")
}

func TestReadAgent(t *testing.T) {
	c, reqs := startServer(t, func(req *request) []string {
		return []string{`{"id":"` + req.ID + `","result":{"type":"pane_read","read":{"pane_id":"w1:p1","text":"hello"}}}`}
	})

	text, err := c.ReadAgent(t.Context(), "w1:p1", ReadSourceRecent, ReadFormatANSI, 0)
	assertion.MustNoError(t, err)
	assertion.Equal(t, text, "hello")
	req := <-reqs
	assertion.Equal(t, req.Method, "agent.read")
	assertion.Equal(t, req.Params["target"], any("w1:p1"))
	assertion.Equal(t, req.Params["source"], any("recent"))
	assertion.Equal(t, req.Params["format"], any("ansi"))
	_, ok := req.Params["lines"]
	assertion.False(t, ok)
}

func TestCreateTab(t *testing.T) {
	c, reqs := startServer(t, func(req *request) []string {
		return []string{`{"id":"` + req.ID + `","result":{"type":"tab_created","tab":{"tab_id":"w1:t2"},"root_pane":{"pane_id":"w1:p3","tab_id":"w1:t2","workspace_id":"w1"}}}`}
	})

	pane, err := c.CreateTab(t.Context(), "w1", "/repo")
	assertion.MustNoError(t, err)
	assertion.Equal(t, pane.PaneID, "w1:p3")
	req := <-reqs
	assertion.Equal(t, req.Method, "tab.create")
	assertion.Equal(t, req.Params["cwd"], any("/repo"))
}

func TestCreateWorkspace(t *testing.T) {
	c, reqs := startServer(t, func(req *request) []string {
		return []string{`{"id":"` + req.ID + `","result":{"type":"workspace_created","workspace":{"workspace_id":"w2"},"tab":{"tab_id":"w2:t1"},"root_pane":{"pane_id":"w2:p1","tab_id":"w2:t1","workspace_id":"w2"}}}`}
	})

	pane, err := c.CreateWorkspace(t.Context(), "/repo")
	assertion.MustNoError(t, err)
	assertion.Equal(t, pane.PaneID, "w2:p1")
	req := <-reqs
	assertion.Equal(t, req.Method, "workspace.create")
	assertion.Equal(t, req.Params["cwd"], any("/repo"))
}

func TestCloseWorkspace(t *testing.T) {
	c, reqs := startServer(t, func(req *request) []string {
		return []string{`{"id":"` + req.ID + `","result":{"type":"ok"}}`}
	})

	assertion.MustNoError(t, c.CloseWorkspace(t.Context(), "w2"))
	req := <-reqs
	assertion.Equal(t, req.Method, "workspace.close")
	assertion.Equal(t, req.Params["workspace_id"], any("w2"))
}

func TestCloseTab(t *testing.T) {
	c, reqs := startServer(t, func(req *request) []string {
		return []string{`{"id":"` + req.ID + `","result":{"type":"ok"}}`}
	})

	assertion.MustNoError(t, c.CloseTab(t.Context(), "w1:t2"))
	req := <-reqs
	assertion.Equal(t, req.Method, "tab.close")
	assertion.Equal(t, req.Params["tab_id"], any("w1:t2"))
}

func TestClosePane(t *testing.T) {
	c, reqs := startServer(t, func(req *request) []string {
		return []string{`{"id":"` + req.ID + `","result":{"type":"ok"}}`}
	})

	assertion.MustNoError(t, c.ClosePane(t.Context(), "w1:p3"))
	req := <-reqs
	assertion.Equal(t, req.Method, "pane.close")
	assertion.Equal(t, req.Params["pane_id"], any("w1:p3"))
}

func TestError(t *testing.T) {
	c, _ := startServer(t, func(req *request) []string {
		return []string{`{"id":"` + req.ID + `","error":{"code":"not_found","message":"agent not found"}}`}
	})

	err := c.PromptAgent(t.Context(), "w1:p1", "hello")
	assertion.MustError(t, err)
	var herdrErr *Error
	assertion.True(t, errors.As(err, &herdrErr))
	assertion.Equal(t, herdrErr.Code, "not_found")
}

func TestSubscribe(t *testing.T) {
	c, reqs := startServer(t, func(req *request) []string {
		return []string{
			`{"id":"` + req.ID + `","result":{"type":"subscription_started"}}`,
			`{"event":"pane.agent_status_changed","data":{"pane_id":"w1:p1","workspace_id":"w1","agent_status":"working"}}`,
			`{"event":"pane_closed","data":{"pane_id":"w1:p1"}}`,
		}
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var events []*Event
	for e, err := range c.Subscribe(ctx, []Subscription{{Type: "pane.closed"}, {Type: "pane.agent_status_changed", PaneID: "w1:p1"}}) {
		assertion.MustNoError(t, err)
		events = append(events, e)
		if len(events) == 2 {
			break
		}
	}
	assertion.MustLen(t, events, 2)
	assertion.Equal(t, events[0].Event, "pane.agent_status_changed")
	assertion.Equal(t, events[1].Event, "pane_closed")

	req := <-reqs
	assertion.Equal(t, req.Method, "events.subscribe")
	subs := req.Params["subscriptions"].([]any)
	assertion.MustLen(t, subs, 2)
	assertion.Equal(t, subs[1].(map[string]any)["pane_id"], any("w1:p1"))
}

func TestStartServer(t *testing.T) {
	c, _ := startServer(t, func(req *request) []string { return nil })
	bin := t.TempDir()
	out := filepath.Join(t.TempDir(), "out")
	script := "#!/bin/sh\necho \"$* $HERDR_SOCKET_PATH $HERDR_PANE_ID\" > " + out + ".tmp && mv " + out + ".tmp " + out + "\n"
	assertion.MustNoError(t, os.WriteFile(filepath.Join(bin, "herdr"), []byte(script), 0755))
	t.Setenv("PATH", bin+":/bin:/usr/bin")
	t.Setenv("HERDR_PANE_ID", "w1:p1")

	assertion.MustNoError(t, c.StartServer(t.Context()))
	var buf []byte
	for range 50 {
		var err error
		if buf, err = os.ReadFile(out); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	assertion.Equal(t, string(buf), "server "+c.socketPath+" \n")
}

func TestStartServerTimeout(t *testing.T) {
	bin := t.TempDir()
	assertion.MustNoError(t, os.WriteFile(filepath.Join(bin, "herdr"), []byte("#!/bin/sh\n"), 0755))
	t.Setenv("PATH", bin)
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	err := NewClient(filepath.Join(t.TempDir(), "s")).StartServer(ctx)
	assertion.MustError(t, err)
}
