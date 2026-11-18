package server

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.f110.dev/xerrors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/codexel/review"
)

type listReviewsInput struct {
	Path string `json:"path,omitempty" jsonschema:"Root of the workspace of the repository. Omit to list the reviews of all repositories"`
}

type listReviewCommentsInput struct {
	ReviewID        string `json:"review_id" jsonschema:"ID of the review"`
	IncludeResolved bool   `json:"include_resolved,omitempty" jsonschema:"Include the resolved comments"`
}

type replyInput struct {
	ReviewID  string `json:"review_id" jsonschema:"ID of the review"`
	CommentID string `json:"comment_id" jsonschema:"ID of the comment"`
	Body      string `json:"body" jsonschema:"Body of the reply written in Markdown"`
}

type resolveInput struct {
	ReviewID  string `json:"review_id" jsonschema:"ID of the review"`
	CommentID string `json:"comment_id" jsonschema:"ID of the comment"`
}

type reviewInput struct {
	ReviewID string `json:"review_id" jsonschema:"ID of the review"`
}

// NewMCPHandler returns the HTTP handler which serves MCP for the coding agents. The operations are done by the agent.
func NewMCPHandler(h *ReviewHandler) http.Handler {
	s := mcp.NewServer(&mcp.Implementation{Name: "codexel", Version: "v0.1.0"}, nil)
	addTool(s, "list_reviews", "List the code reviews in Codexel. The comments are included", h.ListReviews, func(in listReviewsInput) (*api.RequestListReviews, error) {
		return api.RequestListReviews_builder{Path: &in.Path}.Build(), nil
	})
	addTool(s, "list_review_comments", "List the unresolved comments of the code review. change_id, path, side, start_line and end_line are the target. "+
		"The target is the whole review if change_id is empty, the commit message of the change if path is empty, and the file otherwise. The target is the lines "+
		"of it if start_line is not 0. The lines are the lines of the file or the commit message at the commit of the change (side NEW) or the file at its parent "+
		"(side OLD) in the latest round. anchor is the content of the lines when the comment was written. "+
		"drifted is true if the anchor is not found", h.listComments, func(in listReviewCommentsInput) (*listCommentsRequest, error) {
		return &listCommentsRequest{reviewID: in.ReviewID, includeResolved: in.IncludeResolved}, nil
	})
	addTool(s, "reply_review_comment", "Reply to the comment of the code review", h.Reply, func(in replyInput) (*api.RequestReply, error) {
		return api.RequestReply_builder{ReviewId: &in.ReviewID, CommentId: &in.CommentID, Body: &in.Body}.Build(), nil
	})
	addTool(s, "resolve_review_comment", "Resolve the comment of the code review", h.ResolveComment, func(in resolveInput) (*api.RequestResolveComment, error) {
		return api.RequestResolveComment_builder{ReviewId: &in.ReviewID, CommentId: &in.CommentID, Resolved: new(true)}.Build(), nil
	})
	addTool(s, "start_next_round", "Start the next round of the code review after addressing the comments. The current commits of the changes are recorded "+
		"and the comments are moved to the lines of the anchors", h.StartNextRound, func(in reviewInput) (*api.RequestStartNextRound, error) {
		return api.RequestStartNextRound_builder{ReviewId: &in.ReviewID}.Build(), nil
	})

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, &mcp.StreamableHTTPOptions{Stateless: true})
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		handler.ServeHTTP(w, req.WithContext(withAuthor(req.Context(), review.AuthorAgent)))
	})
}

type listCommentsRequest struct {
	reviewID        string
	includeResolved bool
}

// listComments returns the comments of the review. The resolved comments are excluded unless includeResolved.
func (h *ReviewHandler) listComments(ctx context.Context, req *connect.Request[listCommentsRequest]) (*connect.Response[api.ResponseGetReview], error) {
	res, err := h.GetReview(ctx, connect.NewRequest(api.RequestGetReview_builder{Id: &req.Msg.reviewID}.Build()))
	if err != nil {
		return nil, err
	}
	r := res.Msg.GetReview()
	if !req.Msg.includeResolved {
		var comments []*api.ReviewComment
		for _, v := range r.GetComments() {
			if !v.GetResolved() {
				comments = append(comments, v)
			}
		}
		r.SetComments(comments)
	}
	return res, nil
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
