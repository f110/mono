package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v73/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/database/dao/daotest"
	"go.f110.dev/mono/go/logger"
)

type MockBuilder struct {
	jobs   []*config.Job
	called bool
}

var _ Builder = &MockBuilder{}

func (m *MockBuilder) Build(_ context.Context, repo *database.SourceRepository, job *config.Job, revision, bazelVersion, command string, targets, platforms []string, via string, isMainBranch bool) ([]*database.Task, error) {
	m.jobs = append(m.jobs, job)
	m.called = true
	return []*database.Task{}, nil
}

func (m *MockBuilder) ForceStop(_ context.Context, _ int32) error {
	return nil
}

type MockTransport struct {
	req   []*http.Request
	res   []*http.Response
	index int
}

func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(m.res) <= m.index {
		return nil, nil
	}
	m.req = append(m.req, req)
	res := m.res[m.index]
	m.index++
	return res, nil
}

type mockDAO struct {
	Repository        *daotest.SourceRepository
	Task              *daotest.Task
	TrustedUser       *daotest.TrustedUser
	PermitPullRequest *daotest.PermitPullRequest
}

func newMock() *mockDAO {
	return &mockDAO{
		Repository:        daotest.NewSourceRepository(),
		Task:              daotest.NewTask(),
		TrustedUser:       daotest.NewTrustedUser(),
		PermitPullRequest: daotest.NewPermitPullRequest(),
	}
}

func TestGithubWebHook(t *testing.T) {
	logger.SetLogLevel("debug")
	logger.Init()

	trustedUser := &database.TrustedUser{
		Id:        1,
		GithubId:  2178441,
		Username:  "octocat",
		CreatedAt: time.Now(),
	}
	opsRepository := &database.SourceRepository{
		Id:        1,
		Url:       "https://github.com/f110/ops",
		CloneUrl:  "https://github.com/f110/ops.git",
		Name:      "ops",
		Private:   false,
		CreatedAt: time.Now(),
		UpdatedAt: nil,
	}
	sandboxRepository := &database.SourceRepository{
		Id:        2,
		Url:       "https://github.com/f110/sandbox",
		CloneUrl:  "https://github.com/f110/sandbox.git",
		Name:      "sandbox",
		Private:   false,
		CreatedAt: time.Now(),
		UpdatedAt: nil,
	}

	t.Run("OpenedPullRequest", func(t *testing.T) {
		t.Parallel()

		setup := func(t *testing.T) (*mockDAO, *Api, *MockBuilder, http.ResponseWriter, *http.Request) {
			builder := &MockBuilder{}
			d := newMock()
			daos := dao.Options{
				Repository:        d.Repository,
				Task:              d.Task,
				TrustedUser:       d.TrustedUser,
				PermitPullRequest: d.PermitPullRequest,
			}

			s, err := NewApi("", builder, daos, nil, nil, "")
			require.NoError(t, err)
			body, err := os.ReadFile("testdata/pull_request_opened.json")
			require.NoError(t, err)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/webhook", bytes.NewReader(body))
			req.Header.Set("X-Github-Event", "pull_request")

			return d, s, builder, w, req
		}

		t.Run("NotTrustedUser", func(t *testing.T) {
			t.Parallel()

			mock, s, builder, w, req := setup(t)

			mockTransport := &MockTransport{res: []*http.Response{
				{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("{}")),
				},
			}}
			s.githubClient = github.NewClient(&http.Client{Transport: mockTransport})

			mock.TrustedUser.RegisterListByGithubId(2178441, nil, sql.ErrNoRows)
			mock.PermitPullRequest.RegisterListByRepositoryAndNumber("f110/ops", 28, nil, sql.ErrNoRows)

			s.handleWebHook(w, req)

			assert.False(t, builder.called)
			reqBody, err := io.ReadAll(mockTransport.req[0].Body)
			require.NoError(t, err)
			apiReq := &github.IssueComment{}
			err = json.Unmarshal(reqBody, apiReq)
			require.NoError(t, err)
			assert.Greater(t, len(apiReq.GetBody()), 10)
			assert.Contains(t, apiReq.GetBody(), AllowCommand)
		})

		t.Run("TrustedUser", func(t *testing.T) {
			t.Parallel()

			mock, s, builder, w, req := setup(t)

			mock.TrustedUser.RegisterListByGithubId(
				trustedUser.GithubId,
				[]*database.TrustedUser{trustedUser},
				nil,
			)
			mock.Repository.RegisterListByUrl(
				opsRepository.Url,
				[]*database.SourceRepository{opsRepository},
				nil,
			)

			mockTransport := &MockTransport{res: []*http.Response{
				{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"sha":"9697650793febd8884fe38a84365067624efacab"}`)),
				},
				{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"tree":[{"path":"build.star","sha":"buildstarsha"}]}`)),
				},
				{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(`job(
	name = "foo",
    event = ["pull_request"],
	command = "test",
	targets = ["//..."],
	platforms = ["linux_amd64"],
)`)),
				},
			}}
			s.githubClient = github.NewClient(&http.Client{Transport: mockTransport})

			s.handleWebHook(w, req)

			assert.True(t, builder.called)
			assert.Len(t, builder.jobs, 1)
		})

		t.Run("PermitPullRequest", func(t *testing.T) {
			t.Parallel()

			mock, s, builder, w, req := setup(t)

			mockTransport := &MockTransport{res: []*http.Response{
				{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"sha":"9697650793febd8884fe38a84365067624efacab"}`)),
				},
				{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"tree":[{"path":"build.star","sha":"buildstarsha"}]}`)),
				},
				{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(`job(
	name = "foo",
	event = ["pull_request"],
	command = "test",
	targets = ["//..."],
	platforms = ["linux_amd64"],
)`)),
				},
			}}
			s.githubClient = github.NewClient(&http.Client{Transport: mockTransport})

			mock.TrustedUser.RegisterListByGithubId(trustedUser.GithubId, nil, sql.ErrNoRows)
			mock.PermitPullRequest.RegisterListByRepositoryAndNumber("f110/ops", 28,
				[]*database.PermitPullRequest{{Id: 1, Repository: "f110/ops", Number: 28, CreatedAt: time.Now()}},
				nil,
			)
			mock.Repository.RegisterListByUrl(
				opsRepository.Url,
				[]*database.SourceRepository{opsRepository},
				nil,
			)

			s.handleWebHook(w, req)

			assert.True(t, builder.called)
			assert.Len(t, builder.jobs, 1)
		})
	})

	t.Run("SynchronizePullRequest", func(t *testing.T) {
		t.Parallel()

		mock := newMock()
		daos := dao.Options{
			Repository:        mock.Repository,
			Task:              mock.Task,
			TrustedUser:       mock.TrustedUser,
			PermitPullRequest: mock.PermitPullRequest,
		}

		mock.TrustedUser.RegisterListByGithubId(
			trustedUser.GithubId,
			[]*database.TrustedUser{trustedUser},
			nil,
		)
		mock.Repository.RegisterListByUrl(
			opsRepository.Url,
			[]*database.SourceRepository{opsRepository},
			nil,
		)

		builder := &MockBuilder{}

		mockTransport := &MockTransport{res: []*http.Response{
			{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"sha":"9697650793febd8884fe38a84365067624efacab"}`)),
			},
			{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"tree":[{"path":"build.star","sha":"buildstarsha"}]}`)),
			},
			{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`job(
	name = "foo",
    event = ["pull_request"],
	command = "test",
	targets = ["//..."],
	platforms = ["linux_amd64"],
)`)),
			},
		}}

		s, err := NewApi("", builder, daos, github.NewClient(&http.Client{Transport: mockTransport}), nil, "")
		require.NoError(t, err)
		body, err := os.ReadFile("testdata/pull_request_synchronize.json")
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/webhook", bytes.NewReader(body))
		req.Header.Set("X-Github-Event", "pull_request")
		s.handleWebHook(w, req)

		assert.True(t, builder.called)
	})

	t.Run("ClosedPullRequest", func(t *testing.T) {
		t.Parallel()

		mock := newMock()
		daos := dao.Options{
			Repository:        mock.Repository,
			Task:              mock.Task,
			TrustedUser:       mock.TrustedUser,
			PermitPullRequest: mock.PermitPullRequest,
		}

		mock.TrustedUser.RegisterListByGithubId(trustedUser.GithubId, nil, sql.ErrNoRows)
		mock.PermitPullRequest.RegisterListByRepositoryAndNumber("f110/sandbox", 2,
			[]*database.PermitPullRequest{{Id: 1, Repository: "f110/sandbox", Number: 2, CreatedAt: time.Now()}},
			nil,
		)

		builder := &MockBuilder{}

		s, err := NewApi("", builder, daos, nil, nil, "")
		require.NoError(t, err)
		body, err := os.ReadFile("testdata/pull_request_closed.json")
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/webhook", bytes.NewReader(body))
		req.Header.Set("X-Github-Event", "pull_request")
		s.handleWebHook(w, req)

		called := mock.PermitPullRequest.Called("Delete")
		assert.Len(t, called, 1)
		assert.Equal(t, int32(1), called[0].Args["id"])
	})

	t.Run("CommentIssue", func(t *testing.T) {
		t.Parallel()

		mock := newMock()
		daos := dao.Options{
			Repository:        mock.Repository,
			Task:              mock.Task,
			TrustedUser:       mock.TrustedUser,
			PermitPullRequest: mock.PermitPullRequest,
		}

		mock.TrustedUser.RegisterListByGithubId(trustedUser.GithubId, []*database.TrustedUser{trustedUser}, nil)
		mock.Repository.RegisterListByUrl(
			opsRepository.Url,
			[]*database.SourceRepository{opsRepository},
			nil,
		)

		builder := &MockBuilder{}

		mockTransport := &MockTransport{res: []*http.Response{
			{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			},
			{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"head":{"sha":"9697650793febd8884fe38a84365067624efacab"}}`)),
			},
			{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"sha":"9697650793febd8884fe38a84365067624efacab"}`)),
			},
			{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"tree":[{"path":"build.star","sha":"buildstarsha"}]}`)),
			},
			{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`job(
	name = "foo",
	event = ["pull_request"],
	command = "test",
	targets = ["//..."],
	platforms = ["linux_amd64"],
)`)),
			},
		}}

		s, err := NewApi("", builder, daos, github.NewClient(&http.Client{Transport: mockTransport}), nil, "")
		require.NoError(t, err)
		body, err := ioutil.ReadFile("testdata/issue_comment.json")
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/webhook", bytes.NewReader(body))
		req.Header.Set("X-Github-Event", "issue_comment")
		s.handleWebHook(w, req)

		assert.True(t, builder.called)
		called := mock.PermitPullRequest.Called("Create")
		require.Len(t, called, 1)
		assert.Equal(t, "f110/ops", called[0].Args["permitPullRequest"].(*database.PermitPullRequest).Repository)
	})

	t.Run("PublishRelease", func(t *testing.T) {
		t.Parallel()

		mock := newMock()
		daos := dao.Options{
			Repository:        mock.Repository,
			Task:              mock.Task,
			TrustedUser:       mock.TrustedUser,
			PermitPullRequest: mock.PermitPullRequest,
		}

		mock.Repository.RegisterListByUrl(
			sandboxRepository.Url,
			[]*database.SourceRepository{sandboxRepository},
			nil,
		)

		builder := &MockBuilder{}

		mockTransport := &MockTransport{res: []*http.Response{
			{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"object":{"sha":"abc0123"}}`)),
			},
			{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"sha":"9697650793febd8884fe38a84365067624efacab"}`)),
			},
			{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"tree":[{"path":"build.star","sha":"buildstarsha"}]}`)),
			},
			{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`job(
	name = "foo",
	event = ["release"],
	command = "test",
	targets = ["//..."],
	platforms = ["linux_amd64"],
)`)),
			},
		}}

		s, err := NewApi("", builder, daos, github.NewClient(&http.Client{Transport: mockTransport}), nil, "")
		require.NoError(t, err)
		body, err := os.ReadFile("testdata/release_published.json")
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/webhook", bytes.NewReader(body))
		req.Header.Set("X-Github-Event", "release")
		s.handleWebHook(w, req)

		assert.True(t, builder.called)
	})
}
