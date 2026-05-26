package webhook

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/go-github/v85/github"
	"go.f110.dev/githubmock"

	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestIssueCommentReconciler(t *testing.T) {
	logger.SetLogLevel("debug")
	slogger.Init()

	type fixture struct {
		dao     *testDAO
		gh      *github.Client
		builder *recBuilder
	}

	cases := []struct {
		name        string
		payload     string
		setup       func(t *testing.T) *fixture
		wantState   database.GithubEventState
		wantBuilder bool
		check       func(t *testing.T, f *fixture)
	}{
		{
			name:    "trusted user posting /allow-build creates a permit and dispatches the build",
			payload: "issue_comment.json",
			setup: func(t *testing.T) *fixture {
				const opsURL = "https://github.com/f110/ops"
				d := newTestDAO()
				d.TrustedUser.RegisterListByGithubId(2178441, []*database.TrustedUser{{Id: 1, GithubId: 2178441, Username: "octocat"}}, nil)
				d.Repository.RegisterListByUrl(opsURL, []*database.SourceRepository{repoFixture(opsURL, "ops")}, nil)

				m := githubmock.NewMock()
				ghRepo := m.Repository("f110/ops")
				ghRepo.PullRequests(
					githubmock.NewPullRequest().
						Number(28).
						Head(nil, "", "69f2c2703436688cb49bdcf858e8cf59a9b06e08"),
				)
				err := ghRepo.Commits(githubmock.NewCommit().
					SHA("69f2c2703436688cb49bdcf858e8cf59a9b06e08").
					IsHead().
					Files(
						&githubmock.File{Name: ".build/test.cue", Body: []byte(`jobs: {
	test_all: {
		command: "test"
		targets: ["//..."]
		platforms: ["@rules_go//go/toolchain:linux_amd64"]
		all_revision:  true
		github_status: true
		cpu_limit:     "2000m"
		memory_limit:  "8192Mi"
		event: ["pull_request"]
	}
}
`)},
						&githubmock.File{Name: ".bazelversion", Body: []byte("8.4.1")},
					),
				)
				assertion.MustNoError(t, err)

				return &fixture{
					dao:     d,
					gh:      github.NewClient(&http.Client{Transport: m.Transport()}),
					builder: &recBuilder{},
				}
			},
			wantState:   database.GithubEventStateSucceeded,
			wantBuilder: true,
			check: func(t *testing.T, f *fixture) {
				created := f.dao.PermitPullRequest.Called("Create")
				assertion.MustLen(t, created, 1)
				assertion.Equal(t, created[0].Args["permitPullRequest"].(*database.PermitPullRequest).Repository, "f110/ops")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.setup(t)
			r := NewIssueCommentReconciler(f.dao.toOptions(), f.gh, f.builder)
			ev := makeEvent(t, "issue_comment", tc.payload)

			err := r.Reconcile(context.Background(), ev)
			assertion.MustNoError(t, err)
			assertion.Equal(t, ev.State, tc.wantState)
			assertion.Equal(t, f.builder.called, tc.wantBuilder)
			if tc.check != nil {
				tc.check(t, f)
			}
		})
	}
}
