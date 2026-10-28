// Package webhook implements the reconcilers of the GitHub webhook events for Build.
package webhook

import (
	"context"
	"strings"
	"time"

	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/git"
	ghwebhook "go.f110.dev/mono/go/github/webhook"
)

// SkipCIPrefix is the conventional commit/title marker that disables CI.
const SkipCIPrefix = "[skip ci]"

// AllowCommand is the magic phrase a trusted user posts in a PR comment to
// permit a build for an untrusted contributor.
const AllowCommand = "/allow-build"

// Builder dispatches build tasks. It mirrors the subset of
// coordinator.BazelBuilder needed by reconcilers and is declared here to avoid
// an import cycle.
type Builder interface {
	Build(ctx context.Context, repo *database.SourceRepository, job *config.JobV2, revision, bazelVersion, command string, targets, platforms []string, via string, isMainBranch bool) ([]*database.Task, error)
}

// NewEventProcessor returns the EventProcessor that dispatches the events to the reconcilers of Build.
func NewEventProcessor(daos dao.Options, gh *github.Client, builder Builder, gitSyncer GitSyncer, gitDataClient git.GitDataClient, interval, maxProcessingDuration time.Duration) *ghwebhook.EventProcessor {
	p := ghwebhook.NewEventProcessor(newStore(daos.GithubEvent), interval, maxProcessingDuration)
	p.Register[github.PushEvent](NewPushReconciler(daos, gh, builder, gitSyncer, gitDataClient))
	p.Register[github.PullRequestEvent](NewPullRequestReconciler(daos, gh, builder, gitDataClient))
	p.Register[github.ReleaseEvent](NewReleaseReconciler(daos, gh, builder, gitDataClient))
	p.Register[github.IssueCommentEvent](NewIssueCommentReconciler(daos, gh, builder, gitDataClient))
	return p
}

// IsMainBranch returns true if the push ref points at the repository's
// default branch.
func IsMainBranch(ref, masterBranch string) bool {
	b := strings.SplitN(ref, "/", 3)
	if len(b) < 3 {
		return false
	}
	return b[2] == masterBranch
}

// SkipCI returns true if e signals that CI should be skipped — either a push
// whose head commit message begins with SkipCIPrefix, or a pull request whose
// title does.
func SkipCI(e any) bool {
	switch event := e.(type) {
	case *github.PushEvent:
		return strings.HasPrefix(event.GetHeadCommit().GetMessage(), SkipCIPrefix)
	case *github.PullRequestEvent:
		return strings.HasPrefix(event.GetPullRequest().GetTitle(), SkipCIPrefix)
	}
	return false
}

// FindRepository looks up the SourceRepository row whose url matches repoURL.
// Returns nil if no row or multiple rows match — callers handle that the same
// way as the legacy api package did.
func FindRepository(ctx context.Context, daos dao.Options, repoURL string) (*database.SourceRepository, error) {
	repos, err := daos.Repository.ListByUrl(ctx, repoURL)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	if len(repos) != 1 {
		return nil, nil
	}
	return repos[0], nil
}

// TaskIDs extracts the Id field from each task. Convenience for status
// recording.
func TaskIDs(tasks []*database.Task) []int32 {
	out := make([]int32, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, t.Id)
	}
	return out
}
