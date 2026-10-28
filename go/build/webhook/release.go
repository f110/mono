package webhook

import (
	"context"
	"fmt"

	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/git"
	ghwebhook "go.f110.dev/mono/go/github/webhook"
)

// ReleaseReconciler handles `release` deliveries. Only the `published`
// action triggers a build, matching the legacy behavior.
type ReleaseReconciler struct {
	dao           dao.Options
	githubClient  *github.Client
	builder       Builder
	gitDataClient git.GitDataClient
}

func NewReleaseReconciler(daos dao.Options, gh *github.Client, builder Builder, gitDataClient git.GitDataClient) *ReleaseReconciler {
	return &ReleaseReconciler{dao: daos, githubClient: gh, builder: builder, gitDataClient: gitDataClient}
}

func (*ReleaseReconciler) EventType() string { return "release" }

func (r *ReleaseReconciler) Reconcile(ctx context.Context, ev *ghwebhook.Event, event *github.ReleaseEvent) error {
	var status ReleaseStatus
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

	if status.Skipped {
		ev.State = ghwebhook.StateSkipped
		return nil
	}
	if event.GetAction() != "published" {
		status.Skipped = true
		status.SkipReason = "unhandled action: " + event.GetAction()
		_ = ev.WriteStatus(&status)
		ev.State = ghwebhook.StateSkipped
		return nil
	}

	owner := event.GetRepo().GetOwner().GetLogin()
	repoName := event.GetRepo().GetName()

	ref, _, err := r.githubClient.Git.GetRef(ctx, owner, repoName, fmt.Sprintf("tags/%s", event.GetRelease().GetTagName()))
	if err != nil {
		return xerrors.WithStack(err)
	}
	// The legacy handler did a fire-and-forget GetTag call here; preserve it
	// so the GitHub API call counts match in any caching/quota logic.
	r.githubClient.Git.GetTag(ctx, owner, repoName, event.GetRelease().GetTagName())

	revision := ref.GetObject().GetSHA()

	conf, err := fetchBuildConfig(ctx, r.githubClient, r.gitDataClient, gitDataRepoName(repo), owner, repoName, revision)
	if err != nil {
		// Mirrors legacy: log + skip rather than fail.
		status.Skipped = true
		status.SkipReason = "failed to fetch build config"
		_ = ev.WriteStatus(&status)
		ev.State = ghwebhook.StateSkipped
		return nil
	}
	if conf == nil {
		status.Skipped = true
		status.SkipReason = "no build config or no jobs"
		_ = ev.WriteStatus(&status)
		ev.State = ghwebhook.StateSkipped
		return nil
	}

	if status.DispatchedTaskIDs == nil {
		jobs := conf.Job(config.EventRelease)
		tasks, err := dispatchBuilds(ctx, r.builder, owner, repoName, repo, jobs, conf.BazelVersion, revision, "release", false)
		// Checkpoint the created task ids even on a partial failure so a retry
		// resumes instead of dispatching the same jobs again.
		if ids := TaskIDs(tasks); len(ids) > 0 {
			status.DispatchedTaskIDs = ids
		}
		if err != nil {
			_ = ev.WriteStatus(&status)
			return err
		}
	}

	_ = ev.WriteStatus(&status)
	ev.State = ghwebhook.StateSucceeded
	return nil
}
