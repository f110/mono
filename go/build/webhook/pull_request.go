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
	ghwebhook "go.f110.dev/mono/go/github/webhook"
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

func (r *PullRequestReconciler) Reconcile(ctx context.Context, ev *ghwebhook.Event, event *github.PullRequestEvent) error {
	var status PullRequestStatus
	if err := ev.ReadStatus(&status); err != nil {
		return xerrors.WithStack(err)
	}

	repo, err := FindRepository(ctx, r.dao, event.GetRepo().GetHTMLURL())
	if err != nil {
		return err
	}
	if repo == nil {
		status.Skipped = true
		status.SkipReason = "unmanaged repository"
		_ = ev.WriteStatus(&status)
		ev.State = ghwebhook.StateSkipped
		return nil
	}

	switch event.GetAction() {
	case "opened":
		return r.handleOpenedOrSynchronize(ctx, ev, event, repo, &status, true)
	case "synchronize":
		return r.handleOpenedOrSynchronize(ctx, ev, event, repo, &status, false)
	case "closed":
		return r.handleClosed(ctx, ev, event, &status)
	default:
		status.Skipped = true
		status.SkipReason = "unhandled action: " + event.GetAction()
		_ = ev.WriteStatus(&status)
		ev.State = ghwebhook.StateSkipped
		return nil
	}
}

func (r *PullRequestReconciler) handleOpenedOrSynchronize(ctx context.Context, ev *ghwebhook.Event, event *github.PullRequestEvent, repo *database.SourceRepository, status *PullRequestStatus, postCommentIfDenied bool) error {
	if status.Skipped {
		ev.State = ghwebhook.StateSkipped
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
		_ = ev.WriteStatus(status)
		ev.State = ghwebhook.StateSkipped
		return nil
	}

	if SkipCI(event) {
		status.Skipped = true
		status.SkipReason = "skip ci marker"
		_ = ev.WriteStatus(status)
		ev.State = ghwebhook.StateSkipped
		return nil
	}

	owner := event.GetRepo().GetOwner().GetLogin()
	repoName := event.GetRepo().GetName()
	revision := event.GetPullRequest().GetHead().GetSHA()

	conf, err := fetchDefaultBranchConfig(ctx, r.githubClient, r.gitDataClient, gitDataRepoName(repo), owner, repoName, event.GetRepo().GetDefaultBranch())
	if err != nil {
		// Treat as "no config found" — legacy code logged + skipped rather
		// than failing. Same here.
		slogger.Log.Info("Skip build", slogger.E(err), slog.String("owner", owner), slog.String("repo", repoName), slog.String("revision", revision))
		status.Skipped = true
		status.SkipReason = "failed to fetch build config"
		_ = ev.WriteStatus(status)
		ev.State = ghwebhook.StateSkipped
		return nil
	}
	if conf == nil {
		status.Skipped = true
		status.SkipReason = "no build config or no jobs"
		_ = ev.WriteStatus(status)
		ev.State = ghwebhook.StateSkipped
		return nil
	}

	if status.DispatchedTaskIDs == nil {
		jobs := conf.Job(config.EventPullRequest)
		tasks, err := dispatchBuilds(ctx, r.builder, owner, repoName, repo, jobs, conf.BazelVersion, revision, "pull_request", false)
		// Checkpoint the created task ids even on a partial failure so a retry
		// resumes instead of dispatching the same jobs again.
		if ids := TaskIDs(tasks); len(ids) > 0 {
			status.DispatchedTaskIDs = ids
		}
		if err != nil {
			_ = ev.WriteStatus(status)
			return err
		}
	}

	if !status.ConfigValidated {
		if err := r.checkRevisionAvailable(ctx, gitDataRepoName(repo), revision); err != nil {
			_ = ev.WriteStatus(status)
			return err
		}
		validateState := "failure"
		if _, err := fetchBuildConfig(ctx, r.githubClient, r.gitDataClient, gitDataRepoName(repo), owner, repoName, revision); err == nil {
			validateState = "success"
		}
		if _, _, err := r.githubClient.Repositories.CreateStatus(ctx, owner, repoName, revision, github.RepoStatus{State: new(validateState), Context: new("Validate config")}); err != nil {
			_ = ev.WriteStatus(status)
			return xerrors.WithStack(err)
		}
		status.ConfigValidated = true
	}

	_ = ev.WriteStatus(status)
	ev.State = ghwebhook.StateSucceeded
	return nil
}

// checkRevisionAvailable confirms the source the head config is read from
// (git-data-service) has fetched sha, by querying that service directly rather
// than coupling to whatever component performs the fetch.
func (r *PullRequestReconciler) checkRevisionAvailable(ctx context.Context, gitDataRepo, sha string) error {
	if r.gitDataClient == nil {
		return nil
	}
	// GetCommit errors while the object is absent from the mirror.
	if _, err := r.gitDataClient.GetCommit(ctx, &git.RequestGetCommit{Repo: gitDataRepo, Sha: sha}); err != nil {
		return xerrors.WithMessagef(err, "revision %s is not yet available in git-data-service", sha)
	}
	return nil
}

func (r *PullRequestReconciler) handleClosed(ctx context.Context, ev *ghwebhook.Event, event *github.PullRequestEvent, status *PullRequestStatus) error {
	if status.PermitDeletedId != 0 {
		ev.State = ghwebhook.StateSucceeded
		return nil
	}
	permits, err := r.dao.PermitPullRequest.ListByRepositoryAndNumber(ctx, event.GetRepo().GetFullName(), int32(event.GetPullRequest().GetNumber()))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return xerrors.WithStack(err)
	}
	if len(permits) == 0 {
		status.Skipped = true
		status.SkipReason = "no permit pull request row"
		_ = ev.WriteStatus(status)
		ev.State = ghwebhook.StateSkipped
		return nil
	}
	permit := permits[0]
	if err := r.dao.PermitPullRequest.Delete(ctx, permit.Id); err != nil {
		return xerrors.WithStack(err)
	}
	status.PermitDeletedId = permit.Id
	_ = ev.WriteStatus(status)
	ev.State = ghwebhook.StateSucceeded
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
