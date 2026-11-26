package server

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/codexel/review"
	"go.f110.dev/mono/go/jujutsu"
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

func TestReviewHandlerDiscard(t *testing.T) {
	h := NewReviewHandler(newConfig(t), newReviewStore(t), nil)

	discarded, err := h.Discard(t.Context(), connect.NewRequest(api.RequestDiscard_builder{ReviewId: new("01")}.Build()))
	assertion.MustNoError(t, err)
	assertion.Equal(t, discarded.Msg.GetReview().GetState(), api.ReviewState_REVIEW_STATE_DISCARDED)
	assertion.MustLen(t, discarded.Msg.GetReview().GetComments(), 2)

	_, err = h.Discard(t.Context(), connect.NewRequest(api.RequestDiscard_builder{ReviewId: new("01")}.Build()))
	assertion.Equal(t, connect.CodeOf(err), connect.CodeFailedPrecondition)
	_, err = h.Finish(t.Context(), connect.NewRequest(api.RequestFinish_builder{ReviewId: new("01")}.Build()))
	assertion.Equal(t, connect.CodeOf(err), connect.CodeFailedPrecondition)
	_, err = h.Discard(t.Context(), connect.NewRequest(api.RequestDiscard_builder{ReviewId: new("02")}.Build()))
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

	text, isError = call("add_review_comment", map[string]any{"review_id": "01", "change_id": "kkkk", "path": "a.go", "body": "file"})
	assertion.False(t, isError)
	added := &api.ResponseAddComment{}
	assertion.MustNoError(t, protojson.Unmarshal([]byte(text), added))
	assertion.Equal(t, added.GetComment().GetAuthor(), review.AuthorAgent)
	assertion.Equal(t, added.GetComment().GetPath(), "a.go")
	_, isError = call("resolve_review_comment", map[string]any{"review_id": "01", "comment_id": added.GetComment().GetId()})
	assertion.False(t, isError)

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

func TestGeneratedFiles(t *testing.T) {
	if *jjBinaryPath == "" {
		t.Skipf("Skip %s because -test.jj-binary is not set", t.Name())
	}

	dir, _ := newStackRepository(t, "Cargo.lock")
	files := map[string]string{
		".gitattributes":    "keep.pb.go -linguist-generated\nmarked.txt linguist-generated\n",
		"api.pb.go":         "// Code generated by protoc-gen-go. DO NOT EDIT.\n\npackage api\n",
		"generator.go":      "package main\n\nfunc header() {\n\tw.F(\"// Code generated by kubeproto. DO NOT EDIT.\")\n}\n",
		"keep.pb.go":        "// Code generated by protoc-gen-go. DO NOT EDIT.\n\npackage api\n",
		"main.go":           "package main\n\nfunc main() {}\n",
		"marked.txt":        "text\n",
		"package-lock.json": "{}\n",
	}
	for k, v := range files {
		assertion.MustNoError(t, os.WriteFile(filepath.Join(dir, k), []byte(v), 0644))
	}
	assertion.MustNoError(t, os.Remove(filepath.Join(dir, "Cargo.lock")))
	runCommand(t, dir, "jj", "commit", "--message=generated")
	commit := runCommand(t, dir, "jj", "log", "--no-graph", "--revisions=@-", "--template=commit_id")

	repo := jujutsu.NewRepository(dir)
	diff, err := repo.Diff(t.Context(), commit)
	assertion.MustNoError(t, err)
	generated, err := generatedFiles(t.Context(), repo, commit, diff)
	assertion.MustNoError(t, err)
	assertion.Equal(t, generated, []string{"Cargo.lock", "api.pb.go", "marked.txt", "package-lock.json"})
}

func TestStartReviewAgent(t *testing.T) {
	if *jjBinaryPath == "" {
		t.Skipf("Skip %s because -test.jj-binary is not set", t.Name())
	}

	repo, _ := newStackRepository(t, "a")
	change := runCommand(t, repo, "jj", "log", "--no-graph", "--revisions=@-", "--template=change_id.short()")
	workspaceCreated := `"result":{"type":"workspace_created","workspace":{"workspace_id":"w2"},"tab":{"tab_id":"w2:t1"},"root_pane":{"pane_id":"w2:p1","tab_id":"w2:t1","workspace_id":"w2"}}`
	agentStarted := `"result":{"type":"agent_started","argv":["claude"],"agent":{"agent":"claude","agent_status":"idle","pane_id":"w2:p1","tab_id":"w2:t1","workspace_id":"w2"}}`
	workspaces := `"result":{"type":"workspace_list","workspaces":[{"workspace_id":"w2","label":"repo"}]}`
	c, received := startHerdrWithSequence(t, map[string][]string{
		"workspace.create": {workspaceCreated},
		"agent.start":      {agentStarted},
		"agent.prompt":     {`"result":{"type":"ok"}`},
		"workspace.list":   {workspaces},
	})
	conf := newConfig(t, repo)
	store := review.NewStore(t.TempDir())
	h := NewReviewHandler(conf, store, c)
	agents := NewAgentHandler(c, conf, store, "")
	started, err := h.Start(t.Context(), connect.NewRequest(api.RequestStartReview_builder{Path: new(repo), ChangeIds: []string{change}}.Build()))
	assertion.MustNoError(t, err)
	r := started.Msg.GetReview()
	startAgent := func() (*connect.Response[api.ResponseStartAgent], error) {
		return agents.Start(t.Context(), connect.NewRequest(api.RequestStartAgent_builder{ReviewId: new(r.GetId()), Kind: new("claude")}.Build()))
	}

	_, err = agents.Start(t.Context(), connect.NewRequest(api.RequestStartAgent_builder{ReviewId: new(r.GetId()), Kind: new("claude"), Cwd: new(repo)}.Build()))
	assertion.Equal(t, connect.CodeOf(err), connect.CodeInvalidArgument)

	// The prompt must be at trunk().
	assertion.MustNoError(t, os.MkdirAll(filepath.Join(repo, ".codexel"), 0755))
	assertion.MustNoError(t, os.WriteFile(filepath.Join(repo, ".codexel/review.md"), []byte("Check the error handling.\n"), 0644))
	_, err = startAgent()
	assertion.Equal(t, connect.CodeOf(err), connect.CodeFailedPrecondition)
	assertion.Len(t, received(), 0)

	runCommand(t, repo, "jj", "commit", "--message=prompt")
	runCommand(t, repo, "jj", "bookmark", "set", "master", "--revision=@-")
	runCommand(t, repo, "jj", "git", "push", "--bookmark=master")
	assertion.MustNoError(t, os.WriteFile(filepath.Join(repo, ".codexel/review.md"), []byte("Not committed\n"), 0644))
	res, err := startAgent()
	assertion.MustNoError(t, err)
	assertion.Equal(t, res.Msg.GetAgent().GetPaneId(), "w2:p1")
	reqs := received()
	assertion.Equal(t, methods(reqs), []string{"workspace.create", "agent.start", "agent.prompt", "workspace.list"})
	assertion.Equal(t, reqs[0].Params["cwd"], any(repo))
	prompt := reqs[2].Params["text"].(string)
	assertion.Contains(t, prompt, "Check the error handling.")
	assertion.NotContains(t, prompt, "Not committed")
	assertion.Contains(t, prompt, r.GetId())
	assertion.Contains(t, prompt, "add_review_comment")

	_, err = h.Discard(t.Context(), connect.NewRequest(api.RequestDiscard_builder{ReviewId: new(r.GetId())}.Build()))
	assertion.MustNoError(t, err)
	_, err = startAgent()
	assertion.Equal(t, connect.CodeOf(err), connect.CodeFailedPrecondition)
}

func TestFinishResumesAgent(t *testing.T) {
	if *jjBinaryPath == "" {
		t.Skipf("Skip %s because -test.jj-binary is not set", t.Name())
	}

	repo, _ := newStackRepository(t)
	runCommand(t, repo, "jj", "commit", "--message=exited\n\nAgent: claude\nAgent-Session: s1\n")
	runCommand(t, repo, "jj", "commit", "--message=running\n\nAgent: claude\nAgent-Session: live\n")
	changes := strings.Fields(runCommand(t, repo, "jj", "log", "--no-graph", "--revisions=@--|@-", "--template=change_id.short() ++ \"\\n\""))
	agents := `"result":{"type":"agent_list","agents":[{"agent":"claude","agent_session":{"agent":"claude","kind":"id","source":"herdr:claude","value":"live"},"agent_status":"idle","cwd":"/other","pane_id":"w1:p1","tab_id":"w1:t1","workspace_id":"w1"}]}`
	workspaceCreated := `"result":{"type":"workspace_created","workspace":{"workspace_id":"w2"},"tab":{"tab_id":"w2:t1"},"root_pane":{"pane_id":"w2:p1","tab_id":"w2:t1","workspace_id":"w2"}}`
	agentStarted := `"result":{"type":"agent_started","argv":["claude","--resume","s1"],"agent":{"agent":"claude","agent_status":"idle","pane_id":"w2:p1","tab_id":"w2:t1","workspace_id":"w2"}}`
	c, received := startHerdrWithSequence(t, map[string][]string{
		"agent.list":       {agents},
		"agent.prompt":     {`"result":{"type":"ok"}`},
		"workspace.create": {workspaceCreated},
		"agent.start":      {agentStarted, `"error":{"code":"agent_start_failed","message":"failed"}`},
		"workspace.close":  {`"result":{"type":"ok"}`},
		"workspace.list":   {`"result":{"type":"workspace_list","workspaces":[{"workspace_id":"w1","label":"other"},{"workspace_id":"w2","label":"repo"}]}`},
	})
	h := NewReviewHandler(newConfig(t, repo), review.NewStore(t.TempDir()), c)
	finish := func(changeIDs ...string) (*api.Review, *connect.Response[api.ResponseFinish], error) {
		started, err := h.Start(t.Context(), connect.NewRequest(api.RequestStartReview_builder{Path: new(repo), ChangeIds: changeIDs}.Build()))
		assertion.MustNoError(t, err)
		id := started.Msg.GetReview().GetId()
		_, err = h.AddComment(t.Context(), connect.NewRequest(api.RequestAddComment_builder{ReviewId: new(id), Body: new("fix")}.Build()))
		assertion.MustNoError(t, err)
		res, err := h.Finish(t.Context(), connect.NewRequest(api.RequestFinish_builder{ReviewId: new(id)}.Build()))
		got, getErr := h.Get(t.Context(), connect.NewRequest(api.RequestGetReview_builder{Id: new(id)}.Build()))
		assertion.MustNoError(t, getErr)
		return got.Msg.GetReview(), res, err
	}

	// The running agent is notified and the session of the exited agent is resumed in the new workspace of the repository.
	r, res, err := finish(changes...)
	assertion.MustNoError(t, err)
	assertion.Equal(t, r.GetState(), api.ReviewState_REVIEW_STATE_FINISHED)
	reqs := received()
	assertion.Equal(t, methods(reqs), []string{"agent.list", "agent.prompt", "workspace.create", "agent.start", "agent.prompt", "workspace.list"})
	assertion.Equal(t, reqs[1].Params["target"], any("w1:p1"))
	assertion.Equal(t, reqs[2].Params["cwd"], any(repo))
	assertion.Equal(t, reqs[3].Params["kind"], any("claude"))
	assertion.Equal(t, reqs[3].Params["args"], any([]any{"--resume", "s1"}))
	assertion.Equal(t, reqs[4].Params["target"], any("w2:p1"))
	assertion.Contains(t, reqs[4].Params["text"].(string), r.GetId())
	assertion.MustLen(t, res.Msg.GetNotifiedAgents(), 2)
	assertion.Equal(t, res.Msg.GetNotifiedAgents()[1].GetWorkspaceLabel(), "repo")

	// The review stays open and the workspace is closed if the agent fails to start.
	r, _, err = finish(changes[1])
	assertion.Equal(t, connect.CodeOf(err), connect.CodeUnknown)
	assertion.Equal(t, r.GetState(), api.ReviewState_REVIEW_STATE_OPEN)
	assertion.Equal(t, methods(received()[len(reqs):]), []string{"agent.list", "workspace.create", "agent.start", "workspace.close"})
}

func TestListUnreviewedCommits(t *testing.T) {
	if *jjBinaryPath == "" {
		t.Skipf("Skip %s because -test.jj-binary is not set", t.Name())
	}

	repo, _ := newStackRepository(t)
	runCommand(t, repo, "jj", "commit", "--message=approved\n\nAgent: claude\nAgent-Session: s1\n")
	runCommand(t, repo, "jj", "commit", "--message=unreviewed\n\nAgent: claude\nAgent-Session: s1\n")
	runCommand(t, repo, "jj", "commit", "--message=human")
	runCommand(t, repo, "jj", "bookmark", "set", "master", "--revision=@-")
	runCommand(t, repo, "jj", "bookmark", "create", "feature", "--revision=@-")
	approved := runCommand(t, repo, "jj", "log", "--no-graph", "--revisions=@---", "--template=change_id.short()")
	conf := newConfig(t, repo)
	h := NewReviewHandler(conf, review.NewStore(t.TempDir()), nil)
	started, err := h.Start(t.Context(), connect.NewRequest(api.RequestStartReview_builder{Path: new(repo), ChangeIds: []string{approved}}.Build()))
	assertion.MustNoError(t, err)
	_, err = h.Finish(t.Context(), connect.NewRequest(api.RequestFinish_builder{ReviewId: new(started.Msg.GetReview().GetId())}.Build()))
	assertion.MustNoError(t, err)

	list := func(bookmarks ...string) *api.ResponseListUnreviewedCommits {
		res, err := h.ListUnreviewedCommits(t.Context(), connect.NewRequest(api.RequestListUnreviewedCommits_builder{Path: new(repo), Bookmarks: bookmarks}.Build()))
		assertion.MustNoError(t, err)
		return res.Msg
	}
	res := list("feature", "master")
	assertion.Equal(t, res.GetDefaultBranch(), "master")
	assertion.MustLen(t, res.GetCommits(), 1)
	assertion.Equal(t, res.GetCommits()[0].GetDescription(), "unreviewed")
	// The commits pushed to the other bookmarks may be reviewed on the remote.
	assertion.Len(t, list("feature").GetCommits(), 0)

	runCommand(t, repo, "jj", "git", "push", "--bookmark=master")
	assertion.Len(t, list("master").GetCommits(), 0)
}
