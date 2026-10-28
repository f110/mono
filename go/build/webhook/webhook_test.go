package webhook

import (
	"testing"

	"github.com/google/go-github/v85/github"

	ghwebhook "go.f110.dev/mono/go/github/webhook"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestReconcilersSkipUnmanagedRepository(t *testing.T) {
	logger.SetLogLevel("debug")
	slogger.Init()

	cases := []struct {
		name    string
		repoURL string
		run     func(d *testDAO, builder *recBuilder, syncer *recSyncer) (*ghwebhook.Event, error)
		status  func(t *testing.T, ev *ghwebhook.Event) (bool, string)
	}{
		{
			name:    "push",
			repoURL: "https://github.com/f110/ops",
			run: func(d *testDAO, builder *recBuilder, syncer *recSyncer) (*ghwebhook.Event, error) {
				ev := &ghwebhook.Event{EventType: "push", Payload: pushPayload(t, "master", "normal commit", "deadbeef")}
				return ev, reconcile[github.PushEvent](NewPushReconciler(d.toOptions(), nil, builder, syncer, nil), ev)
			},
			status: func(t *testing.T, ev *ghwebhook.Event) (bool, string) {
				var st PushStatus
				assertion.MustNoError(t, ev.ReadStatus(&st))
				return st.Skipped, st.SkipReason
			},
		},
		{
			name:    "pull_request",
			repoURL: "https://github.com/f110/ops",
			run: func(d *testDAO, builder *recBuilder, _ *recSyncer) (*ghwebhook.Event, error) {
				ev := makeEvent(t, "pull_request", "pull_request_opened.json")
				return ev, reconcile[github.PullRequestEvent](NewPullRequestReconciler(d.toOptions(), nil, builder, nil), ev)
			},
			status: func(t *testing.T, ev *ghwebhook.Event) (bool, string) {
				var st PullRequestStatus
				assertion.MustNoError(t, ev.ReadStatus(&st))
				return st.Skipped, st.SkipReason
			},
		},
		{
			name:    "release",
			repoURL: "https://github.com/f110/sandbox",
			run: func(d *testDAO, builder *recBuilder, _ *recSyncer) (*ghwebhook.Event, error) {
				ev := makeEvent(t, "release", "release_published.json")
				return ev, reconcile[github.ReleaseEvent](NewReleaseReconciler(d.toOptions(), nil, builder, nil), ev)
			},
			status: func(t *testing.T, ev *ghwebhook.Event) (bool, string) {
				var st ReleaseStatus
				assertion.MustNoError(t, ev.ReadStatus(&st))
				return st.Skipped, st.SkipReason
			},
		},
		{
			name:    "issue_comment",
			repoURL: "https://github.com/f110/ops",
			run: func(d *testDAO, builder *recBuilder, _ *recSyncer) (*ghwebhook.Event, error) {
				ev := makeEvent(t, "issue_comment", "issue_comment.json")
				return ev, reconcile[github.IssueCommentEvent](NewIssueCommentReconciler(d.toOptions(), nil, builder, nil), ev)
			},
			status: func(t *testing.T, ev *ghwebhook.Event) (bool, string) {
				var st IssueCommentStatus
				assertion.MustNoError(t, ev.ReadStatus(&st))
				return st.Skipped, st.SkipReason
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newTestDAO()
			d.Repository.RegisterListByUrl(tc.repoURL, nil, nil)
			builder := &recBuilder{}
			syncer := &recSyncer{}

			ev, err := tc.run(d, builder, syncer)
			assertion.MustNoError(t, err)
			assertion.Equal(t, ev.State, ghwebhook.StateSkipped)
			assertion.False(t, builder.called)
			assertion.Len(t, syncer.urls, 0)
			skipped, reason := tc.status(t, ev)
			assertion.True(t, skipped)
			assertion.Equal(t, reason, "unmanaged repository")
		})
	}
}
