package webhook

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/logger/slogger"
)

// PullRequestReconciler dispatches the pull_request action set: opened,
// synchronize, closed.
type PullRequestReconciler struct {
	dao           dao.Options
	githubClient  *github.Client
	builder       Builder
	gitDataClient git.GitDataClient
}

func NewPullRequestReconciler(daos dao.Options, gh *github.Client, builder Builder, gitDataClient git.GitDataClient) *PullRequestReconciler {
	return &PullRequestReconciler{dao: daos, githubClient: gh, builder: builder, gitDataClient: gitDataClient}
}

func (*PullRequestReconciler) EventType() string { return "pull_request" }

func (r *PullRequestReconciler) Reconcile(ctx context.Context, ev *database.GithubEvent) (retErr error) {
	if err := Claim(ctx, r.dao, ev); err != nil {
		return err
	}
	defer func() { Finalize(ctx, r.dao, ev, retErr) }()

	var event github.PullRequestEvent
	if err := unmarshalPayload(ev, &event); err != nil {
		return err
	}

	var status PullRequestStatus
	if err := readStatus(ev, &status); err != nil {
		return xerrors.WithStack(err)
	}

	switch event.GetAction() {
	case "opened":
		return r.handleOpenedOrSynchronize(ctx, ev, &event, &status, true)
	case "synchronize":
		return r.handleOpenedOrSynchronize(ctx, ev, &event, &status, false)
	case "closed":
		return r.handleClosed(ctx, ev, &event, &status)
	default:
		status.Skipped = true
		status.SkipReason = "unhandled action: " + event.GetAction()
		_ = WriteStatus(ev, &status)
		ev.State = database.GithubEventStateSkipped
		return nil
	}
}

func (r *PullRequestReconciler) handleOpenedOrSynchronize(ctx context.Context, ev *database.GithubEvent, event *github.PullRequestEvent, status *PullRequestStatus, postCommentIfDenied bool) error {
	if status.Skipped {
		ev.State = database.GithubEventStateSkipped
		return nil
	}

	allowed, err := r.allowPullRequest(ctx, event)
	if err != nil {
		return err
	}
	if !allowed {
		if postCommentIfDenied && !status.CommentPosted {
			body := "Sorry, We could not build this pull request. Because building this pull request is not allowed due to security reason.\n\n" +
				"For author, Thank you for your contribution. We appreciate your work. Please wait for permitting to build this pull request by administrator.\n" +
				"For administrator, If you are going to allow this pull request, please comment `" + AllowCommand + "`."
			if _, _, err := r.githubClient.Issues.CreateComment(
				ctx,
				event.GetRepo().GetOwner().GetLogin(),
				event.GetRepo().GetName(),
				event.GetPullRequest().GetNumber(),
				&github.IssueComment{Body: new(body)},
			); err != nil {
				return xerrors.WithStack(err)
			}
			status.CommentPosted = true
		}
		status.NotAllowed = true
		status.Skipped = true
		status.SkipReason = "pull request not allowed"
		_ = WriteStatus(ev, status)
		ev.State = database.GithubEventStateSkipped
		return nil
	}

	if SkipCI(event) {
		status.Skipped = true
		status.SkipReason = "skip ci marker"
		_ = WriteStatus(ev, status)
		ev.State = database.GithubEventStateSkipped
		return nil
	}

	owner := event.GetRepo().GetOwner().GetLogin()
	repoName := event.GetRepo().GetName()
	revision := event.GetPullRequest().GetHead().GetSHA()

	repo, err := FindRepository(ctx, r.dao, event.GetRepo().GetHTMLURL())
	if err != nil {
		return err
	}

	conf, err := fetchBuildConfig(ctx, r.githubClient, r.gitDataClient, owner, repoName, "HEAD")
	if err != nil {
		// Treat as "no config found" — legacy code logged + skipped rather
		// than failing. Same here.
		slogger.Log.Info("Skip build", slogger.E(err), slog.String("owner", owner), slog.String("repo", repoName), slog.String("revision", revision))
		status.Skipped = true
		status.SkipReason = "failed to fetch build config"
		_ = WriteStatus(ev, status)
		ev.State = database.GithubEventStateSkipped
		return nil
	}
	if conf == nil {
		status.Skipped = true
		status.SkipReason = "no build config or no jobs"
		_ = WriteStatus(ev, status)
		ev.State = database.GithubEventStateSkipped
		return nil
	}

	if status.DispatchedTaskIDs == nil {
		jobs := conf.Job(config.EventPullRequest)
		tasks, err := dispatchBuilds(ctx, r.builder, owner, repoName, repo, jobs, conf.BazelVersion, revision, "pull_request", false)
		if err != nil {
			_ = WriteStatus(ev, status)
			return err
		}
		status.DispatchedTaskIDs = TaskIDs(tasks)
	}

	if !status.ConfigValidated {
		validateState := "failure"
		if _, err := fetchBuildConfig(ctx, r.githubClient, r.gitDataClient, owner, repoName, revision); err == nil {
			validateState = "success"
		}
		if _, _, err := r.githubClient.Repositories.CreateStatus(ctx, owner, repoName, revision, github.RepoStatus{State: new(validateState), Context: new("Validate config")}); err != nil {
			_ = WriteStatus(ev, status)
			return xerrors.WithStack(err)
		}
		status.ConfigValidated = true
	}

	_ = WriteStatus(ev, status)
	ev.State = database.GithubEventStateSucceeded
	return nil
}

func (r *PullRequestReconciler) handleClosed(ctx context.Context, ev *database.GithubEvent, event *github.PullRequestEvent, status *PullRequestStatus) error {
	if status.PermitDeletedId != 0 {
		ev.State = database.GithubEventStateSucceeded
		return nil
	}
	permits, err := r.dao.PermitPullRequest.ListByRepositoryAndNumber(ctx, event.GetRepo().GetFullName(), int32(event.GetPullRequest().GetNumber()))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return xerrors.WithStack(err)
	}
	if len(permits) == 0 {
		status.Skipped = true
		status.SkipReason = "no permit pull request row"
		_ = WriteStatus(ev, status)
		ev.State = database.GithubEventStateSkipped
		return nil
	}
	permit := permits[0]
	if err := r.dao.PermitPullRequest.Delete(ctx, permit.Id); err != nil {
		return xerrors.WithStack(err)
	}
	status.PermitDeletedId = permit.Id
	_ = WriteStatus(ev, status)
	ev.State = database.GithubEventStateSucceeded
	return nil
}

// allowPullRequest mirrors the legacy api.allowPullRequest: a PR is allowed
// when the sender owns the repo, the sender is a TrustedUser, or there is a
// PermitPullRequest row recorded for this PR.
func (r *PullRequestReconciler) allowPullRequest(ctx context.Context, event *github.PullRequestEvent) (bool, error) {
	if event.GetRepo().GetOwner().GetLogin() == event.GetSender().GetLogin() {
		return true, nil
	}
	users, err := r.dao.TrustedUser.ListByGithubId(ctx, event.GetSender().GetID())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, xerrors.WithStack(err)
	}
	if len(users) == 1 {
		return true, nil
	}
	permits, err := r.dao.PermitPullRequest.ListByRepositoryAndNumber(ctx, event.GetRepo().GetFullName(), int32(event.GetPullRequest().GetNumber()))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, xerrors.WithStack(err)
	}
	if len(permits) > 0 {
		return true, nil
	}
	return false, nil
}
