package server

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/codexel/config"
	"go.f110.dev/mono/go/codexel/review"
	"go.f110.dev/mono/go/gitattributes"
	"go.f110.dev/mono/go/herdr"
	"go.f110.dev/mono/go/jujutsu"
)

type ReviewHandler struct {
	config    *config.File
	store     *review.Store
	herdr     *herdr.Client
	approvals *review.Approvals
	now       func() time.Time
}

var _ api.ReviewServiceHandler = (*ReviewHandler)(nil)

func NewReviewHandler(conf *config.File, store *review.Store, herdrClient *herdr.Client) *ReviewHandler {
	return &ReviewHandler{config: conf, store: store, herdr: herdrClient, approvals: review.NewApprovals(), now: time.Now}
}

type authorKey struct{}

// withAuthor returns the context whose operations are done by the author. The default author is the user.
func withAuthor(ctx context.Context, author string) context.Context {
	return context.WithValue(ctx, authorKey{}, author)
}

func authorFrom(ctx context.Context) string {
	if v, ok := ctx.Value(authorKey{}).(string); ok {
		return v
	}
	return review.AuthorUser
}

func (h *ReviewHandler) ListReviews(_ context.Context, req *connect.Request[api.RequestListReviews]) (*connect.Response[api.ResponseListReviews], error) {
	reviews, err := h.store.List(req.Msg.GetPath())
	if err != nil {
		return nil, toReviewError(err)
	}
	res := make([]*api.Review, 0, len(reviews))
	for _, v := range reviews {
		res = append(res, toReview(v))
	}
	return connect.NewResponse(api.ResponseListReviews_builder{Reviews: res}.Build()), nil
}

func (h *ReviewHandler) ListApprovedCommits(ctx context.Context, req *connect.Request[api.RequestListApprovedCommits]) (*connect.Response[api.ResponseListApprovedCommits], error) {
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	reviews, err := h.store.List(repo.Root())
	if err != nil {
		return nil, toReviewError(err)
	}
	commits, err := h.approvals.Commits(ctx, repo, reviews)
	if err != nil {
		return nil, toReviewError(err)
	}
	return connect.NewResponse(api.ResponseListApprovedCommits_builder{CommitIds: commits}.Build()), nil
}

func (h *ReviewHandler) GetReview(_ context.Context, req *connect.Request[api.RequestGetReview]) (*connect.Response[api.ResponseGetReview], error) {
	r, err := h.store.Get(req.Msg.GetId())
	if err != nil {
		return nil, toReviewError(err)
	}
	return connect.NewResponse(api.ResponseGetReview_builder{Review: toReview(r)}.Build()), nil
}

func (h *ReviewHandler) StartReview(ctx context.Context, req *connect.Request[api.RequestStartReview]) (*connect.Response[api.ResponseStartReview], error) {
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	conf, err := h.config.Load()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	r, err := review.New(ctx, repo, req.Msg.GetChangeIds(), conf.AutoViewedPatterns, h.now())
	if err != nil {
		return nil, toReviewError(err)
	}
	if err := h.store.Create(r); err != nil {
		return nil, toReviewError(err)
	}
	return connect.NewResponse(api.ResponseStartReview_builder{Review: toReview(r)}.Build()), nil
}

func (h *ReviewHandler) AddComment(ctx context.Context, req *connect.Request[api.RequestAddComment]) (*connect.Response[api.ResponseAddComment], error) {
	var c *review.Comment
	_, err := h.store.Update(req.Msg.GetReviewId(), func(r *review.Review) error {
		var repo review.Repository
		if req.Msg.GetStartLine() != 0 {
			v, err := openedRepository(h.config, r.Repository)
			if err != nil {
				return err
			}
			repo = v
		}
		var err error
		c, err = r.AddComment(ctx, repo, req.Msg.GetChangeId(), req.Msg.GetPath(), toSide(req.Msg.GetSide()), int(req.Msg.GetStartLine()),
			int(req.Msg.GetEndLine()), req.Msg.GetBody(), authorFrom(ctx), h.now())
		return err
	})
	if err != nil {
		return nil, toReviewError(err)
	}
	return connect.NewResponse(api.ResponseAddComment_builder{Comment: toReviewComment(c)}.Build()), nil
}

func (h *ReviewHandler) EditComment(_ context.Context, req *connect.Request[api.RequestEditComment]) (*connect.Response[api.ResponseEditComment], error) {
	var c *review.Comment
	_, err := h.store.Update(req.Msg.GetReviewId(), func(r *review.Review) error {
		var err error
		c, err = r.EditComment(req.Msg.GetCommentId(), req.Msg.GetBody(), h.now())
		return err
	})
	if err != nil {
		return nil, toReviewError(err)
	}
	return connect.NewResponse(api.ResponseEditComment_builder{Comment: toReviewComment(c)}.Build()), nil
}

func (h *ReviewHandler) DeleteComment(_ context.Context, req *connect.Request[api.RequestDeleteComment]) (*connect.Response[api.ResponseDeleteComment], error) {
	_, err := h.store.Update(req.Msg.GetReviewId(), func(r *review.Review) error {
		return r.DeleteComment(req.Msg.GetCommentId(), h.now())
	})
	if err != nil {
		return nil, toReviewError(err)
	}
	return connect.NewResponse(&api.ResponseDeleteComment{}), nil
}

func (h *ReviewHandler) Reply(ctx context.Context, req *connect.Request[api.RequestReply]) (*connect.Response[api.ResponseReply], error) {
	var c *review.Comment
	_, err := h.store.Update(req.Msg.GetReviewId(), func(r *review.Review) error {
		var err error
		c, err = r.Reply(req.Msg.GetCommentId(), req.Msg.GetBody(), authorFrom(ctx), h.now())
		return err
	})
	if err != nil {
		return nil, toReviewError(err)
	}
	return connect.NewResponse(api.ResponseReply_builder{Comment: toReviewComment(c)}.Build()), nil
}

func (h *ReviewHandler) ResolveComment(_ context.Context, req *connect.Request[api.RequestResolveComment]) (*connect.Response[api.ResponseResolveComment], error) {
	var c *review.Comment
	_, err := h.store.Update(req.Msg.GetReviewId(), func(r *review.Review) error {
		var err error
		c, err = r.Resolve(req.Msg.GetCommentId(), req.Msg.GetResolved(), h.now())
		return err
	})
	if err != nil {
		return nil, toReviewError(err)
	}
	return connect.NewResponse(api.ResponseResolveComment_builder{Comment: toReviewComment(c)}.Build()), nil
}

// Finish tells the agents before finishing the review so that the user can retry if herdr fails.
func (h *ReviewHandler) Finish(ctx context.Context, req *connect.Request[api.RequestFinish]) (*connect.Response[api.ResponseFinish], error) {
	r, err := h.store.Get(req.Msg.GetReviewId())
	if err != nil {
		return nil, toReviewError(err)
	}
	if r.State != review.StateOpen {
		return nil, toReviewError(xerrors.WithMessagef(review.ErrInvalidState, "the review is %s", r.State))
	}
	repo, err := openedRepository(h.config, r.Repository)
	if err != nil {
		return nil, err
	}
	agents, err := h.notifyAgents(ctx, repo, r)
	if err != nil {
		return nil, err
	}
	r, err = h.store.Update(r.ID, func(r *review.Review) error { return r.Finish(h.now()) })
	if err != nil {
		return nil, toReviewError(err)
	}
	return connect.NewResponse(api.ResponseFinish_builder{Review: toReview(r), NotifiedAgents: agents}.Build()), nil
}

// notifyAgents prompts the running agents which created the changes of the latest round. The agents are found by the
// Agent and Agent-Session trailers.
func (h *ReviewHandler) notifyAgents(ctx context.Context, repo *jujutsu.Repository, r *review.Review) ([]*api.Agent, error) {
	commits := make([]string, 0, len(r.Changes))
	for _, v := range r.Changes {
		if commit, ok := r.Commit(r.LatestRound(), v); ok {
			commits = append(commits, commit)
		}
	}
	if len(commits) == 0 {
		return nil, nil
	}
	rows, err := repo.Log(ctx, strings.Join(commits, "|"), 0)
	if err != nil {
		return nil, toJujutsuError(err)
	}
	agents, err := h.herdr.ListAgents(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	workspaces, err := workspaceLabels(ctx, h.herdr)
	if err != nil {
		return nil, err
	}

	var notified []*api.Agent
	for _, v := range agents {
		a := toAgent(v, workspaces)
		if a.GetSessionId() == "" || !slices.ContainsFunc(rows, func(row *jujutsu.LogRow) bool { return row.Commit.CreatedBy(a.GetKind(), a.GetSessionId()) }) {
			continue
		}
		if err := h.herdr.PromptAgent(ctx, a.GetPaneId(), finishedPrompt(r.ID)); err != nil {
			return nil, toConnectError(err)
		}
		notified = append(notified, a)
	}
	return notified, nil
}

// finishedPrompt is the prompt to the agent when the review is finished.
func finishedPrompt(id string) string {
	return "The code review " + id + " in Codexel is finished. Read the unresolved comments by the list_review_comments tool of the codexel MCP server. " +
		"Address each comment, and reply to it or resolve it. Then call the start_next_round tool."
}

func (h *ReviewHandler) Approve(_ context.Context, req *connect.Request[api.RequestApprove]) (*connect.Response[api.ResponseApprove], error) {
	r, err := h.store.Update(req.Msg.GetReviewId(), func(r *review.Review) error { return r.Approve(h.now()) })
	if err != nil {
		return nil, toReviewError(err)
	}
	return connect.NewResponse(api.ResponseApprove_builder{Review: toReview(r)}.Build()), nil
}

func (h *ReviewHandler) StartNextRound(ctx context.Context, req *connect.Request[api.RequestStartNextRound]) (*connect.Response[api.ResponseStartNextRound], error) {
	conf, err := h.config.Load()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	r, err := h.store.Update(req.Msg.GetReviewId(), func(r *review.Review) error {
		repo, err := openedRepository(h.config, r.Repository)
		if err != nil {
			return err
		}
		return r.NextRound(ctx, repo, conf.AutoViewedPatterns, h.now())
	})
	if err != nil {
		return nil, toReviewError(err)
	}
	return connect.NewResponse(api.ResponseStartNextRound_builder{Review: toReview(r)}.Build()), nil
}

func (h *ReviewHandler) SetViewed(_ context.Context, req *connect.Request[api.RequestSetViewed]) (*connect.Response[api.ResponseSetViewed], error) {
	r, err := h.store.Update(req.Msg.GetReviewId(), func(r *review.Review) error {
		return r.SetViewed(req.Msg.GetChangeId(), req.Msg.GetPath(), req.Msg.GetViewed(), h.now())
	})
	if err != nil {
		return nil, toReviewError(err)
	}
	return connect.NewResponse(api.ResponseSetViewed_builder{Review: toReview(r)}.Build()), nil
}

func (h *ReviewHandler) RoundDiff(ctx context.Context, req *connect.Request[api.RequestRoundDiff]) (*connect.Response[api.ResponseRoundDiff], error) {
	r, err := h.store.Get(req.Msg.GetReviewId())
	if err != nil {
		return nil, toReviewError(err)
	}
	to := int(req.Msg.GetToRound())
	if to == 0 {
		to = r.LatestRound()
	}
	from := int(req.Msg.GetFromRound())
	if from < 0 || from >= to {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("from_round must be less than to_round"))
	}
	commit, ok := r.Commit(to, req.Msg.GetChangeId())
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("the change is not found in the round"))
	}
	repo, err := openedRepository(h.config, r.Repository)
	if err != nil {
		return nil, err
	}
	c, err := repo.Show(ctx, commit)
	if err != nil {
		return nil, toJujutsuError(err)
	}
	var files []*jujutsu.FileDiff
	fromDescription := c.FullDescription
	if from == 0 {
		files, err = repo.Diff(ctx, commit)
	} else {
		fromCommit, ok := r.Commit(from, req.Msg.GetChangeId())
		if !ok {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("the change is not found in from_round"))
		}
		files, err = repo.DiffRange(ctx, fromCommit, commit)
		if err == nil {
			var fc *jujutsu.Commit
			fc, err = repo.Show(ctx, fromCommit)
			if err == nil {
				fromDescription = fc.FullDescription
			}
		}
	}
	if err != nil {
		return nil, toJujutsuError(err)
	}
	generated, err := generatedFiles(ctx, repo, commit, files)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseRoundDiff_builder{
		Commit:    toCommit(c),
		Files:     toFileDiffs(files),
		Generated: generated,
		Message:   toFileDiff(review.MessageDiff(fromDescription, c.FullDescription)),
	}.Build()), nil
}

// generatedFiles returns the paths of the files marked as linguist-generated by .gitattributes at the root of the
// commit. Only the top-level .gitattributes is read.
func generatedFiles(ctx context.Context, repo *jujutsu.Repository, commit string, files []*jujutsu.FileDiff) ([]string, error) {
	content, err := repo.File(ctx, commit, ".gitattributes")
	if _, ok := errors.AsType[*jujutsu.Error](err); ok {
		return nil, nil
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	attrs := gitattributes.Parse(content)
	var generated []string
	for _, v := range files {
		if attrs.Generated(v.Path) {
			generated = append(generated, v.Path)
		}
	}
	return generated, nil
}

// toReviewError converts the error of the review. The errors of Connect and jj are returned as they are.
func toReviewError(err error) error {
	if _, ok := errors.AsType[*connect.Error](err); ok {
		return err
	}
	switch {
	case errors.Is(err, review.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, review.ErrInvalidArgument):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, review.ErrInvalidState):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return toJujutsuError(err)
}

var reviewStates = map[review.State]api.ReviewState{
	review.StateOpen:     api.ReviewState_REVIEW_STATE_OPEN,
	review.StateFinished: api.ReviewState_REVIEW_STATE_FINISHED,
	review.StateApproved: api.ReviewState_REVIEW_STATE_APPROVED,
}

var diffSides = map[review.Side]api.DiffSide{
	review.SideNew: api.DiffSide_DIFF_SIDE_NEW,
	review.SideOld: api.DiffSide_DIFF_SIDE_OLD,
}

func toSide(s api.DiffSide) review.Side {
	for k, v := range diffSides {
		if v == s {
			return k
		}
	}
	return ""
}

func toReview(r *review.Review) *api.Review {
	rounds := make([]*api.ReviewRound, 0, len(r.Rounds))
	for i, v := range r.Rounds {
		commits := make([]*api.RoundCommit, 0, len(v.Commits))
		for _, change := range r.Changes {
			if commit, ok := v.Commits[change]; ok {
				commits = append(commits, api.RoundCommit_builder{ChangeId: new(change), CommitId: new(commit)}.Build())
			}
		}
		rounds = append(rounds, api.ReviewRound_builder{Number: new(int32(i + 1)), Commits: commits, CreatedAt: new(v.CreatedAt.Unix())}.Build())
	}
	comments := make([]*api.ReviewComment, 0, len(r.Comments))
	for _, v := range r.Comments {
		comments = append(comments, toReviewComment(v))
	}
	viewed := make([]*api.ViewedFile, 0, len(r.Viewed))
	for _, v := range r.Viewed {
		viewed = append(viewed, api.ViewedFile_builder{ChangeId: new(v.ChangeID), Path: new(v.Path), Viewed: new(v.Viewed)}.Build())
	}
	return api.Review_builder{
		Id:         new(r.ID),
		Repository: new(r.Repository),
		ChangeIds:  r.Changes,
		State:      new(reviewStates[r.State]),
		Rounds:     rounds,
		Comments:   comments,
		Viewed:     viewed,
		CreatedAt:  new(r.CreatedAt.Unix()),
		UpdatedAt:  new(r.UpdatedAt.Unix()),
	}.Build()
}

func toReviewComment(c *review.Comment) *api.ReviewComment {
	replies := make([]*api.ReviewReply, 0, len(c.Replies))
	for _, v := range c.Replies {
		replies = append(replies, api.ReviewReply_builder{Id: new(v.ID), Body: new(v.Body), Author: new(v.Author), CreatedAt: new(v.CreatedAt.Unix())}.Build())
	}
	return api.ReviewComment_builder{
		Id:        new(c.ID),
		ChangeId:  new(c.ChangeID),
		Path:      new(c.Path),
		Side:      new(diffSides[c.Side]),
		StartLine: new(int32(c.StartLine)),
		EndLine:   new(int32(c.EndLine)),
		Anchor:    new(c.Anchor),
		Drifted:   new(c.Drifted),
		Round:     new(int32(c.Round)),
		Body:      new(c.Body),
		Author:    new(c.Author),
		Resolved:  new(c.Resolved),
		Replies:   replies,
		CreatedAt: new(c.CreatedAt.Unix()),
		UpdatedAt: new(c.UpdatedAt.Unix()),
	}.Build()
}
