// Package webhook persists incoming GitHub webhook deliveries to the
// `github_event` table and reconciles them asynchronously.
//
// The HTTP handler is intentionally thin: it parses the delivery, inserts a
// PENDING row, and returns 200. A scheduler running on the elected leader
// polls the table at a fixed interval (and reacts to in-process kicks) and
// dispatches each row to a Reconciler keyed by event_type. Reconcilers persist
// progress into the row's status JSON column so that they can be re-run
// idempotently after process restarts or partial failures.
package webhook

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/logger/slogger"
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

// Reconciler processes a single GitHub event row to completion.
//
// Each Reconciler owns the row's state lifecycle from PROCESSING through a
// terminal state (SUCCEEDED / SKIPPED / FAILED). The scheduler does not write
// ev.State after dispatching; implementations must persist the row via the
// dao themselves — Claim and Finalize are provided for the conventional
// pattern.
//
// Implementations must be idempotent: when called repeatedly with the same
// row, intermediate progress must be checkpointed into ev.Status (typically a
// JSON document keyed to EventType()) so that re-entry resumes work instead
// of duplicating side effects. A non-nil error is informational only — the
// reconciler is expected to have already moved the row to FAILED (via
// Finalize) before returning.
type Reconciler interface {
	EventType() string
	Reconcile(ctx context.Context, ev *database.GithubEvent) error
}

// Claim transitions ev to PROCESSING and persists the row. Reconcilers call
// this at the top of Reconcile so a concurrent or restarted scheduler does
// not pick the same row up twice.
func Claim(ctx context.Context, daos dao.Options, ev *database.GithubEvent) error {
	ev.State = database.GithubEventStateProcessing
	if err := daos.GithubEvent.Update(ctx, ev); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

// Finalize is the deferred counterpart of Claim. It chooses a terminal state
// based on recErr and the reconciler's in-memory ev.State, then persists the
// row.
//
//   - recErr != nil           → FAILED (last_error = recErr.Error())
//   - ev.State == PROCESSING  → SUCCEEDED (the reconciler completed without
//     picking a non-default terminal state)
//   - ev.State == SKIPPED/…   → preserved (the reconciler set it explicitly)
//
// Persistence failures here are logged but not returned — the row will get
// picked up again on the next tick if its state still matches the scan
// predicate.
func Finalize(ctx context.Context, daos dao.Options, ev *database.GithubEvent, recErr error) {
	if recErr != nil {
		ev.State = database.GithubEventStateFailed
		ev.LastError = recErr.Error()
	} else if ev.State == database.GithubEventStateProcessing {
		ev.State = database.GithubEventStateSucceeded
		ev.LastError = ""
	} else {
		ev.LastError = ""
	}
	if err := daos.GithubEvent.Update(ctx, ev); err != nil {
		slogger.Log.Warn("Failed to persist reconcile state", slog.Int("id", int(ev.Id)), slogger.E(err))
	}
}

// Reconcilers groups reconcilers by event_type for the scheduler.
type Reconcilers map[string]Reconciler

func (r Reconcilers) Register(rec Reconciler) {
	r[rec.EventType()] = rec
}

// readStatus unmarshals the row's status JSON column into out. An empty
// status (newly inserted row) leaves out at its zero value.
func readStatus(ev *database.GithubEvent, out any) error {
	if len(ev.Status) == 0 {
		return nil
	}
	return json.Unmarshal(ev.Status, out)
}

// WriteStatus marshals in into the row's status JSON column.
func WriteStatus(ev *database.GithubEvent, in any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return xerrors.WithStack(err)
	}
	ev.Status = b
	return nil
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
