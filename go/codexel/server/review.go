package server

import (
	"context"
	"errors"
	"log/slog"
	"path"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/go-enry/go-enry/v2"
	"go.f110.dev/xerrors"
	"golang.org/x/sync/errgroup"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/codexel/config"
	"go.f110.dev/mono/go/codexel/review"
	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/herdr"
	"go.f110.dev/mono/go/jujutsu"
	"go.f110.dev/mono/go/logger"
)

const (
	// reviewPromptPath is the path of the prompt of the agent which reviews the changes. It is read at reviewPromptRevision.
	reviewPromptPath     = ".codexel/review.md"
	reviewPromptRevision = "trunk()"
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

func (h *ReviewHandler) List(_ context.Context, req *connect.Request[api.RequestListReviews]) (*connect.Response[api.ResponseListReviews], error) {
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

func (h *ReviewHandler) ListUnreviewedCommits(ctx context.Context, req *connect.Request[api.RequestListUnreviewedCommits]) (*connect.Response[api.ResponseListUnreviewedCommits], error) {
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	trunk, err := repo.Log(ctx, "trunk()", 1)
	if err != nil {
		return nil, toJujutsuError(err)
	}
	if len(trunk) == 0 || len(trunk[0].Commit.RemoteBookmarks) == 0 {
		return connect.NewResponse(&api.ResponseListUnreviewedCommits{}), nil
	}
	defaultBranch := trunk[0].Commit.RemoteBookmarks[0].Name
	res := api.ResponseListUnreviewedCommits_builder{DefaultBranch: new(defaultBranch)}
	if !slices.Contains(req.Msg.GetBookmarks(), defaultBranch) {
		return connect.NewResponse(res.Build()), nil
	}
	bookmarks, err := repo.Bookmarks(ctx)
	if err != nil {
		return nil, toJujutsuError(err)
	}
	i := slices.IndexFunc(bookmarks, func(b *jujutsu.Bookmark) bool { return b.Remote == "" && b.Name == defaultBranch })
	if i == -1 || bookmarks[i].Target == "" {
		return connect.NewResponse(res.Build()), nil
	}
	rows, err := repo.Log(ctx, "trunk().."+bookmarks[i].Target, 0)
	if err != nil {
		return nil, toJujutsuError(err)
	}
	reviews, err := h.store.List(repo.Root())
	if err != nil {
		return nil, toReviewError(err)
	}
	approved, err := h.approvals.Commits(ctx, repo, reviews)
	if err != nil {
		return nil, toReviewError(err)
	}
	for _, row := range rows {
		if row.Commit.CreatedByAgent() && !slices.Contains(approved, row.Commit.CommitID) {
			res.Commits = append(res.Commits, toCommit(row.Commit))
		}
	}
	return connect.NewResponse(res.Build()), nil
}

func (h *ReviewHandler) Get(_ context.Context, req *connect.Request[api.RequestGetReview]) (*connect.Response[api.ResponseGetReview], error) {
	r, err := h.store.Get(req.Msg.GetId())
	if err != nil {
		return nil, toReviewError(err)
	}
	return connect.NewResponse(api.ResponseGetReview_builder{Review: toReview(r)}.Build()), nil
}

func (h *ReviewHandler) Start(ctx context.Context, req *connect.Request[api.RequestStartReview]) (*connect.Response[api.ResponseStartReview], error) {
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

// Finish tells the agents before finishing the review so that the user can retry if herdr fails. The agents are not told
// if the review is approved because all comments are resolved.
func (h *ReviewHandler) Finish(ctx context.Context, req *connect.Request[api.RequestFinish]) (*connect.Response[api.ResponseFinish], error) {
	r, err := h.store.Get(req.Msg.GetReviewId())
	if err != nil {
		return nil, toReviewError(err)
	}
	if r.State != review.StateOpen {
		return nil, toReviewError(xerrors.WithMessagef(review.ErrInvalidState, "the review is %s", r.State))
	}
	var agents []*api.Agent
	if r.Unresolved() {
		repo, err := openedRepository(h.config, r.Repository)
		if err != nil {
			return nil, err
		}
		agents, err = h.notifyAgents(ctx, repo, r)
		if err != nil {
			return nil, err
		}
	}
	r, err = h.store.Update(r.ID, func(r *review.Review) error { return r.Finish(h.now()) })
	if err != nil {
		return nil, toReviewError(err)
	}
	return connect.NewResponse(api.ResponseFinish_builder{Review: toReview(r), NotifiedAgents: agents}.Build()), nil
}

// notifyAgents prompts the agents which created the changes of the latest round. The agents are found by the Agent and
// Agent-Session trailers. The session of the agent which is not running is resumed in the new workspace of herdr whose
// working directory is the repository of the review.
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

	var sessions [][2]string
	var notified []*herdr.Agent
	for _, row := range rows {
		i := slices.IndexFunc(row.Commit.Trailers, func(t jujutsu.Trailer) bool { return t.Key == jujutsu.TrailerAgent })
		j := slices.IndexFunc(row.Commit.Trailers, func(t jujutsu.Trailer) bool { return t.Key == jujutsu.TrailerAgentSession })
		if i == -1 || j == -1 {
			continue
		}
		kind, sessionID := row.Commit.Trailers[i].Value, row.Commit.Trailers[j].Value
		if slices.Contains(sessions, [2]string{kind, sessionID}) {
			continue
		}
		sessions = append(sessions, [2]string{kind, sessionID})

		if k := slices.IndexFunc(agents, func(v *herdr.Agent) bool {
			a := toAgent(v, nil)
			return a.GetKind() == kind && a.GetSessionId() == sessionID
		}); k != -1 {
			if err := h.herdr.PromptAgent(ctx, agents[k].PaneID, finishedPrompt(r.ID)); err != nil {
				return nil, toConnectError(err)
			}
			notified = append(notified, agents[k])
			continue
		}

		resume, ok := resumeArgs[kind]
		if !ok {
			return nil, connect.NewError(connect.CodeFailedPrecondition, xerrors.Newf("the session %s of %s can't be resumed", sessionID, kind))
		}
		pane, cleanup, err := createPane(ctx, h.herdr, "", r.Repository)
		if err != nil {
			return nil, toConnectError(err)
		}
		agent, err := startAgent(ctx, h.herdr, agentName(kind), kind, pane.PaneID, resume(sessionID))
		if err == nil {
			err = h.herdr.PromptAgent(ctx, agent.PaneID, finishedPrompt(r.ID))
		}
		if err != nil {
			if closeErr := cleanup(context.WithoutCancel(ctx)); closeErr != nil {
				logger.Log.Info("Failed to close the pane", slog.String("pane_id", pane.PaneID), logger.E(closeErr))
			}
			return nil, toConnectError(err)
		}
		notified = append(notified, agent)
	}
	if len(notified) == 0 {
		return nil, nil
	}

	workspaces, err := workspaceLabels(ctx, h.herdr)
	if err != nil {
		return nil, err
	}
	res := make([]*api.Agent, 0, len(notified))
	for _, v := range notified {
		res = append(res, toAgent(v, workspaces))
	}
	return res, nil
}

// finishedPrompt is the prompt to the agent when the review is finished.
func finishedPrompt(id string) string {
	return "The code review " + id + " in Codexel is finished. Read the unresolved comments by the list_review_comments tool of the codexel MCP server. " +
		"Address each comment, and reply to it or resolve it. Then call the start_next_round tool."
}

func (h *ReviewHandler) Discard(_ context.Context, req *connect.Request[api.RequestDiscard]) (*connect.Response[api.ResponseDiscard], error) {
	r, err := h.store.Update(req.Msg.GetReviewId(), func(r *review.Review) error { return r.Discard(h.now()) })
	if err != nil {
		return nil, toReviewError(err)
	}
	return connect.NewResponse(api.ResponseDiscard_builder{Review: toReview(r)}.Build()), nil
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

// reviewAgent returns the working directory and the prompt of the agent which reviews the open review and comments on
// it by MCP. The prompt is read from trunk() so that the changes under review don't change it.
func reviewAgent(ctx context.Context, conf *config.File, store *review.Store, reviewID string) (string, string, error) {
	r, err := store.Get(reviewID)
	if err != nil {
		return "", "", toReviewError(err)
	}
	if r.State != review.StateOpen {
		return "", "", toReviewError(xerrors.WithMessagef(review.ErrInvalidState, "the review is %s", r.State))
	}
	repo, err := openedRepository(conf, r.Repository)
	if err != nil {
		return "", "", err
	}
	instructions, err := repo.File(ctx, reviewPromptRevision, reviewPromptPath)
	if jjErr, ok := errors.AsType[*jujutsu.Error](err); ok {
		return "", "", connect.NewError(connect.CodeFailedPrecondition, xerrors.Newf("failed to read %s at %s: %s", reviewPromptPath, reviewPromptRevision, jjErr.Error()))
	} else if err != nil {
		return "", "", connect.NewError(connect.CodeInternal, err)
	}
	return r.Repository, reviewerPrompt(r, string(instructions)), nil
}

// reviewerPrompt is the prompt to the agent which reviews the changes of the latest round. instructions are the
// instructions of the repository.
func reviewerPrompt(r *review.Review, instructions string) string {
	var b strings.Builder
	b.WriteString("You are the reviewer of the code review " + r.ID + " in Codexel. Review the changes below and comment on them by the " +
		"add_review_comment tool of the codexel MCP server. Don't change the files and the commits. Don't reply to or resolve the comments, " +
		"and don't start the next round. The user decides which comments are passed to the agent which created the changes.\n\n")
	b.WriteString("The changes (change ID: commit ID):\n")
	for _, v := range r.Changes {
		if commit, ok := r.Commit(r.LatestRound(), v); ok {
			b.WriteString("- " + v + ": " + commit + "\n")
		}
	}
	b.WriteString("\nReview them by the following instructions.\n\n")
	b.WriteString(instructions)
	return b.String()
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

// generatedFiles returns the paths of the generated files like GitHub. linguist-generated of .gitattributes at the root
// of the commit is used if it is specified, otherwise the heuristics of Linguist are used. The content of the Go files
// is judged by the convention of Go instead of Linguist. Only the top-level .gitattributes is read. The deleted files and the binary files are judged only by the path.
// The files are read concurrently because reading each file runs jj.
func generatedFiles(ctx context.Context, repo *jujutsu.Repository, commit string, files []*jujutsu.FileDiff) ([]string, error) {
	content, err := repo.File(ctx, commit, ".gitattributes")
	if _, ok := errors.AsType[*jujutsu.Error](err); ok {
		content = nil
	} else if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	attrs := git.ParseAttributes(content)
	isGenerated := make([]bool, len(files))
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(runtime.NumCPU())
	for i, v := range files {
		if g, ok := attrs.Generated(v.Path); ok {
			isGenerated[i] = g
			continue
		}
		eg.Go(func() error {
			var data []byte
			if v.Status != jujutsu.FileStatusDeleted && !v.Binary {
				d, err := repo.File(ctx, commit, v.Path)
				if err != nil {
					return toJujutsuError(err)
				}
				data = d
			}
			if path.Ext(v.Path) == ".go" {
				isGenerated[i] = enry.IsGenerated(v.Path, nil) || generatedGoRegexp.Match(data)
			} else {
				isGenerated[i] = enry.IsGenerated(v.Path, data)
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	var generated []string
	for i, v := range files {
		if isGenerated[i] {
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
	review.StateOpen:      api.ReviewState_REVIEW_STATE_OPEN,
	review.StateFinished:  api.ReviewState_REVIEW_STATE_FINISHED,
	review.StateApproved:  api.ReviewState_REVIEW_STATE_APPROVED,
	review.StateDiscarded: api.ReviewState_REVIEW_STATE_DISCARDED,
}

// resumeArgs returns the arguments of the agent to resume the session for each kind of the agents which add the
// trailers by jj-trailer.
var resumeArgs = map[string]func(sessionID string) []string{
	"claude":   func(sessionID string) []string { return []string{"--resume", sessionID} },
	"opencode": func(sessionID string) []string { return []string{"--session", sessionID} },
}

// generatedGoRegexp is the header of the generated Go files defined at https://go.dev/s/generatedcode
var generatedGoRegexp = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

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
