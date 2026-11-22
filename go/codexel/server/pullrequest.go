package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"
	"golang.org/x/oauth2"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/codexel/config"
	"go.f110.dev/mono/go/codexel/pullrequest"
	githubutil "go.f110.dev/mono/go/github"
	"go.f110.dev/mono/go/jujutsu"
	"go.f110.dev/mono/go/logger"
)

type PullRequestHandler struct {
	config          *config.File
	pullRequests    *pullrequest.File
	newGitHubClient func(context.Context) (*github.Client, error)
}

var _ api.PullRequestServiceHandler = (*PullRequestHandler)(nil)

func NewPullRequestHandler(conf *config.File, pullRequests *pullrequest.File) *PullRequestHandler {
	return &PullRequestHandler{config: conf, pullRequests: pullRequests, newGitHubClient: newGitHubClient}
}

func (h *PullRequestHandler) List(_ context.Context, req *connect.Request[api.RequestListPullRequests]) (*connect.Response[api.ResponseListPullRequests], error) {
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	prs, err := h.pullRequests.List(repo.Root())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	res := make([]*api.PullRequest, 0, len(prs))
	for _, v := range prs {
		res = append(res, api.PullRequest_builder{ChangeId: new(jujutsu.ShortChangeID(v.ChangeID)), Url: new(v.URL)}.Build())
	}
	return connect.NewResponse(api.ResponseListPullRequests_builder{PullRequests: res}.Build()), nil
}

func (h *PullRequestHandler) ListTemplates(_ context.Context, req *connect.Request[api.RequestListPullRequestTemplates]) (*connect.Response[api.ResponseListPullRequestTemplates], error) {
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	templates, err := jujutsu.PullRequestTemplates(repo.Root())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	res := make([]string, 0, len(templates))
	for _, v := range templates {
		rel, err := filepath.Rel(repo.Root(), v)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, xerrors.WithStack(err))
		}
		res = append(res, rel)
	}
	return connect.NewResponse(api.ResponseListPullRequestTemplates_builder{Templates: res}.Build()), nil
}

func (h *PullRequestHandler) Validate(ctx context.Context, req *connect.Request[api.RequestValidatePullRequest]) (*connect.Response[api.ResponseValidatePullRequest], error) {
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	client, err := h.newGitHubClient(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	_, stack, reason, err := submitRevision(ctx, repo, client, req.Msg.GetRevisions())
	if err != nil {
		return nil, err
	}
	pushed := slices.ContainsFunc(stack, func(c *jujutsu.StackCommit) bool { return !c.NoSend() && len(c.Bookmarks) > 0 })
	return connect.NewResponse(api.ResponseValidatePullRequest_builder{Reason: new(reason), HasPushedCommits: new(pushed)}.Build()), nil
}

func (h *PullRequestHandler) Submit(ctx context.Context, req *connect.Request[api.RequestSubmitPullRequest]) (*connect.Response[api.ResponseSubmitPullRequest], error) {
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	client, err := h.newGitHubClient(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	revision, _, reason, err := submitRevision(ctx, repo, client, req.Msg.GetRevisions())
	if err != nil {
		return nil, err
	}
	if reason != "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New(reason))
	}

	s := jujutsu.NewPullRequestSubmitter()
	s.Dir, s.RootDir, s.Revision, s.SinglePR = repo.Root(), repo.Root(), revision, len(req.Msg.GetRevisions()) > 1
	s.CommentOnUpdate = req.Msg.GetCommentOnUpdate()
	s.GitHubClient = client
	s.SelectTemplate = func(_ []string) (string, error) {
		if req.Msg.GetTemplate() == "" {
			return "", nil
		}
		return filepath.Join(repo.Root(), req.Msg.GetTemplate()), nil
	}
	res, err := s.Submit(ctx)
	if err != nil {
		return nil, toJujutsuError(err)
	}
	saved := make([]*pullrequest.PullRequest, 0, len(res.PullRequests))
	prs := make([]*api.SubmittedPullRequest, 0, len(res.PullRequests))
	for _, v := range res.PullRequests {
		saved = append(saved, &pullrequest.PullRequest{Repository: repo.Root(), ChangeID: v.ChangeID, URL: v.URL})
		prs = append(prs, api.SubmittedPullRequest_builder{ChangeId: new(v.ChangeID), Url: new(v.URL), Action: new(pullRequestActions[v.Action])}.Build())
	}
	if err := h.pullRequests.Save(saved); err != nil {
		logger.Log.Info("Failed to save the pull requests", slog.String("path", repo.Root()), logger.E(err))
	}
	return connect.NewResponse(api.ResponseSubmitPullRequest_builder{Pushed: res.Pushed, PullRequests: prs}.Build()), nil
}

var pullRequestActions = map[jujutsu.PullRequestAction]api.PullRequestAction{
	jujutsu.PullRequestCreated:  api.PullRequestAction_PULL_REQUEST_ACTION_CREATED,
	jujutsu.PullRequestUpdated:  api.PullRequestAction_PULL_REQUEST_ACTION_UPDATED,
	jujutsu.PullRequestExisting: api.PullRequestAction_PULL_REQUEST_ACTION_EXISTING,
}

// submitRevision returns the change ID which is submitted for the revisions and the stack of it. reason is not empty if
// the revisions can't be submitted. Multiple revisions are submitted as one pull request of the newest one, so they must
// be the whole stack of it.
func submitRevision(ctx context.Context, repo *jujutsu.Repository, client *github.Client, revisions []string) (string, []*jujutsu.StackCommit, string, error) {
	if len(revisions) == 0 {
		return "", nil, "", connect.NewError(connect.CodeInvalidArgument, errors.New("revisions are required"))
	}
	heads, err := repo.Log(ctx, "heads("+strings.Join(revisions, "|")+")", 0)
	if err != nil {
		return "", nil, "", toJujutsuError(err)
	}
	if len(heads) != 1 {
		return "", nil, "The revisions are not in a line", nil
	}
	top := heads[0].Commit.ChangeID
	owner, name, err := jujutsu.GitHubRepository(ctx, repo.Root())
	if err != nil {
		return "", nil, "", connect.NewError(connect.CodeFailedPrecondition, err)
	}
	defaultBranch, err := jujutsu.DefaultBranch(ctx, client, repo.Root(), owner, name)
	if err != nil {
		return "", nil, "", connect.NewError(connect.CodeInternal, err)
	}
	stack, err := jujutsu.Stack(ctx, false, repo.Root(), defaultBranch, top)
	if err != nil {
		return "", nil, "", connect.NewError(connect.CodeInternal, err)
	}
	if len(stack) == 0 {
		return "", nil, fmt.Sprintf("There are no commits to submit between %s@origin and the revision", defaultBranch), nil
	}
	if len(revisions) > 1 && !isWholeStack(stack, revisions) {
		return "", nil, fmt.Sprintf("The revisions must be all commits between %s@origin and the newest revision (%d commits)", defaultBranch, len(stack)), nil
	}
	if !slices.ContainsFunc(stack, func(c *jujutsu.StackCommit) bool { return !c.NoSend() }) {
		return "", nil, fmt.Sprintf(`There are no commits to submit between %s@origin and the revision. The commits whose description starts with "no-send:" or "WIP:" are excluded`, defaultBranch), nil
	}
	return top, stack, "", nil
}

func newGitHubClient(ctx context.Context) (*github.Client, error) {
	token, err := githubutil.CLIToken(ctx)
	if err != nil {
		return nil, xerrors.WithMessage(err, "could not get the token of GitHub by gh")
	}
	return github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))), nil
}

// isWholeStack reports whether revisions are all commits of the stack. The revisions may be the prefixes of the commit
// IDs.
func isWholeStack(stack []*jujutsu.StackCommit, revisions []string) bool {
	if len(stack) != len(revisions) {
		return false
	}
	for _, v := range revisions {
		if !slices.ContainsFunc(stack, func(c *jujutsu.StackCommit) bool { return strings.HasPrefix(c.CommitID, v) }) {
			return false
		}
	}
	return true
}
