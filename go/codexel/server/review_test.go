package server

import (
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/codexel/review"
	"go.f110.dev/mono/go/testing/assertion"
)

func newReviewStore(t *testing.T) *review.Store {
	t.Helper()

	now := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	s := review.NewStore(t.TempDir())
	err := s.Create(&review.Review{
		ID:         "01",
		Repository: "/repo",
		Changes:    []string{"kkkk"},
		State:      review.StateOpen,
		Rounds:     []*review.Round{{Commits: map[string]string{"kkkk": "c1"}, CreatedAt: now}},
		Comments: []*review.Comment{
			{ID: "a", ChangeID: "kkkk", Path: "a.go", Side: review.SideNew, StartLine: 1, EndLine: 1, Anchor: "package a", Round: 1, Body: "open", Author: review.AuthorUser},
			{ID: "b", Round: 1, Body: "resolved", Author: review.AuthorUser, Resolved: true},
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	assertion.MustNoError(t, err)
	return s
}

func TestReviewHandler(t *testing.T) {
	h := NewReviewHandler(newConfig(t), newReviewStore(t), nil)

	added, err := h.AddComment(t.Context(), connect.NewRequest(api.RequestAddComment_builder{ReviewId: new("01"), Body: new("overall")}.Build()))
	assertion.MustNoError(t, err)
	assertion.Equal(t, added.Msg.GetComment().GetAuthor(), review.AuthorUser)
	assertion.Equal(t, added.Msg.GetComment().GetRound(), int32(1))

	_, err = h.AddComment(t.Context(), connect.NewRequest(api.RequestAddComment_builder{ReviewId: new("01"), Body: new("")}.Build()))
	assertion.Equal(t, connect.CodeOf(err), connect.CodeInvalidArgument)
	_, err = h.Get(t.Context(), connect.NewRequest(api.RequestGetReview_builder{Id: new("02")}.Build()))
	assertion.Equal(t, connect.CodeOf(err), connect.CodeNotFound)
	// The lines are read from the repository which must be opened.
	_, err = h.AddComment(t.Context(), connect.NewRequest(api.RequestAddComment_builder{
		ReviewId: new("01"), ChangeId: new("kkkk"), Path: new("a.go"), Side: new(api.DiffSide_DIFF_SIDE_NEW), StartLine: new(int32(1)), EndLine: new(int32(1)), Body: new("line"),
	}.Build()))
	assertion.Equal(t, connect.CodeOf(err), connect.CodeNotFound)

	viewed, err := h.SetViewed(t.Context(), connect.NewRequest(api.RequestSetViewed_builder{ReviewId: new("01"), ChangeId: new("kkkk"), Path: new("a.go"), Viewed: new(true)}.Build()))
	assertion.MustNoError(t, err)
	assertion.MustLen(t, viewed.Msg.GetReview().GetViewed(), 1)
	assertion.Equal(t, viewed.Msg.GetReview().GetViewed()[0].GetPath(), "a.go")
	assertion.True(t, viewed.Msg.GetReview().GetViewed()[0].GetViewed())
	_, err = h.SetViewed(t.Context(), connect.NewRequest(api.RequestSetViewed_builder{ReviewId: new("01"), ChangeId: new("mmmm"), Path: new("a.go"), Viewed: new(true)}.Build()))
	assertion.Equal(t, connect.CodeOf(err), connect.CodeNotFound)

	// Finish approves the review without notifying the agents if all comments are resolved.
	for _, id := range []string{"a", added.Msg.GetComment().GetId()} {
		_, err = h.ResolveComment(t.Context(), connect.NewRequest(api.RequestResolveComment_builder{ReviewId: new("01"), CommentId: new(id), Resolved: new(true)}.Build()))
		assertion.MustNoError(t, err)
	}
	approved, err := h.Finish(t.Context(), connect.NewRequest(api.RequestFinish_builder{ReviewId: new("01")}.Build()))
	assertion.MustNoError(t, err)
	assertion.Equal(t, approved.Msg.GetReview().GetState(), api.ReviewState_REVIEW_STATE_APPROVED)
	assertion.Len(t, approved.Msg.GetNotifiedAgents(), 0)
	_, err = h.Finish(t.Context(), connect.NewRequest(api.RequestFinish_builder{ReviewId: new("01")}.Build()))
	assertion.Equal(t, connect.CodeOf(err), connect.CodeFailedPrecondition)

	res, err := h.List(t.Context(), connect.NewRequest(api.RequestListReviews_builder{Path: new("/repo")}.Build()))
	assertion.MustNoError(t, err)
	assertion.MustLen(t, res.Msg.GetReviews(), 1)
	assertion.Equal(t, res.Msg.GetReviews()[0].GetRounds()[0].GetCommits()[0].GetCommitId(), "c1")
	assertion.MustLen(t, res.Msg.GetReviews()[0].GetComments(), 3)

	// The commits are read from the repository which must be opened.
	_, err = h.ListApprovedCommits(t.Context(), connect.NewRequest(api.RequestListApprovedCommits_builder{Path: new("/repo")}.Build()))
	assertion.Equal(t, connect.CodeOf(err), connect.CodeNotFound)
}

func TestReviewMCP(t *testing.T) {
	h := NewReviewHandler(newConfig(t), newReviewStore(t), nil)
	srv := httptest.NewServer(NewMCPHandler(h))
	t.Cleanup(srv.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0.0.0"}, nil)
	session, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{Endpoint: srv.URL, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
	assertion.MustNoError(t, err)
	t.Cleanup(func() { session.Close() })
	call := func(name string, args map[string]any) (string, bool) {
		t.Helper()
		res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
		assertion.MustNoError(t, err)
		return res.Content[0].(*mcp.TextContent).Text, res.IsError
	}

	text, isError := call("list_review_comments", map[string]any{"review_id": "01"})
	assertion.False(t, isError)
	listed := &api.ResponseGetReview{}
	assertion.MustNoError(t, protojson.Unmarshal([]byte(text), listed))
	assertion.MustLen(t, listed.GetReview().GetComments(), 1)
	assertion.Equal(t, listed.GetReview().GetComments()[0].GetId(), "a")

	text, isError = call("reply_review_comment", map[string]any{"review_id": "01", "comment_id": "a", "body": "fixed"})
	assertion.False(t, isError)
	replied := &api.ResponseReply{}
	assertion.MustNoError(t, protojson.Unmarshal([]byte(text), replied))
	assertion.Equal(t, replied.GetComment().GetReplies()[0].GetAuthor(), review.AuthorAgent)

	_, isError = call("resolve_review_comment", map[string]any{"review_id": "01", "comment_id": "a"})
	assertion.False(t, isError)
	text, _ = call("list_review_comments", map[string]any{"review_id": "01"})
	assertion.MustNoError(t, protojson.Unmarshal([]byte(text), listed))
	assertion.MustLen(t, listed.GetReview().GetComments(), 0)

	// The repository of the review is not opened.
	text, isError = call("start_next_round", map[string]any{"review_id": "01"})
	assertion.True(t, isError)
	assertion.Contains(t, text, "is not opened")
}
