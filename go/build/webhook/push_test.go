package webhook

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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
