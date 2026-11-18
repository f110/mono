// Package herdr provides a client of the socket API of herdr.
//
// herdr accepts one newline-delimited JSON request per connection and returns one response.
// A connection of events.subscribe keeps streaming the events after the response.
package herdr

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"

	"go.f110.dev/xerrors"
)

type AgentStatus string

const (
	AgentStatusIdle    AgentStatus = "idle"
	AgentStatusWorking AgentStatus = "working"
	AgentStatusBlocked AgentStatus = "blocked"
	AgentStatusDone    AgentStatus = "done"
	AgentStatusUnknown AgentStatus = "unknown"
)

type ReadSource string

const (
	ReadSourceVisible         ReadSource = "visible"
	ReadSourceRecent          ReadSource = "recent"
	ReadSourceRecentUnwrapped ReadSource = "recent_unwrapped"
)

type ReadFormat string

const (
	ReadFormatText ReadFormat = "text"
	ReadFormatANSI ReadFormat = "ansi"
)

// AgentSession identifies the session of the agent. Value is the session ID of the agent itself
// (e.g. the session ID of Claude Code).
type AgentSession struct {
	Agent  string `json:"agent"`
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Value  string `json:"value"`
}

type Agent struct {
	Agent                 string        `json:"agent"`
	AgentSession          *AgentSession `json:"agent_session"`
	AgentStatus           AgentStatus   `json:"agent_status"`
	Cwd                   string        `json:"cwd"`
	ForegroundCwd         string        `json:"foreground_cwd"`
	Focused               bool          `json:"focused"`
	PaneID                string        `json:"pane_id"`
	TabID                 string        `json:"tab_id"`
	WorkspaceID           string        `json:"workspace_id"`
	TerminalID            string        `json:"terminal_id"`
	TerminalTitle         string        `json:"terminal_title"`
	TerminalTitleStripped string        `json:"terminal_title_stripped"`
}

type Workspace struct {
	WorkspaceID string      `json:"workspace_id"`
	Number      int         `json:"number"`
	Label       string      `json:"label"`
	Focused     bool        `json:"focused"`
	PaneCount   int         `json:"pane_count"`
	TabCount    int         `json:"tab_count"`
	ActiveTabID string      `json:"active_tab_id"`
	AgentStatus AgentStatus `json:"agent_status"`
}

type Pane struct {
	PaneID      string `json:"pane_id"`
	TabID       string `json:"tab_id"`
	WorkspaceID string `json:"workspace_id"`
}

// Subscription is the event type to subscribe. PaneID is required by some types (e.g. pane.agent_status_changed).
type Subscription struct {
	Type   string `json:"type"`
	PaneID string `json:"pane_id,omitempty"`
}

// Event is an event delivered by Subscribe. Data depends on Event.
type Event struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// Error is the error returned by herdr.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("herdr: %s: %s", e.Code, e.Message)
}

type Client struct {
	socketPath string
	seq        atomic.Uint64
}

func NewClient(socketPath string) *Client {
	return &Client{socketPath: socketPath}
}

// DefaultSocketPath returns $HERDR_SOCKET_PATH or ~/.config/herdr/herdr.sock.
func DefaultSocketPath() (string, error) {
	if v := os.Getenv("HERDR_SOCKET_PATH"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	return filepath.Join(home, ".config", "herdr", "herdr.sock"), nil
}

func (c *Client) ListAgents(ctx context.Context) ([]*Agent, error) {
	var res struct {
		Agents []*Agent `json:"agents"`
	}
	if err := c.call(ctx, "agent.list", struct{}{}, &res); err != nil {
		return nil, err
	}
	return res.Agents, nil
}

func (c *Client) GetAgent(ctx context.Context, target string) (*Agent, error) {
	var res struct {
		Agent *Agent `json:"agent"`
	}
	if err := c.call(ctx, "agent.get", map[string]any{"target": target}, &res); err != nil {
		return nil, err
	}
	return res.Agent, nil
}

// ReadAgent returns the terminal output of the agent. lines is ignored if it is zero.
func (c *Client) ReadAgent(ctx context.Context, target string, source ReadSource, format ReadFormat, lines int) (string, error) {
	params := map[string]any{"target": target, "source": source, "format": format}
	if lines > 0 {
		params["lines"] = lines
	}
	var res struct {
		Read struct {
			Text string `json:"text"`
		} `json:"read"`
	}
	if err := c.call(ctx, "agent.read", params, &res); err != nil {
		return "", err
	}
	return res.Read.Text, nil
}

func (c *Client) PromptAgent(ctx context.Context, target, text string) error {
	return c.call(ctx, "agent.prompt", map[string]any{"target": target, "text": text}, nil)
}

func (c *Client) SendKeys(ctx context.Context, target string, keys []string) error {
	return c.call(ctx, "agent.send_keys", map[string]any{"target": target, "keys": keys}, nil)
}

func (c *Client) FocusAgent(ctx context.Context, target string) error {
	return c.call(ctx, "agent.focus", map[string]any{"target": target}, nil)
}

// StartAgent starts the agent in the pane. The pane must be at its interactive shell prompt.
func (c *Client) StartAgent(ctx context.Context, name, kind, paneID string, args []string) (*Agent, error) {
	params := map[string]any{"name": name, "kind": kind, "pane_id": paneID}
	if len(args) > 0 {
		params["args"] = args
	}
	var res struct {
		Agent *Agent `json:"agent"`
	}
	if err := c.call(ctx, "agent.start", params, &res); err != nil {
		return nil, err
	}
	return res.Agent, nil
}

func (c *Client) ListWorkspaces(ctx context.Context) ([]*Workspace, error) {
	var res struct {
		Workspaces []*Workspace `json:"workspaces"`
	}
	if err := c.call(ctx, "workspace.list", struct{}{}, &res); err != nil {
		return nil, err
	}
	return res.Workspaces, nil
}

// CreateTab creates the tab in the workspace and returns the root pane of the tab.
func (c *Client) CreateTab(ctx context.Context, workspaceID, cwd string) (*Pane, error) {
	params := map[string]any{"workspace_id": workspaceID}
	if cwd != "" {
		params["cwd"] = cwd
	}
	var res struct {
		RootPane *Pane `json:"root_pane"`
	}
	if err := c.call(ctx, "tab.create", params, &res); err != nil {
		return nil, err
	}
	return res.RootPane, nil
}

func (c *Client) CloseTab(ctx context.Context, tabID string) error {
	return c.call(ctx, "tab.close", map[string]any{"tab_id": tabID}, nil)
}

// Subscribe subscribes the events. The iteration continues until ctx is canceled or the connection is closed.
func (c *Client) Subscribe(ctx context.Context, subs []Subscription) iter.Seq2[*Event, error] {
	return func(yield func(*Event, error) bool) {
		conn, r, err := c.request(ctx, "events.subscribe", map[string]any{"subscriptions": subs})
		if err != nil {
			yield(nil, err)
			return
		}
		defer conn.Close()
		if err := readResponse(r, nil); err != nil {
			yield(nil, err)
			return
		}

		stop := context.AfterFunc(ctx, func() { conn.Close() })
		defer stop()
		for {
			line, err := r.ReadBytes('\n')
			if err != nil {
				if ctx.Err() == nil {
					yield(nil, xerrors.WithStack(err))
				}
				return
			}
			e := &Event{}
			if err := json.Unmarshal(line, e); err != nil {
				if !yield(nil, xerrors.WithStack(err)) {
					return
				}
				continue
			}
			if !yield(e, nil) {
				return
			}
		}
	}
}

func (c *Client) call(ctx context.Context, method string, params, result any) error {
	conn, r, err := c.request(ctx, method, params)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	if err := readResponse(r, result); err != nil {
		if ctx.Err() != nil {
			return xerrors.WithStack(ctx.Err())
		}
		return err
	}
	return nil
}

func (c *Client) request(ctx context.Context, method string, params any) (net.Conn, *bufio.Reader, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", c.socketPath)
	if err != nil {
		return nil, nil, xerrors.WithStack(err)
	}
	req := struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params"`
	}{ID: strconv.FormatUint(c.seq.Add(1), 10), Method: method, Params: params}
	buf, err := json.Marshal(req)
	if err != nil {
		conn.Close()
		return nil, nil, xerrors.WithStack(err)
	}
	if _, err := conn.Write(append(buf, '\n')); err != nil {
		conn.Close()
		return nil, nil, xerrors.WithStack(err)
	}
	return conn, bufio.NewReader(conn), nil
}

func readResponse(r *bufio.Reader, result any) error {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return xerrors.WithStack(err)
	}
	var res struct {
		Result json.RawMessage `json:"result"`
		Error  *Error          `json:"error"`
	}
	if err := json.Unmarshal(line, &res); err != nil {
		return xerrors.WithStack(err)
	}
	if res.Error != nil {
		return xerrors.WithStack(res.Error)
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(res.Result, result); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}
