package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/testing/assertion"
)

// pushPayload builds a minimal PushEvent JSON. branch is the short branch
// name (e.g. "master" or "feature"); message is the head_commit message used
// for [skip ci] detection.
func pushPayload(t *testing.T, branch, message string) []byte {
	t.Helper()
	p := map[string]any{
		"ref": "refs/heads/" + branch,
		"head_commit": map[string]any{
			"id":      "deadbeef",
			"message": message,
		},
		"repository": map[string]any{
			"name":          "ops",
			"full_name":     "f110/ops",
			"html_url":      "https://github.com/f110/ops",
			"clone_url":     "https://github.com/f110/ops.git",
			"master_branch": "master",
			"owner":         map[string]any{"login": "f110"},
		},
	}
	b, err := json.Marshal(p)
	assertion.MustNoError(t, err)
	return b
}

func TestPushReconciler(t *testing.T) {
	logger.SetLogLevel("debug")
	slogger.Init()

	cases := []struct {
		name           string
		branch         string
		commitMessage  string
		wantState      database.GithubEventState
		wantSkipReason string
	}{
		{
			name:           "push to a non-main branch is skipped",
			branch:         "feature",
			commitMessage:  "normal commit",
			wantState:      database.GithubEventStateSkipped,
			wantSkipReason: "non-main branch",
		},
		{
			name:           "push to main with [skip ci] is skipped",
			branch:         "master",
			commitMessage:  "[skip ci] no-op change",
			wantState:      database.GithubEventStateSkipped,
			wantSkipReason: "skip ci marker",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newTestDAO()
			builder := &recBuilder{}
			r := NewPushReconciler(d.toOptions(), nil, builder, nil, nil)

			ev := &database.GithubEvent{
				EventType: "push",
				Payload:   pushPayload(t, tc.branch, tc.commitMessage),
				State:     database.GithubEventStateProcessing,
				CreatedAt: time.Now(),
			}
			err := r.Reconcile(context.Background(), ev)
			assertion.MustNoError(t, err)
			assertion.Equal(t, ev.State, tc.wantState)
			assertion.Equal(t, builder.called, false)

			var st PushStatus
			assertion.MustNoError(t, readStatus(ev, &st))
			assertion.Equal(t, st.Skipped, true)
			assertion.Equal(t, st.SkipReason, tc.wantSkipReason)
		})
	}
}

// recSyncer records the URLs it was asked to sync so tests can assert that the
// push reconciler triggered a git-data fetch.
type recSyncer struct {
	urls []string
	err  error
}

var _ GitSyncer = (*recSyncer)(nil)

func (s *recSyncer) Sync(_ context.Context, url string) error {
	s.urls = append(s.urls, url)
	return s.err
}

// A push to a non-main branch must still sync git-data before it is skipped
// for dispatch: when the branch backs a pull request, the PR reconciler later
// needs that revision available in git-data-service. The branch is not built
// (state stays SKIPPED with reason "non-main branch"), but the fetch must have
// run.
func TestPushReconciler_SyncsNonMainBranch(t *testing.T) {
	logger.SetLogLevel("debug")
	slogger.Init()

	d := newTestDAO()
	builder := &recBuilder{}
	syncer := &recSyncer{}
	r := NewPushReconciler(d.toOptions(), nil, builder, syncer, nil)

	ev := &database.GithubEvent{
		EventType: "push",
		Payload:   pushPayload(t, "feature", "normal commit"),
		State:     database.GithubEventStateProcessing,
		CreatedAt: time.Now(),
	}
	err := r.Reconcile(context.Background(), ev)
	assertion.MustNoError(t, err)

	// git-data was synced for the pushed clone URL even though the branch is
	// not built.
	assertion.MustLen(t, syncer.urls, 1)
	assertion.Equal(t, syncer.urls[0], "https://github.com/f110/ops.git")
	assertion.Equal(t, builder.called, false)
	assertion.Equal(t, ev.State, database.GithubEventStateSkipped)

	var st PushStatus
	assertion.MustNoError(t, readStatus(ev, &st))
	assertion.True(t, st.GitSyncedAt != nil)
	assertion.Equal(t, st.SkipReason, "non-main branch")
}

// When a build is dispatched but the builder fails after creating the task
// row, the created task id must still be recorded in the status so the retry
// does not create a duplicate task for the same commit.
func TestPushReconciler_RecordsTasksOnDispatchError(t *testing.T) {
	logger.SetLogLevel("debug")
	slogger.Init()

	const opsURL = "https://github.com/f110/ops"
	const headSHA = "deadbeef"

	d := newTestDAO()
	d.Repository.RegisterListByUrl(opsURL, []*database.SourceRepository{repoFixture(opsURL, "ops")}, nil)
	d.Job.RegisterListByRepositoryId(1, nil, nil)
	d.ExternalReleaseTrigger.RegisterListByRepositoryId(1, nil, nil)
	builder := &recBuilder{err: xerrors.Define("failed to launch job").WithStack()}
	gh := github.NewClient(&http.Client{Transport: configTransport(t, "f110/ops", headSHA, "push")})
	r := NewPushReconciler(d.toOptions(), gh, builder, nil, nil)

	ev := &database.GithubEvent{
		Id:        1,
		EventType: "push",
		Payload:   pushPayload(t, "master", "a change"),
		State:     database.GithubEventStateProcessing,
		CreatedAt: time.Now(),
	}
	err := r.Reconcile(context.Background(), ev)
	assertion.Error(t, err)
	assertion.Equal(t, ev.State, database.GithubEventStateFailed)

	var st PushStatus
	assertion.MustNoError(t, readStatus(ev, &st))
	assertion.Equal(t, len(st.DispatchedTaskIDs), 1)
}
