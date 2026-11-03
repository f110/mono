// Package mcp exposes the operations of the tickets to the coding agents by MCP. Every tool calls the handler of the
// Connect API, so the tools behave the same as the Connect API. The actor of the operations is the agent which the human
// of the subject of heimdallr owns.
package mcp

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.f110.dev/xerrors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"go.f110.dev/mono/go/http/authproxy"
	"go.f110.dev/mono/go/issue"
	"go.f110.dev/mono/go/issue/api"
	"go.f110.dev/mono/go/issue/server"
)

type listTicketsInput struct {
	Filter  string `json:"filter,omitempty" jsonschema:"Filter of the tickets written in AIP-160. The fields are status, priority, component, assignee, reporter, title and body. component, assignee and reporter are the names. The value without the field matches the title, the body and the comments partially. e.g. (status = OPEN OR status = IN_PROGRESS) AND priority <= P1 AND component = build crash"`
	OrderBy string `json:"order_by,omitempty" jsonschema:"Order of the tickets written in AIP-132. The fields are number, status, priority, component, assignee, created_at and last_activity_at. The default is last_activity_at desc"`
}

type ticketInput struct {
	Number int32 `json:"number" jsonschema:"Number of the ticket"`
}

type createTicketInput struct {
	Title       string `json:"title"`
	Body        string `json:"body,omitempty" jsonschema:"Body of the ticket written in Markdown. #123 is the link to the ticket 123"`
	Priority    string `json:"priority,omitempty" jsonschema:"One of P0, P1, P2, P3 and P4. P0 is the highest. Omit to leave the priority unset"`
	ComponentID *int32 `json:"component_id,omitempty" jsonschema:"ID of the component. See list_components"`
	AssigneeID  *int32 `json:"assignee_id,omitempty" jsonschema:"ID of the actor who is assigned. See list_actors"`
}

type updateTicketInput struct {
	Number      int32   `json:"number" jsonschema:"Number of the ticket"`
	Title       *string `json:"title,omitempty"`
	Body        *string `json:"body,omitempty" jsonschema:"Body of the ticket written in Markdown"`
	Status      *string `json:"status,omitempty" jsonschema:"One of OPEN, IN_PROGRESS, DONE and WONT_FIX"`
	Priority    *string `json:"priority,omitempty" jsonschema:"One of P0, P1, P2, P3, P4 and NONE. NONE clears the priority"`
	ComponentID *int32  `json:"component_id,omitempty" jsonschema:"ID of the component. 0 clears the component. See list_components"`
	AssigneeID  *int32  `json:"assignee_id,omitempty" jsonschema:"ID of the actor who is assigned. 0 clears the assignee. See list_actors"`
}

type relationInput struct {
	From int32  `json:"from" jsonschema:"Number of the ticket"`
	Type string `json:"type" jsonschema:"One of PARENT_OF, BLOCKS, DUPLICATE_OF and RELATES_TO. e.g. from BLOCKS to means the ticket from blocks the ticket to"`
	To   int32  `json:"to" jsonschema:"Number of the ticket"`
}

type addCommentInput struct {
	Ticket int32  `json:"ticket" jsonschema:"Number of the ticket"`
	Body   string `json:"body" jsonschema:"Body of the comment written in Markdown"`
}

type editCommentInput struct {
	ID   int32  `json:"id" jsonschema:"ID of the comment"`
	Body string `json:"body" jsonschema:"Body of the comment written in Markdown"`
}

type commentInput struct {
	ID int32 `json:"id" jsonschema:"ID of the comment"`
}

type activitiesInput struct {
	Ticket int32 `json:"ticket" jsonschema:"Number of the ticket"`
}

type emptyInput struct{}

// New returns the HTTP handler which serves MCP. The requests are authenticated by authenticate which puts the claims of
// heimdallr to the context (e.g. authproxy.Authenticate), and the agent which the human of the subject owns is the actor
// of the operations.
func New(tracker *issue.Tracker, authenticate func(http.Handler) http.Handler) http.Handler {
	s := newServer(server.NewIssueHandler(tracker))
	// The handler is stateless so that every request is operated by the actor of the request.
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, &mcp.StreamableHTTPOptions{Stateless: true})
	return authenticate(withAgent(handler))
}

// newServer returns the MCP server which has the tools of the operations of the tickets.
func newServer(h *server.IssueHandler) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "issue", Version: "v0.1.0"}, nil)
	addTool(s, "list_tickets", "List the tickets which match the filter", h.ListTickets, func(in listTicketsInput) (*api.RequestListTickets, error) {
		return api.RequestListTickets_builder{Filter: &in.Filter, OrderBy: &in.OrderBy}.Build(), nil
	})
	addTool(s, "get_ticket", "Get the ticket with the relations and the comments", h.GetTicket, func(in ticketInput) (*api.RequestGetTicket, error) {
		return api.RequestGetTicket_builder{Number: &in.Number}.Build(), nil
	})
	addTool(s, "create_ticket", "Create the ticket", h.CreateTicket, func(in createTicketInput) (*api.RequestCreateTicket, error) {
		b := api.RequestCreateTicket_builder{Title: &in.Title, Body: &in.Body, ComponentId: in.ComponentID, AssigneeId: in.AssigneeID}
		if in.Priority != "" {
			p, err := toPriority(in.Priority)
			if err != nil {
				return nil, err
			}
			b.Priority = &p
		}
		return b.Build(), nil
	})
	addTool(s, "update_ticket", "Update the fields of the ticket. Only the specified fields are changed", h.UpdateTicket, func(in updateTicketInput) (*api.RequestUpdateTicket, error) {
		b := api.RequestUpdateTicket_builder{Number: &in.Number, Title: in.Title, Body: in.Body, ComponentId: in.ComponentID, AssigneeId: in.AssigneeID}
		if in.Status != nil {
			s, err := parseEnum[api.TicketStatus](api.TicketStatus_value, "TICKET_STATUS_", "status", *in.Status)
			if err != nil {
				return nil, err
			}
			b.Status = &s
		}
		if in.Priority != nil {
			p, err := toPriority(*in.Priority)
			if err != nil {
				return nil, err
			}
			b.Priority = &p
		}
		return b.Build(), nil
	})
	addTool(s, "add_relation", "Add the relation between the tickets", h.AddRelation, func(in relationInput) (*api.RequestAddRelation, error) {
		r, err := toRelation(in)
		if err != nil {
			return nil, err
		}
		return api.RequestAddRelation_builder{Relation: r}.Build(), nil
	})
	addTool(s, "remove_relation", "Remove the relation between the tickets", h.RemoveRelation, func(in relationInput) (*api.RequestRemoveRelation, error) {
		r, err := toRelation(in)
		if err != nil {
			return nil, err
		}
		return api.RequestRemoveRelation_builder{Relation: r}.Build(), nil
	})
	addTool(s, "add_comment", "Add the comment to the ticket", h.AddComment, func(in addCommentInput) (*api.RequestAddComment, error) {
		return api.RequestAddComment_builder{Ticket: &in.Ticket, Body: &in.Body}.Build(), nil
	})
	addTool(s, "edit_comment", "Edit the comment", h.EditComment, func(in editCommentInput) (*api.RequestEditComment, error) {
		return api.RequestEditComment_builder{Id: &in.ID, Body: &in.Body}.Build(), nil
	})
	addTool(s, "delete_comment", "Delete the comment", h.DeleteComment, func(in commentInput) (*api.RequestDeleteComment, error) {
		return api.RequestDeleteComment_builder{Id: &in.ID}.Build(), nil
	})
	addTool(s, "list_activities", "List the history of the operations to the ticket", h.ListActivities, func(in activitiesInput) (*api.RequestListActivities, error) {
		return api.RequestListActivities_builder{Ticket: &in.Ticket}.Build(), nil
	})
	addTool(s, "list_ticket_revisions", "List the bodies of the ticket before the edits", h.ListTicketRevisions, func(in activitiesInput) (*api.RequestListTicketRevisions, error) {
		return api.RequestListTicketRevisions_builder{Ticket: &in.Ticket}.Build(), nil
	})
	addTool(s, "list_comment_revisions", "List the bodies of the comment before the edits", h.ListCommentRevisions, func(in commentInput) (*api.RequestListCommentRevisions, error) {
		return api.RequestListCommentRevisions_builder{Id: &in.ID}.Build(), nil
	})
	addTool(s, "list_components", "List the components which the tickets can be classified into", h.ListComponents, func(emptyInput) (*api.RequestListComponents, error) {
		return &api.RequestListComponents{}, nil
	})
	addTool(s, "list_actors", "List the actors. The id is used as assignee_id", h.ListActors, func(emptyInput) (*api.RequestListActors, error) {
		return &api.RequestListActors{}, nil
	})
	addTool(s, "who_am_i", "Get the actor of the operations", h.WhoAmI, func(emptyInput) (*api.RequestWhoAmI, error) {
		return &api.RequestWhoAmI{}, nil
	})
	return s
}

var marshalOptions = protojson.MarshalOptions{UseProtoNames: true}

// addTool adds the tool which converts the input to the request by toRequest and calls rpc. The response is returned as
// JSON. The error is returned as the error of the tool so that the agent can read the reason.
func addTool[In, Req, Res any](s *mcp.Server, name, description string, rpc func(context.Context, *connect.Request[Req]) (*connect.Response[Res], error), toRequest func(In) (*Req, error)) {
	mcp.AddTool(s, &mcp.Tool{Name: name, Description: description}, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		req, err := toRequest(in)
		if err != nil {
			return nil, nil, err
		}
		res, err := rpc(ctx, connect.NewRequest(req))
		if err != nil {
			return nil, nil, err
		}
		msg, ok := any(res.Msg).(proto.Message)
		if !ok {
			return nil, nil, xerrors.NewfWithStack("%T is not a proto message", res.Msg)
		}
		b, err := marshalOptions.Marshal(msg)
		if err != nil {
			return nil, nil, xerrors.WithStack(err)
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil, nil
	})
}

func toPriority(s string) (api.Priority, error) {
	if s == "NONE" {
		return api.Priority_PRIORITY_UNSPECIFIED, nil
	}
	return parseEnum[api.Priority](api.Priority_value, "PRIORITY_", "priority", s)
}

func toRelation(in relationInput) (*api.Relation, error) {
	t, err := parseEnum[api.RelationType](api.RelationType_value, "RELATION_TYPE_", "type", in.Type)
	if err != nil {
		return nil, err
	}
	return api.Relation_builder{From: &in.From, Type: &t, To: &in.To}.Build(), nil
}

// parseEnum returns the value of the enum whose name is prefix + s. The unspecified value is not accepted.
func parseEnum[T ~int32](values map[string]int32, prefix, field, s string) (T, error) {
	v, ok := values[prefix+s]
	if !ok || v == 0 {
		return 0, xerrors.Newf("invalid %s: %q", field, s)
	}
	return T(v), nil
}

func withAgent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		if claims, ok := authproxy.ClaimsFrom(ctx); ok {
			ctx = issue.WithAgent(ctx, claims.Subject)
		}
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}
