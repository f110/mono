// Package server provides the HTTP handler of Codexel which serves the API and the frontend.
package server

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/herdr"
)

// New returns the handler of the API and the frontend.
func New(herdrClient *herdr.Client, frontend fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(api.NewAgentServiceHandler(NewAgentHandler(herdrClient)))
	mux.Handle("/", newFrontendHandler(frontend))
	return mux
}

// newFrontendHandler serves the files of the frontend. The other paths return index.html because the frontend has
// its own routes.
func newFrontendHandler(frontend fs.FS) http.Handler {
	fileServer := http.FileServerFS(frontend)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		p := strings.TrimPrefix(req.URL.Path, "/")
		if p != "" {
			if _, err := fs.Stat(frontend, p); err == nil {
				fileServer.ServeHTTP(w, req)
				return
			}
		}
		http.ServeFileFS(w, req, frontend, "index.html")
	})
}

type AgentHandler struct {
	herdr *herdr.Client
}

var _ api.AgentServiceHandler = (*AgentHandler)(nil)

func NewAgentHandler(c *herdr.Client) *AgentHandler {
	return &AgentHandler{herdr: c}
}

func (h *AgentHandler) ListAgents(ctx context.Context, _ *connect.Request[api.RequestListAgents]) (*connect.Response[api.ResponseListAgents], error) {
	agents, _, err := h.listAgents(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseListAgents_builder{Agents: agents}.Build()), nil
}

// WatchAgents sends the agents every time herdr notifies the change of the panes or the status of the agents.
// The status of the agents can be subscribed only per pane, so the subscription is recreated after every change.
func (h *AgentHandler) WatchAgents(ctx context.Context, _ *connect.Request[api.RequestWatchAgents], stream *connect.ServerStream[api.ResponseWatchAgents]) error {
	for {
		agents, panes, err := h.listAgents(ctx)
		if err != nil {
			return err
		}
		if err := stream.Send(api.ResponseWatchAgents_builder{Agents: agents}.Build()); err != nil {
			return xerrors.WithStack(err)
		}

		subs := []herdr.Subscription{{Type: "pane.created"}, {Type: "pane.closed"}, {Type: "pane.updated"}, {Type: "pane.agent_detected"}}
		for _, v := range panes {
			subs = append(subs, herdr.Subscription{Type: "pane.agent_status_changed", PaneID: v})
		}
		for _, err := range h.herdr.Subscribe(ctx, subs) {
			if err != nil {
				return toConnectError(err)
			}
			break
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

func (h *AgentHandler) ReadOutput(ctx context.Context, req *connect.Request[api.RequestReadOutput]) (*connect.Response[api.ResponseReadOutput], error) {
	if req.Msg.GetPaneId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("pane_id is required"))
	}
	text, err := h.herdr.ReadAgent(ctx, req.Msg.GetPaneId(), herdr.ReadSourceRecent, herdr.ReadFormatANSI, int(req.Msg.GetLines()))
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(api.ResponseReadOutput_builder{Text: new(text)}.Build()), nil
}

func (h *AgentHandler) SendPrompt(ctx context.Context, req *connect.Request[api.RequestSendPrompt]) (*connect.Response[api.ResponseSendPrompt], error) {
	if req.Msg.GetPaneId() == "" || req.Msg.GetText() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("pane_id and text are required"))
	}
	if err := h.herdr.PromptAgent(ctx, req.Msg.GetPaneId(), req.Msg.GetText()); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&api.ResponseSendPrompt{}), nil
}

func (h *AgentHandler) SendKeys(ctx context.Context, req *connect.Request[api.RequestSendKeys]) (*connect.Response[api.ResponseSendKeys], error) {
	if req.Msg.GetPaneId() == "" || len(req.Msg.GetKeys()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("pane_id and keys are required"))
	}
	if err := h.herdr.SendKeys(ctx, req.Msg.GetPaneId(), req.Msg.GetKeys()); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&api.ResponseSendKeys{}), nil
}

func (h *AgentHandler) FocusAgent(ctx context.Context, req *connect.Request[api.RequestFocusAgent]) (*connect.Response[api.ResponseFocusAgent], error) {
	if req.Msg.GetPaneId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("pane_id is required"))
	}
	if err := h.herdr.FocusAgent(ctx, req.Msg.GetPaneId()); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&api.ResponseFocusAgent{}), nil
}

func (h *AgentHandler) ListWorkspaces(ctx context.Context, _ *connect.Request[api.RequestListWorkspaces]) (*connect.Response[api.ResponseListWorkspaces], error) {
	workspaces, err := h.herdr.ListWorkspaces(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	res := make([]*api.Workspace, 0, len(workspaces))
	for _, v := range workspaces {
		res = append(res, api.Workspace_builder{WorkspaceId: new(v.WorkspaceID), Label: new(v.Label)}.Build())
	}
	return connect.NewResponse(api.ResponseListWorkspaces_builder{Workspaces: res}.Build()), nil
}

func (h *AgentHandler) StartAgent(ctx context.Context, req *connect.Request[api.RequestStartAgent]) (*connect.Response[api.ResponseStartAgent], error) {
	if req.Msg.GetWorkspaceId() == "" || req.Msg.GetKind() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id and kind are required"))
	}
	name := req.Msg.GetName()
	if name == "" {
		name = req.Msg.GetKind()
	}

	pane, err := h.herdr.CreateTab(ctx, req.Msg.GetWorkspaceId(), req.Msg.GetCwd())
	if err != nil {
		return nil, toConnectError(err)
	}
	agent, err := h.herdr.StartAgent(ctx, name, req.Msg.GetKind(), pane.PaneID, req.Msg.GetArgs())
	if err != nil {
		return nil, toConnectError(err)
	}
	workspaces, err := h.workspaceLabels(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseStartAgent_builder{Agent: toAgent(agent, workspaces)}.Build()), nil
}

// listAgents returns the agents and the panes of them.
func (h *AgentHandler) listAgents(ctx context.Context) ([]*api.Agent, []string, error) {
	agents, err := h.herdr.ListAgents(ctx)
	if err != nil {
		return nil, nil, toConnectError(err)
	}
	workspaces, err := h.workspaceLabels(ctx)
	if err != nil {
		return nil, nil, err
	}
	res := make([]*api.Agent, 0, len(agents))
	panes := make([]string, 0, len(agents))
	for _, v := range agents {
		res = append(res, toAgent(v, workspaces))
		panes = append(panes, v.PaneID)
	}
	return res, panes, nil
}

func (h *AgentHandler) workspaceLabels(ctx context.Context) (map[string]string, error) {
	workspaces, err := h.herdr.ListWorkspaces(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	labels := make(map[string]string, len(workspaces))
	for _, v := range workspaces {
		labels[v.WorkspaceID] = v.Label
	}
	return labels, nil
}

func toAgent(a *herdr.Agent, workspaceLabels map[string]string) *api.Agent {
	var sessionID string
	kind := a.Agent
	if a.AgentSession != nil {
		sessionID = a.AgentSession.Value
		if a.AgentSession.Agent != "" {
			kind = a.AgentSession.Agent
		}
	}
	return api.Agent_builder{
		PaneId:         new(a.PaneID),
		WorkspaceId:    new(a.WorkspaceID),
		WorkspaceLabel: new(workspaceLabels[a.WorkspaceID]),
		TabId:          new(a.TabID),
		Kind:           new(kind),
		SessionId:      new(sessionID),
		Status:         new(toAgentStatus(a.AgentStatus)),
		Cwd:            new(a.Cwd),
		Title:          new(a.TerminalTitleStripped),
		Focused:        new(a.Focused),
	}.Build()
}

func toAgentStatus(s herdr.AgentStatus) api.AgentStatus {
	switch s {
	case herdr.AgentStatusIdle:
		return api.AgentStatus_AGENT_STATUS_IDLE
	case herdr.AgentStatusWorking:
		return api.AgentStatus_AGENT_STATUS_WORKING
	case herdr.AgentStatusBlocked:
		return api.AgentStatus_AGENT_STATUS_BLOCKED
	case herdr.AgentStatusDone:
		return api.AgentStatus_AGENT_STATUS_DONE
	default:
		return api.AgentStatus_AGENT_STATUS_UNKNOWN
	}
}

func toConnectError(err error) error {
	if herdrErr, ok := errors.AsType[*herdr.Error](err); ok {
		switch {
		case strings.HasSuffix(herdrErr.Code, "_not_found"):
			return connect.NewError(connect.CodeNotFound, err)
		case herdrErr.Code == "invalid_request":
			return connect.NewError(connect.CodeInvalidArgument, err)
		}
		return connect.NewError(connect.CodeUnknown, err)
	}
	if _, ok := errors.AsType[*net.OpError](err); ok {
		return connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}
