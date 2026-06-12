package webhook

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/logger/slogger"
)

// PushReconciler handles `push` deliveries. It fetches the build config at
// the pushed revision, reconciles external_release_trigger rows (only for
// main-branch pushes), and dispatches the build tasks for jobs subscribed to
// EventPush.
type PushReconciler struct {
	dao           dao.Options
	githubClient  *github.Client
	builder       Builder
	gitUpdater    *git.Updater
	gitDataClient git.GitDataClient
}

func NewPushReconciler(daos dao.Options, gh *github.Client, builder Builder, gitUpdater *git.Updater, gitDataClient git.GitDataClient) *PushReconciler {
	return &PushReconciler{dao: daos, githubClient: gh, builder: builder, gitUpdater: gitUpdater, gitDataClient: gitDataClient}
}

func (*PushReconciler) EventType() string { return "push" }

func (r *PushReconciler) Reconcile(ctx context.Context, ev *database.GithubEvent) (retErr error) {
	if err := Claim(ctx, r.dao, ev); err != nil {
		return err
	}
	defer func() { Finalize(ctx, r.dao, ev, retErr) }()

	var event github.PushEvent
	if err := unmarshalPayload(ev, &event); err != nil {
		return err
	}

	var status PushStatus
	if err := readStatus(ev, &status); err != nil {
		return xerrors.WithStack(err)
	}

	if r.gitUpdater != nil && status.GitSyncedAt == nil {
		cloneURL := event.GetRepo().GetCloneURL()
		err := r.gitUpdater.Sync(ctx, cloneURL)
		switch {
		case err == nil:
			status.GitSyncedAt = new(time.Now())
			status.GitSyncError = ""
		case errors.Is(err, git.ErrRepositoryNotTracked):
			status.GitSyncError = "repository is not tracked by git-data-service"
			slogger.Log.Info("Skip git sync: repository not tracked", slog.String("repo", cloneURL))
		default:
			status.GitSyncError = err.Error()
			slogger.Log.Warn("Failed to sync repository", slogger.E(err), slog.String("repo", cloneURL))
			_ = WriteStatus(ev, &status)
			return err
		}
	}

	if status.Skipped {
		ev.State = database.GithubEventStateSkipped
		return nil
	}

	isMain := IsMainBranch(event.GetRef(), event.GetRepo().GetMasterBranch())
	if !isMain {
		// The legacy handler skipped non-main pushes entirely. Preserve that.
		status.Skipped = true
		status.SkipReason = "non-main branch"
		_ = WriteStatus(ev, &status)
		ev.State = database.GithubEventStateSkipped
		return nil
	}
	if SkipCI(&event) {
		status.Skipped = true
		status.SkipReason = "skip ci marker"
		_ = WriteStatus(ev, &status)
		ev.State = database.GithubEventStateSkipped
		return nil
	}

	repo, err := FindRepository(ctx, r.dao, event.GetRepo().GetHTMLURL())
	if err != nil {
		return err
	}

	owner := event.GetRepo().GetOwner().GetLogin()
	repoName := event.GetRepo().GetName()
	revision := event.GetHeadCommit().GetID()

	conf, err := r.fetchConfig(ctx, owner, repoName, revision)
	if err != nil {
		return err
	}
	if conf == nil {
		// Empty config or no jobs — nothing more to do.
		status.Skipped = true
		status.SkipReason = "no build config or no jobs"
		_ = WriteStatus(ev, &status)
		ev.State = database.GithubEventStateSkipped
		return nil
	}
	if status.ConfigFetchedAt == nil {
		status.ConfigFetchedAt = new(time.Now())
	}

	if repo != nil && status.ExternalReconciledAt == nil {
		if err := reconcileExternalReleaseTriggers(ctx, r.dao, repo, conf.Jobs); err != nil {
			// External release reconcile errors are non-fatal in the legacy
			// flow — they were logged and skipped. Mirror that here so
			// transient DB hiccups don't block the build dispatch.
			slogger.Log.Warn("Failed to reconcile external_release triggers", slog.String("repo", repoName), slogger.E(err))
		} else {
			status.ExternalReconciledAt = new(time.Now())
		}
	}

	if repo != nil && status.JobsReconciledAt == nil {
		if err := reconcileJobs(ctx, r.dao, repo, conf.Jobs); err != nil {
			// Failing to refresh the manual-job cache is non-fatal: InvokeJob
			// can still fall back to reading from GitHub. Log and continue
			// so a transient DB hiccup doesn't block the build dispatch.
			slogger.Log.Warn("Failed to reconcile jobs", slog.String("repo", repoName), slogger.E(err))
		} else {
			status.JobsReconciledAt = new(time.Now())
		}
		if repo.BazelVersion != conf.BazelVersion {
			repo.BazelVersion = conf.BazelVersion
			if err := r.dao.Repository.Update(ctx, repo); err != nil {
				slogger.Log.Warn("Failed to update bazel_version", slog.String("repo", repoName), slogger.E(err))
			}
		}
	}

	if status.DispatchedTaskIDs == nil {
		jobs := conf.Job(config.EventPush)
		tasks, err := dispatchBuilds(ctx, r.builder, owner, repoName, repo, jobs, conf.BazelVersion, revision, "push", true)
		if err != nil {
			_ = WriteStatus(ev, &status)
			return err
		}
		status.DispatchedTaskIDs = TaskIDs(tasks)
	}

	if err := WriteStatus(ev, &status); err != nil {
		return err
	}
	ev.State = database.GithubEventStateSucceeded
	return nil
}
