package webhook

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v85/github"

	"go.f110.dev/mono/go/build/database"
	ghwebhook "go.f110.dev/mono/go/github/webhook"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestPullRequestReconciler(t *testing.T) {
	logger.SetLogLevel("debug")
	slogger.Init()

	const opsURL = "https://github.com/f110/ops"
	const sandboxURL = "https://github.com/f110/sandbox"
	trustedUser := &database.TrustedUser{Id: 1, GithubId: 2178441, Username: "octocat"}

	// fixture captures everything a case needs to assert against after
	// Reconcile returns. Cases without a mockTransport leave it nil.
	type fixture struct {
		dao      *testDAO
		gh       *github.Client
		recTrans *recTransport
		builder  *recBuilder
		// headSHA, when set, is injected into the payload's pull request head so
		// it matches the commit the githubmock serves.
		headSHA string
	}

	cases := []struct {
		name        string
		payload     string
		setup       func(t *testing.T) *fixture
		wantState   ghwebhook.State
		wantBuilder bool
		check       func(t *testing.T, f *fixture, ev *ghwebhook.Event)
	}{
		{
			name:    "opened by an untrusted user posts the allow-build comment and skips",
			payload: "pull_request_opened.json",
			setup: func(t *testing.T) *fixture {
				d := newTestDAO()
				d.Repository.RegisterListByUrl(opsURL, []*database.SourceRepository{repoFixture(opsURL, "ops")}, nil)
				d.TrustedUser.RegisterListByGithubId(2178441, nil, sql.ErrNoRows)
				d.PermitPullRequest.RegisterListByRepositoryAndNumber("f110/ops", 28, nil, sql.ErrNoRows)
				tr := &recTransport{res: []*http.Response{{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}}}
				return &fixture{
					dao:      d,
					gh:       github.NewClient(&http.Client{Transport: tr}),
					recTrans: tr,
					builder:  &recBuilder{},
				}
			},
			wantState:   ghwebhook.StateSkipped,
			wantBuilder: false,
			check: func(t *testing.T, f *fixture, ev *ghwebhook.Event) {
				assertion.MustLen(t, f.recTrans.req, 1)
				bodyBytes, err := io.ReadAll(f.recTrans.req[0].Body)
				assertion.MustNoError(t, err)
				var apiReq github.IssueComment
				assertion.MustNoError(t, json.Unmarshal(bodyBytes, &apiReq))
				assertion.True(t, strings.Contains(apiReq.GetBody(), AllowCommand))

				var st PullRequestStatus
				assertion.MustNoError(t, ev.ReadStatus(&st))
				assertion.Equal(t, st.NotAllowed, true)
				assertion.Equal(t, st.CommentPosted, true)
			},
		},
		{
			name:    "opened by a trusted user dispatches the build",
			payload: "pull_request_opened.json",
			setup: func(t *testing.T) *fixture {
				d := newTestDAO()
				d.TrustedUser.RegisterListByGithubId(trustedUser.GithubId, []*database.TrustedUser{trustedUser}, nil)
				d.Repository.RegisterListByUrl(opsURL, []*database.SourceRepository{repoFixture(opsURL, "ops")}, nil)
				tr, sha := configTransport(t, "f110/ops", "pull_request")
				return &fixture{
					dao:     d,
					gh:      github.NewClient(&http.Client{Transport: tr}),
					headSHA: sha,
					builder: &recBuilder{},
				}
			},
			wantState:   ghwebhook.StateSucceeded,
			wantBuilder: true,
			check: func(t *testing.T, f *fixture, _ *ghwebhook.Event) {
				assertion.MustLen(t, f.builder.jobNames, 1)
			},
		},
		{
			name:    "opened with an existing PermitPullRequest row dispatches the build",
			payload: "pull_request_opened.json",
			setup: func(t *testing.T) *fixture {
				d := newTestDAO()
				d.TrustedUser.RegisterListByGithubId(2178441, nil, sql.ErrNoRows)
				d.PermitPullRequest.RegisterListByRepositoryAndNumber("f110/ops", 28,
					[]*database.PermitPullRequest{{Id: 1, Repository: "f110/ops", Number: 28, CreatedAt: time.Now()}},
					nil,
				)
				d.Repository.RegisterListByUrl(opsURL, []*database.SourceRepository{repoFixture(opsURL, "ops")}, nil)
				tr, sha := configTransport(t, "f110/ops", "pull_request")
				return &fixture{
					dao:     d,
					gh:      github.NewClient(&http.Client{Transport: tr}),
					headSHA: sha,
					builder: &recBuilder{},
				}
			},
			wantState:   ghwebhook.StateSucceeded,
			wantBuilder: true,
		},
		{
			name:    "synchronize dispatches the build",
			payload: "pull_request_synchronize.json",
			setup: func(t *testing.T) *fixture {
				d := newTestDAO()
				d.TrustedUser.RegisterListByGithubId(trustedUser.GithubId, []*database.TrustedUser{trustedUser}, nil)
				d.Repository.RegisterListByUrl(opsURL, []*database.SourceRepository{repoFixture(opsURL, "ops")}, nil)
				tr, sha := configTransport(t, "f110/ops", "pull_request")
				return &fixture{
					dao:     d,
					gh:      github.NewClient(&http.Client{Transport: tr}),
					headSHA: sha,
					builder: &recBuilder{},
				}
			},
			wantState:   ghwebhook.StateSucceeded,
			wantBuilder: true,
		},
		{
			name:    "closed deletes the matching PermitPullRequest row",
			payload: "pull_request_closed.json",
			setup: func(t *testing.T) *fixture {
				d := newTestDAO()
				d.Repository.RegisterListByUrl(sandboxURL, []*database.SourceRepository{repoFixture(sandboxURL, "sandbox")}, nil)
				d.PermitPullRequest.RegisterListByRepositoryAndNumber("f110/sandbox", 2,
					[]*database.PermitPullRequest{{Id: 7, Repository: "f110/sandbox", Number: 2, CreatedAt: time.Now()}},
					nil,
				)
				return &fixture{dao: d, builder: &recBuilder{}}
			},
			wantState:   ghwebhook.StateSucceeded,
			wantBuilder: false,
			check: func(t *testing.T, f *fixture, _ *ghwebhook.Event) {
				called := f.dao.PermitPullRequest.Called("Delete")
				assertion.MustLen(t, called, 1)
				assertion.Equal[any](t, called[0].Args["id"], int32(7))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.setup(t)
			r := NewPullRequestReconciler(f.dao.toOptions(), f.gh, f.builder, nil)
			ev := makeEvent(t, "pull_request", tc.payload)
			if f.headSHA != "" {
				ev.Payload = setPullRequestHeadSHA(t, ev.Payload, f.headSHA)
			}

			err := reconcile[github.PullRequestEvent](r, ev)
			assertion.MustNoError(t, err)
			assertion.Equal(t, ev.State, tc.wantState)
			assertion.Equal(t, f.builder.called, tc.wantBuilder)
			if tc.check != nil {
				tc.check(t, f, ev)
			}
		})
	}
}

// setPullRequestHeadSHA overrides the pull request head SHA in a webhook
// payload so it matches the commit the githubmock serves (whose SHA is
// git-computed and cannot be hard-coded).
func setPullRequestHeadSHA(t *testing.T, payload []byte, sha string) []byte {
	t.Helper()
	var ev github.PullRequestEvent
	assertion.MustNoError(t, json.Unmarshal(payload, &ev))
	if ev.PullRequest == nil {
		ev.PullRequest = &github.PullRequest{}
	}
	if ev.PullRequest.Head == nil {
		ev.PullRequest.Head = &github.PullRequestBranch{}
	}
	ev.PullRequest.Head.SHA = &sha
	b, err := json.Marshal(&ev)
	assertion.MustNoError(t, err)
	return b
}

func TestPullRequestReconciler_RequeuesUntilHeadRevisionAvailable(t *testing.T) {
	logger.SetLogLevel("debug")
	slogger.Init()

	const opsURL = "https://github.com/f110/ops"
	const headSHA = "69f2c2703436688cb49bdcf858e8cf59a9b06e08"
	trustedUser := &database.TrustedUser{Id: 1, GithubId: 2178441, Username: "octocat"}

	d := newTestDAO()
	d.TrustedUser.RegisterListByGithubId(trustedUser.GithubId, []*database.TrustedUser{trustedUser}, nil)
	d.Repository.RegisterListByUrl(opsURL, []*database.SourceRepository{repoFixture(opsURL, "ops")}, nil)

	// gh is only used to post the "Validate config" commit status on the retry.
	gh := github.NewClient(&http.Client{Transport: &recTransport{res: []*http.Response{
		{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader("{}"))},
	}}})
	builder := &recBuilder{}
	// The head is reported missing on the first probe, then available.
	fakeGit := &fakeGitData{defaultHash: "masterhash", availableAfter: 1}

	r := NewPullRequestReconciler(d.toOptions(), gh, builder, fakeGit)
	ev := makeEvent(t, "pull_request", "pull_request_opened.json")

	// First loop: the head is not in git-data-service yet, so the reconcile
	// requeues (returns an error → event FAILED). The build was dispatched and
	// checkpointed before the validate step.
	err := reconcile[github.PullRequestEvent](r, ev)
	assertion.Error(t, err)
	assertion.Equal(t, ev.State, ghwebhook.StateFailed)
	assertion.MustLen(t, fakeGit.commitSHAs, 1)
	assertion.MustLen(t, builder.jobNames, 1)
	assertion.False(t, slices.Contains(fakeGit.treeRefs, headSHA)) // head not read yet

	// Next loop: the revision has landed, so validation reads the head config
	// and the event succeeds. The build is not dispatched again.
	err = reconcile[github.PullRequestEvent](r, ev)
	assertion.MustNoError(t, err)
	assertion.Equal(t, ev.State, ghwebhook.StateSucceeded)
	assertion.True(t, slices.Contains(fakeGit.commitSHAs, headSHA))
	assertion.True(t, slices.Contains(fakeGit.treeRefs, headSHA))
	assertion.MustLen(t, builder.jobNames, 1) // no re-dispatch
}

func TestPullRequestReconciler_ClosedDoesNotQueryGitData(t *testing.T) {
	logger.SetLogLevel("debug")
	slogger.Init()

	const sandboxURL = "https://github.com/f110/sandbox"
	d := newTestDAO()
	d.Repository.RegisterListByUrl(sandboxURL, []*database.SourceRepository{repoFixture(sandboxURL, "sandbox")}, nil)
	d.PermitPullRequest.RegisterListByRepositoryAndNumber("f110/sandbox", 2,
		[]*database.PermitPullRequest{{Id: 7, Repository: "f110/sandbox", Number: 2, CreatedAt: time.Now()}},
		nil,
	)
	fakeGit := &fakeGitData{defaultHash: "masterhash"}

	r := NewPullRequestReconciler(d.toOptions(), nil, &recBuilder{}, fakeGit)
	ev := makeEvent(t, "pull_request", "pull_request_closed.json")

	err := reconcile[github.PullRequestEvent](r, ev)
	assertion.MustNoError(t, err)
	assertion.Equal(t, ev.State, ghwebhook.StateSucceeded)
	assertion.MustLen(t, fakeGit.commitSHAs, 0)
}
