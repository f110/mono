package webhook

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/git"
)

// IssueCommentReconciler implements the `/allow-build` flow: a trusted user
// commenting AllowCommand on an untrusted contributor's PR records a
// PermitPullRequest row and immediately re-runs the build pipeline against
// the PR head.
type IssueCommentReconciler struct {
	dao           dao.Options
	githubClient  *github.Client
	builder       Builder
	gitDataClient git.GitDataClient
}

func NewIssueCommentReconciler(daos dao.Options, gh *github.Client, builder Builder, gitDataClient git.GitDataClient) *IssueCommentReconciler {
	return &IssueCommentReconciler{dao: daos, githubClient: gh, builder: builder, gitDataClient: gitDataClient}
}

func (*IssueCommentReconciler) EventType() string { return "issue_comment" }

func (r *IssueCommentReconciler) Reconcile(ctx context.Context, ev *database.GithubEvent) (retErr error) {
	if err := Claim(ctx, r.dao, ev); err != nil {
		return err
	}
	defer func() { Finalize(ctx, r.dao, ev, retErr) }()

	var event github.IssueCommentEvent
	if err := unmarshalPayload(ev, &event); err != nil {
		return err
	}

	var status IssueCommentStatus
	if err := readStatus(ev, &status); err != nil {
		return xerrors.WithStack(err)
	}

	if status.Skipped {
		ev.State = database.GithubEventStateSkipped
		return nil
	}
	if event.GetAction() != "created" || !strings.Contains(event.GetComment().GetBody(), AllowCommand) {
		status.Skipped = true
		status.SkipReason = "comment is not an allow-build trigger"
		_ = WriteStatus(ev, &status)
		ev.State = database.GithubEventStateSkipped
		return nil
	}

	users, err := r.dao.TrustedUser.ListByGithubId(ctx, event.GetSender().GetID())
	if err != nil {
		return xerrors.WithStack(err)
	}
	if len(users) != 1 {
		status.Skipped = true
		status.SkipReason = "sender is not a trusted user"
		_ = WriteStatus(ev, &status)
		ev.State = database.GithubEventStateSkipped
		return nil
	}

	if status.PermitCreatedId == 0 {
		permit, err := r.dao.PermitPullRequest.Create(ctx, &database.PermitPullRequest{
			Repository: event.GetRepo().GetFullName(),
			Number:     int32(event.GetIssue().GetNumber()),
		})
		if err != nil {
			return xerrors.WithStack(err)
		}
		status.PermitCreatedId = permit.Id
	}

	if !status.CommentPosted {
		body := "Understood. This pull request added to allow list.\n" +
			"We are going to build the job."
		if _, _, err := r.githubClient.Issues.CreateComment(
			ctx,
			event.GetRepo().GetOwner().GetLogin(),
			event.GetRepo().GetName(),
			event.GetIssue().GetNumber(),
			&github.IssueComment{Body: new(body)},
		); err != nil {
			_ = WriteStatus(ev, &status)
			return xerrors.WithStack(err)
		}
		status.CommentPosted = true
	}

	if status.DispatchedTaskIDs == nil {
		owner := event.GetRepo().GetOwner().GetLogin()
		repoName := event.GetRepo().GetName()
		number := event.GetIssue().GetNumber()

		pr, res, err := r.githubClient.PullRequests.Get(ctx, owner, repoName, number)
		if err != nil {
			_ = WriteStatus(ev, &status)
			return xerrors.WithStack(err)
		}
		if res.StatusCode != http.StatusOK {
			_ = WriteStatus(ev, &status)
			return xerrors.Define("could not get pr").WithStack()
		}
		revision := pr.GetHead().GetSHA()

		repo, err := FindRepository(ctx, r.dao, event.GetRepo().GetHTMLURL())
		if err != nil {
			_ = WriteStatus(ev, &status)
			return err
		}

		conf, err := fetchBuildConfig(ctx, r.githubClient, r.gitDataClient, owner, repoName, "HEAD")
		if err != nil {
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

		jobs := conf.Job(config.EventPullRequest)
		tasks, err := dispatchBuilds(ctx, r.builder, owner, repoName, repo, jobs, conf.BazelVersion, revision, "pr", false)
		// Checkpoint the created task ids even on a partial failure so a retry
		// resumes instead of dispatching the same jobs again.
		if ids := TaskIDs(tasks); len(ids) > 0 {
			status.DispatchedTaskIDs = ids
		}
		if err != nil {
			_ = WriteStatus(ev, &status)
			return err
		}
	}

	_ = WriteStatus(ev, &status)
	ev.State = database.GithubEventStateSucceeded
	return nil
}
