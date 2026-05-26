package webhook

import (
	"context"
	"fmt"

	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
)

// ReleaseReconciler handles `release` deliveries. Only the `published`
// action triggers a build, matching the legacy behavior.
type ReleaseReconciler struct {
	dao          dao.Options
	githubClient *github.Client
	builder      Builder
}

func NewReleaseReconciler(daos dao.Options, gh *github.Client, builder Builder) *ReleaseReconciler {
	return &ReleaseReconciler{dao: daos, githubClient: gh, builder: builder}
}

func (*ReleaseReconciler) EventType() string { return "release" }

func (r *ReleaseReconciler) Reconcile(ctx context.Context, ev *database.GithubEvent) (retErr error) {
	if err := Claim(ctx, r.dao, ev); err != nil {
		return err
	}
	defer func() { Finalize(ctx, r.dao, ev, retErr) }()

	var event github.ReleaseEvent
	if err := unmarshalPayload(ev, &event); err != nil {
		return err
	}

	var status ReleaseStatus
	if err := readStatus(ev, &status); err != nil {
		return xerrors.WithStack(err)
	}

	if status.Skipped {
		ev.State = database.GithubEventStateSkipped
		return nil
	}
	if event.GetAction() != "published" {
		status.Skipped = true
		status.SkipReason = "unhandled action: " + event.GetAction()
		_ = WriteStatus(ev, &status)
		ev.State = database.GithubEventStateSkipped
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

	repo, err := FindRepository(ctx, r.dao, event.GetRepo().GetHTMLURL())
	if err != nil {
		return err
	}

	conf, err := fetchBuildConfig(ctx, r.githubClient, owner, repoName, revision, true)
	if err != nil {
		// Mirrors legacy: log + skip rather than fail.
		status.Skipped = true
		status.SkipReason = "failed to fetch build config"
		_ = WriteStatus(ev, &status)
		ev.State = database.GithubEventStateSkipped
		return nil
	}
	if conf == nil {
		status.Skipped = true
		status.SkipReason = "no build config or no jobs"
		_ = WriteStatus(ev, &status)
		ev.State = database.GithubEventStateSkipped
		return nil
	}

	if status.DispatchedTaskIDs == nil {
		jobs := conf.Job(config.EventRelease)
		tasks, err := dispatchBuilds(ctx, r.builder, owner, repoName, repo, jobs, conf.BazelVersion, revision, "release", false)
		if err != nil {
			_ = WriteStatus(ev, &status)
			return err
		}
		status.DispatchedTaskIDs = TaskIDs(tasks)
	}

	_ = WriteStatus(ev, &status)
	ev.State = database.GithubEventStateSucceeded
	return nil
}
