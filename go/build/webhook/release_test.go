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

func TestReleaseReconciler(t *testing.T) {
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
	}{
		{
			name:    "published release dispatches the release jobs",
			payload: "release_published.json",
			setup: func(t *testing.T) *fixture {
				const sandboxURL = "https://github.com/f110/sandbox"
				d := newTestDAO()
				d.Repository.RegisterListByUrl(sandboxURL, []*database.SourceRepository{repoFixture(sandboxURL, "sandbox")}, nil)

				m := githubmock.NewMock()
				ghRepo := m.Repository("f110/sandbox")
				commit := githubmock.NewCommit().
					SHA("69f2c2703436688cb49bdcf858e8cf59a9b06e08").
					IsHead().
					Files(
						&githubmock.File{Name: ".build/release.cue", Body: []byte(`jobs: {
	release: {
		command: "test"
		targets: ["//..."]
		platforms: ["@rules_go//go/toolchain:linux_amd64"]
		all_revision:  true
		github_status: true
		cpu_limit:     "2000m"
		memory_limit:  "8192Mi"
		event: ["release"]
	}
}
`)},
						&githubmock.File{Name: ".bazelversion", Body: []byte("8.4.1")},
					)
				assertion.MustNoError(t, ghRepo.Commits(commit))
				ghRepo.Tags(githubmock.NewTag().Name("1605187034").Commit(commit))

				return &fixture{
					dao:     d,
					gh:      github.NewClient(&http.Client{Transport: m.Transport()}),
					builder: &recBuilder{},
				}
			},
			wantState:   database.GithubEventStateSucceeded,
			wantBuilder: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.setup(t)
			r := NewReleaseReconciler(f.dao.toOptions(), f.gh, f.builder, nil)
			ev := makeEvent(t, "release", tc.payload)

			err := r.Reconcile(context.Background(), ev)
			assertion.MustNoError(t, err)
			assertion.Equal(t, ev.State, tc.wantState)
			assertion.Equal(t, f.builder.called, tc.wantBuilder)
		})
	}
}
