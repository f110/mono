package webhook

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v85/github"

	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestPullRequestReconciler(t *testing.T) {
	logger.SetLogLevel("debug")
	slogger.Init()

	const opsURL = "https://github.com/f110/ops"
	const headSHA = "69f2c2703436688cb49bdcf858e8cf59a9b06e08"
	const syncSHA = "5bd79ba34f1d860afc697c15830c80e2e63edfbf"
	trustedUser := &database.TrustedUser{Id: 1, GithubId: 2178441, Username: "octocat"}

	// fixture captures everything a case needs to assert against after
	// Reconcile returns. Cases without a mockTransport leave it nil.
	type fixture struct {
		dao         *testDAO
		gh          *github.Client
		recTrans    *recTransport
		builder     *recBuilder
	}

	cases := []struct {
		name        string
		payload     string
		setup       func(t *testing.T) *fixture
		wantState   database.GithubEventState
		wantBuilder bool
		check       func(t *testing.T, f *fixture, ev *database.GithubEvent)
	}{
		{
			name:    "opened by an untrusted user posts the allow-build comment and skips",
			payload: "pull_request_opened.json",
			setup: func(t *testing.T) *fixture {
				d := newTestDAO()
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
			wantState:   database.GithubEventStateSkipped,
			wantBuilder: false,
			check: func(t *testing.T, f *fixture, ev *database.GithubEvent) {
				assertion.MustLen(t, f.recTrans.req, 1)
				bodyBytes, err := io.ReadAll(f.recTrans.req[0].Body)
				assertion.MustNoError(t, err)
				var apiReq github.IssueComment
				assertion.MustNoError(t, json.Unmarshal(bodyBytes, &apiReq))
				assertion.True(t, strings.Contains(apiReq.GetBody(), AllowCommand))

				var st PullRequestStatus
				assertion.MustNoError(t, readStatus(ev, &st))
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
				return &fixture{
					dao:     d,
					gh:      github.NewClient(&http.Client{Transport: configTransport(t, "f110/ops", headSHA, "pull_request")}),
					builder: &recBuilder{},
				}
			},
			wantState:   database.GithubEventStateSucceeded,
			wantBuilder: true,
			check: func(t *testing.T, f *fixture, _ *database.GithubEvent) {
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
				return &fixture{
					dao:     d,
					gh:      github.NewClient(&http.Client{Transport: configTransport(t, "f110/ops", headSHA, "pull_request")}),
					builder: &recBuilder{},
				}
			},
			wantState:   database.GithubEventStateSucceeded,
			wantBuilder: true,
		},
		{
			name:    "synchronize dispatches the build",
			payload: "pull_request_synchronize.json",
			setup: func(t *testing.T) *fixture {
				d := newTestDAO()
				d.TrustedUser.RegisterListByGithubId(trustedUser.GithubId, []*database.TrustedUser{trustedUser}, nil)
				d.Repository.RegisterListByUrl(opsURL, []*database.SourceRepository{repoFixture(opsURL, "ops")}, nil)
				return &fixture{
					dao:     d,
					gh:      github.NewClient(&http.Client{Transport: configTransport(t, "f110/ops", syncSHA, "pull_request")}),
					builder: &recBuilder{},
				}
			},
			wantState:   database.GithubEventStateSucceeded,
			wantBuilder: true,
		},
		{
			name:    "closed deletes the matching PermitPullRequest row",
			payload: "pull_request_closed.json",
			setup: func(t *testing.T) *fixture {
				d := newTestDAO()
				d.PermitPullRequest.RegisterListByRepositoryAndNumber("f110/sandbox", 2,
					[]*database.PermitPullRequest{{Id: 7, Repository: "f110/sandbox", Number: 2, CreatedAt: time.Now()}},
					nil,
				)
				return &fixture{dao: d, builder: &recBuilder{}}
			},
			wantState:   database.GithubEventStateSucceeded,
			wantBuilder: false,
			check: func(t *testing.T, f *fixture, _ *database.GithubEvent) {
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

			err := r.Reconcile(context.Background(), ev)
			assertion.MustNoError(t, err)
			assertion.Equal(t, ev.State, tc.wantState)
			assertion.Equal(t, f.builder.called, tc.wantBuilder)
			if tc.check != nil {
				tc.check(t, f, ev)
			}
		})
	}
}
