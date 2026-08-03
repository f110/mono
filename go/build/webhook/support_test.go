package webhook

import (
	"context"
	"net/http"
	"os"
	"path"
	"testing"
	"time"

	"go.f110.dev/githubmock"
	"go.f110.dev/xerrors"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/database/dao/daotest"
	"go.f110.dev/mono/go/git"
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

// prTestCue is a single-job build config subscribed to the pull_request event,
// served by fakeGitData for the git-data-service read path.
var prTestCue = []byte(`jobs: {
	test_all: {
		command: "test"
		targets: ["//..."]
		platforms: ["@rules_go//go/toolchain:linux_amd64"]
		all_revision:  true
		github_status: true
		event: ["pull_request"]
	}
}
`)

// fakeGitData is a minimal git.GitDataClient that serves prTestCue as the
// repository's .build config for any ref, so reconciler tests can exercise the
// git-data-service read path without a real service. Only the methods the
// config reader uses are implemented; the embedded nil interface makes any
// other call panic, flagging unexpected access.
type fakeGitData struct {
	git.GitDataClient
	defaultHash string
	treeRefs    []string // refs passed to GetTree, in call order
	commitSHAs  []string // SHAs probed via GetCommit, in call order
	// availableAfter is how many GetCommit calls report the commit as missing
	// before it becomes available, simulating a mirror that has not fetched the
	// revision yet.
	availableAfter int
}

var _ git.GitDataClient = (*fakeGitData)(nil)

func (f *fakeGitData) GetReference(_ context.Context, _ *git.RequestGetReference, _ ...grpc.CallOption) (*git.ResponseGetReference, error) {
	return &git.ResponseGetReference{Ref: &git.Reference{Hash: f.defaultHash}}, nil
}

func (f *fakeGitData) GetCommit(_ context.Context, in *git.RequestGetCommit, _ ...grpc.CallOption) (*git.ResponseGetCommit, error) {
	f.commitSHAs = append(f.commitSHAs, in.GetSha())
	if len(f.commitSHAs) <= f.availableAfter {
		return nil, xerrors.New("object not found")
	}
	return &git.ResponseGetCommit{Commit: &git.Commit{Sha: in.GetSha()}}, nil
}

func (f *fakeGitData) GetTree(_ context.Context, in *git.RequestGetTree, _ ...grpc.CallOption) (*git.ResponseGetTree, error) {
	f.treeRefs = append(f.treeRefs, in.GetRef())
	return &git.ResponseGetTree{Tree: []*git.TreeEntry{
		{Path: ".build/test.cue", Mode: "0100644", Size: int64(len(prTestCue))},
	}}, nil
}

func (f *fakeGitData) GetFile(_ context.Context, in *git.RequestGetFile, _ ...grpc.CallOption) (*git.ResponseGetFile, error) {
	switch path.Base(in.GetPath()) {
	case "test.cue":
		return &git.ResponseGetFile{Content: prTestCue}, nil
	case ".bazelversion":
		return &git.ResponseGetFile{Content: []byte("8.4.1")}, nil
	}
	return nil, xerrors.Definef("fakeGitData: unexpected file %q", in.GetPath()).WithStack()
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

// configTransport builds a githubmock transport that serves a repository with
// a single job `.build/test.cue` subscribing to eventType. It returns the
// transport and the git-computed SHA of the head commit so callers can align
// the webhook payload (PR head / head_commit) with the commit the mock serves.
func configTransport(t *testing.T, fullName, eventType string) (http.RoundTripper, string) {
	t.Helper()
	m := githubmock.NewMock()
	repo := m.Repository(fullName)
	commit := githubmock.NewCommit().
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
		)
	assertion.MustNoError(t, repo.Commits(commit))
	return m.Transport(), commit.GetSHA()
}
