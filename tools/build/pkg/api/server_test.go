package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/tools/build/pkg/database"
	"go.f110.dev/mono/tools/build/pkg/database/dao"
	"go.f110.dev/mono/tools/build/pkg/database/dao/daotest"
)

type MockBuilder struct {
	jobs   []*database.Job
	called bool
}

func (m *MockBuilder) Build(_ context.Context, job *database.Job, revision, command, target, via string) (*database.Task, error) {
	m.jobs = append(m.jobs, job)
	m.called = true
	return &database.Task{}, nil
}

type MockTransport struct {
	req *http.Request
	res *http.Response
}

func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.req = req
	return m.res, nil
}

func (m *MockTransport) RequestBody() ([]byte, error) {
	reqBody, err := ioutil.ReadAll(m.req.Body)
	if err != nil {
		return nil, err
	}
	return reqBody, nil
}

type mockDAO struct {
	Repository        *daotest.SourceRepository
	Job               *daotest.Job
	Task              *daotest.Task
	TrustedUser       *daotest.TrustedUser
	PermitPullRequest *daotest.PermitPullRequest
}

func newMock() *mockDAO {
	return &mockDAO{
		Repository:        daotest.NewSourceRepository(),
		Job:               daotest.NewJob(),
		Task:              daotest.NewTask(),
		TrustedUser:       daotest.NewTrustedUser(),
		PermitPullRequest: daotest.NewPermitPullRequest(),
	}
}

func TestGithubWebHook(t *testing.T) {
	logger.SetLogLevel("warn")
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
	testJob := &database.Job{
		Id:           1,
		RepositoryId: opsRepository.Id,
		Repository:   opsRepository,
		Command:      "test",
		Target:       "//...",
		Active:       true,
		AllRevision:  true,
		GithubStatus: false,
		CpuLimit:     "300m",
		MemoryLimit:  "512Mi",
		Exclusive:    true,
		Sync:         true,
		ConfigName:   "test",
		BazelVersion: "3.5.0",
		CreatedAt:    time.Now(),
	}
	sandboxJob := &database.Job{
		Id:           2,
		RepositoryId: sandboxRepository.Id,
		Repository:   sandboxRepository,
		Command:      "test",
		Target:       "//...",
		Active:       true,
		AllRevision:  true,
		GithubStatus: false,
		CpuLimit:     "300m",
		MemoryLimit:  "512Mi",
		Exclusive:    true,
		Sync:         true,
		ConfigName:   "test",
		BazelVersion: "3.5.0",
		CreatedAt:    time.Now(),
	}

	t.Run("OpenedPullRequest", func(t *testing.T) {
		t.Parallel()

		setup := func(t *testing.T) (*mockDAO, *Api, *MockBuilder, http.ResponseWriter, *http.Request) {
			builder := &MockBuilder{}
			d := newMock()
			daos := dao.Options{
				Repository:        d.Repository,
				Job:               d.Job,
				Task:              d.Task,
				TrustedUser:       d.TrustedUser,
				PermitPullRequest: d.PermitPullRequest,
			}

			s, err := NewApi("", builder, nil, daos, nil)
			if err != nil {
				t.Fatal(err)
			}
			body, err := ioutil.ReadFile("testdata/pull_request_opened.json")
			if err != nil {
				t.Fatal(err)
			}

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/webhook", bytes.NewReader(body))
			req.Header.Set("X-Github-Event", "pull_request")

			return d, s, builder, w, req
		}

		t.Run("NotTrustedUser", func(t *testing.T) {
			t.Parallel()

			mock, s, builder, w, req := setup(t)

			mockTransport := &MockTransport{res: &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(strings.NewReader("{}")),
			}}
			s.githubClient = github.NewClient(&http.Client{Transport: mockTransport})

			mock.TrustedUser.RegisterListByGithubId(2178441, nil, sql.ErrNoRows)
			mock.PermitPullRequest.RegisterListByRepositoryAndNumber("f110/ops", 28, nil, sql.ErrNoRows)

			s.handleWebHook(w, req)

			assert.False(t, builder.called)
			reqBody, err := mockTransport.RequestBody()
			if err != nil {
				t.Fatal(err)
			}
			apiReq := &github.IssueComment{}
			if err := json.Unmarshal(reqBody, apiReq); err != nil {
				t.Fatal(err)
			}
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
			mock.Job.RegisterListBySourceRepositoryId(
				opsRepository.Id,
				[]*database.Job{testJob},
				nil,
			)

			s.handleWebHook(w, req)

			assert.True(t, builder.called)
			assert.Len(t, builder.jobs, 1)
		})

		t.Run("PermitPullRequest", func(t *testing.T) {
			t.Parallel()

			mock, s, builder, w, req := setup(t)

			mockTransport := &MockTransport{res: &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(strings.NewReader("{}")),
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
			mock.Job.RegisterListBySourceRepositoryId(
				opsRepository.Id,
				[]*database.Job{testJob},
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
			Job:               mock.Job,
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
		mock.Job.RegisterListBySourceRepositoryId(
			opsRepository.Id,
			[]*database.Job{testJob},
			nil,
		)

		builder := &MockBuilder{}

		s, err := NewApi("", builder, nil, daos, nil)
		if err != nil {
			t.Fatal(err)
		}
		body, err := ioutil.ReadFile("testdata/pull_request_synchronize.json")
		if err != nil {
			t.Fatal(err)
		}

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
			Job:               mock.Job,
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

		s, err := NewApi("", builder, nil, daos, nil)
		if err != nil {
			t.Fatal(err)
		}
		body, err := ioutil.ReadFile("testdata/pull_request_closed.json")
		if err != nil {
			t.Fatal(err)
		}

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
			Job:               mock.Job,
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
		mock.Job.RegisterListBySourceRepositoryId(
			opsRepository.Id,
			[]*database.Job{testJob},
			nil,
		)

		builder := &MockBuilder{}

		mockTransport := &MockTransport{res: &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(strings.NewReader("{}")),
		}}

		s, err := NewApi("", builder, nil, daos, github.NewClient(&http.Client{Transport: mockTransport}))
		if err != nil {
			t.Fatal(err)
		}
		body, err := ioutil.ReadFile("testdata/issue_comment.json")
		if err != nil {
			t.Fatal(err)
		}

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
			Job:               mock.Job,
			Task:              mock.Task,
			TrustedUser:       mock.TrustedUser,
			PermitPullRequest: mock.PermitPullRequest,
		}

		mock.Repository.RegisterListByUrl(
			sandboxRepository.Url,
			[]*database.SourceRepository{sandboxRepository},
			nil,
		)
		mock.Job.RegisterListBySourceRepositoryId(
			sandboxRepository.Id,
			[]*database.Job{sandboxJob},
			nil,
		)

		builder := &MockBuilder{}

		mockTransport := &MockTransport{res: &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(strings.NewReader(`{"object":{"sha":"abc0123"}}`)),
		}}

		s, err := NewApi("", builder, nil, daos, github.NewClient(&http.Client{Transport: mockTransport}))
		if err != nil {
			t.Fatal(err)
		}
		body, err := ioutil.ReadFile("testdata/release_published.json")
		if err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/webhook", bytes.NewReader(body))
		req.Header.Set("X-Github-Event", "release")
		s.handleWebHook(w, req)

		assert.True(t, builder.called)
	})
}
