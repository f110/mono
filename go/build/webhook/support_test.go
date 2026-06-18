package webhook

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"go.f110.dev/githubmock"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/database/dao/daotest"
	"go.f110.dev/mono/go/testing/assertion"
)

// testDAO bundles the daotest mocks the reconcilers exercise.
type testDAO struct {
	Repository             *daotest.SourceRepository
	Task                   *daotest.Task
	TrustedUser            *daotest.TrustedUser
	PermitPullRequest      *daotest.PermitPullRequest
	GithubEvent            *daotest.GithubEvent
	ExternalReleaseTrigger *daotest.ExternalReleaseTrigger
	Job                    *daotest.Job
}

func newTestDAO() *testDAO {
	return &testDAO{
		Repository:             daotest.NewSourceRepository(),
		Task:                   daotest.NewTask(),
		TrustedUser:            daotest.NewTrustedUser(),
		PermitPullRequest:      daotest.NewPermitPullRequest(),
		GithubEvent:            daotest.NewGithubEvent(),
		ExternalReleaseTrigger: daotest.NewExternalReleaseTrigger(),
		Job:                    daotest.NewJob(),
	}
}

func (d *testDAO) toOptions() dao.Options {
	return dao.Options{
		Repository:             d.Repository,
		Task:                   d.Task,
		TrustedUser:            d.TrustedUser,
		PermitPullRequest:      d.PermitPullRequest,
		GithubEvent:            d.GithubEvent,
		ExternalReleaseTrigger: d.ExternalReleaseTrigger,
		Job:                    d.Job,
	}
}

// recBuilder records each Build invocation and returns a single fake Task.
// When err is set it returns that error alongside the created task, mirroring
// coordinator.BazelBuilder.Build which persists the task row before it may fail
// to launch the underlying job.
type recBuilder struct {
	called   bool
	jobNames []string
	tasks    []*database.Task
	err      error
}

var _ Builder = (*recBuilder)(nil)

func (m *recBuilder) Build(_ context.Context, _ *database.SourceRepository, job *config.JobV2, _, _, _ string, _, _ []string, _ string, _ bool) ([]*database.Task, error) {
	m.called = true
	m.jobNames = append(m.jobNames, job.Name)
	t := &database.Task{Id: int32(len(m.jobNames))}
	m.tasks = append(m.tasks, t)
	return []*database.Task{t}, m.err
}

// recTransport captures every request it sees so tests can assert on the
// body sent to the GitHub API (e.g. the comment posted on an unallowed PR).
// It returns the canned responses supplied at construction time, in order.
type recTransport struct {
	res   []*http.Response
	req   []*http.Request
	index int
}

func (m *recTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(m.res) <= m.index {
		return nil, nil
	}
	m.req = append(m.req, req)
	r := m.res[m.index]
	m.index++
	return r, nil
}

func loadPayload(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	assertion.MustNoError(t, err)
	return b
}

func makeEvent(t *testing.T, eventType, payloadFile string) *database.GithubEvent {
	t.Helper()
	return &database.GithubEvent{
		Id:         1,
		DeliveryId: "test-delivery",
		EventType:  eventType,
		Payload:    loadPayload(t, payloadFile),
		State:      database.GithubEventStateProcessing,
		CreatedAt:  time.Now(),
	}
}

// repoFixture is the SourceRepository row the reconcilers expect when they
// look up a known repository.
func repoFixture(url, name string) *database.SourceRepository {
	return &database.SourceRepository{
		Id:        1,
		Url:       url,
		CloneUrl:  url + ".git",
		Name:      name,
		CreatedAt: time.Now(),
	}
}

// configTransport builds a githubmock transport that serves a repository
// with a single job `.build/test.cue` that subscribes to eventType. Used by
// any reconciler test where the fetchBuildConfig path must succeed.
func configTransport(t *testing.T, fullName, sha, eventType string) http.RoundTripper {
	t.Helper()
	m := githubmock.NewMock()
	repo := m.Repository(fullName)
	err := repo.Commits(githubmock.NewCommit().
		SHA(sha).
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
		event: ["` + eventType + `"]
	}
}
`)},
			&githubmock.File{Name: ".bazelversion", Body: []byte("8.4.1")},
		),
	)
	assertion.MustNoError(t, err)
	return m.Transport()
}
