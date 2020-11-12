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

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/go-github/v32/github"
	"github.com/stretchr/testify/assert"

	"go.f110.dev/mono/lib/logger"
	"go.f110.dev/mono/tools/build/pkg/database"
	"go.f110.dev/mono/tools/build/pkg/database/dao"
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

func TestGithubWebHook(t *testing.T) {
	logger.SetLogLevel("warn")
	logger.Init()

	t.Run("OpenedPullRequest", func(t *testing.T) {
		t.Parallel()

		setup := func(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *Api, *MockBuilder, http.ResponseWriter, *http.Request) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}

			builder := &MockBuilder{}

			s, err := NewApi("", builder, nil, dao.NewOptions(db), nil)
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

			return db, mock, s, builder, w, req
		}

		t.Run("NotTrustedUser", func(t *testing.T) {
			t.Parallel()

			db, mock, s, builder, w, req := setup(t)
			defer db.Close()

			mockTransport := &MockTransport{res: &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(strings.NewReader("{}")),
			}}
			s.githubClient = github.NewClient(&http.Client{Transport: mockTransport})

			mock.ExpectQuery("select").WithArgs(2178441).WillReturnError(sql.ErrNoRows)
			mock.ExpectQuery(`select .+ from permit_pull_request`).WithArgs("f110/ops", 28).WillReturnError(sql.ErrNoRows)

			s.handleWebHook(w, req)

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
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

			db, mock, s, builder, w, req := setup(t)
			defer db.Close()

			// TrustedUser
			mock.ExpectQuery("select").WithArgs(2178441).WillReturnRows(
				sqlmock.NewRows([]string{"id", "github_id", "username", "created_at", "updated_at"}).AddRow(1, 1, "octocat", time.Now(), nil),
			)
			// SourceRepository
			mock.ExpectQuery("select").WithArgs("https://github.com/f110/ops").
				WillReturnRows(
					sqlmock.NewRows(
						[]string{"id", "url", "clone_url", "name", "private", "created_at", "updated_at"},
					).AddRow(1, "https://github.com/f110/ops", "https://github.com/f110/ops.git", "ops", 0, time.Now(), nil),
				)
			// Job
			mock.ExpectQuery("select .+ from job").WithArgs(1).
				WillReturnRows(
					sqlmock.NewRows(
						[]string{"id", "repository_id", "command", "target", "active", "all_revision", "github_status",
							"cpu_limit", "memory_limit", "exclusive", "sync", "config_name", "bazel_version", "created_at", "updated_at"},
					).AddRow(1, 1, "test", "//...", 1, 1, 0, 1, 1, 1, 1, "test", "3.5.0", time.Now(), time.Now()),
				)
			// SourceRepository
			mock.ExpectQuery("SELECT").WithArgs(1).
				WillReturnRows(
					sqlmock.NewRows([]string{"id", "url", "clone_url", "name", "private", "created_at", "updated_at"}).AddRow(1, "https://github.com/f110/ops", "https://github.com/f110/ops.git", "ops", 0, time.Now(), nil),
				)

			s.handleWebHook(w, req)

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
			assert.True(t, builder.called)
			assert.Len(t, builder.jobs, 1)
		})

		t.Run("PermitPullRequest", func(t *testing.T) {
			t.Parallel()

			db, mock, s, builder, w, req := setup(t)
			defer db.Close()

			mockTransport := &MockTransport{res: &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(strings.NewReader("{}")),
			}}
			s.githubClient = github.NewClient(&http.Client{Transport: mockTransport})

			// TrustedUser will not return any row.
			mock.ExpectQuery("select").WithArgs(2178441).WillReturnError(sql.ErrNoRows)
			mock.ExpectQuery(`select .+ from permit_pull_request`).WithArgs("f110/ops", 28).WillReturnRows(
				sqlmock.NewRows([]string{"id", "repository", "number", "created_at", "updated_at"}).
					AddRow(1, "f110/ops", 28, time.Now(), time.Now()),
			)
			// SourceRepository will return a row
			mock.ExpectQuery("select").WithArgs("https://github.com/f110/ops").
				WillReturnRows(
					sqlmock.NewRows([]string{"id", "url", "clone_url", "name", "private", "created_at", "updated_at"}).AddRow(1, "https://github.com/f110/ops", "https://github.com/f110/ops.git", "ops", 1, time.Now(), nil),
				)
			// Job will return a row
			mock.ExpectQuery(`select .+ from job`).WithArgs(1).
				WillReturnRows(
					sqlmock.NewRows([]string{"id", "repository_id", "command", "target", "active", "all_revision", "github_status", "cpu_limit", "memory_limit", "exclusive", "sync", "config_name", "bazel_version", "created_at", "updated_at"}).
						AddRow(1, 1, "test", "//...", 1, 1, 0, "100m", "1024Mi", 1, 1, "test", "3.5.0", time.Now(), time.Now()),
				)
			mock.ExpectQuery(`SELECT .+ FROM .source_repository`).WithArgs(1).WillReturnRows(
				sqlmock.NewRows([]string{"id", "url", "clone_url", "name", "private", "created_at", "updated_at"}).
					AddRow(1, "https://github.com/f110/ops", "https://github.com/f110/ops.git", "ops", 0, time.Now(), time.Now()),
			)

			s.handleWebHook(w, req)

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
			assert.True(t, builder.called)
			assert.Len(t, builder.jobs, 1)
		})
	})

	t.Run("SynchronizePullRequest", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		mock.ExpectQuery("select").WithArgs(2178441).WillReturnRows(
			sqlmock.NewRows([]string{"id", "github_id", "username", "created_at", "updated_at"}).AddRow(1, 2178441, "octocat", time.Now(), nil))
		// List Repository by url
		mock.ExpectQuery("select").WithArgs("https://github.com/f110/ops").
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "url", "clone_url", "name", "private", "created_at", "updated_at"}).AddRow(1, "https://github.com/f110/ops", "https://github.com/f110/ops.git", "ops", 0, time.Now(), nil),
			)
		// Job
		mock.ExpectQuery("select .+ from job").WithArgs(1).
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "repository_id", "command", "target", "active", "all_revision", "github_status", "cpu_limit", "memory_limit", "exclusive", "sync", "config_name", "bazel_version", "created_at", "updated_at"}).
					AddRow(1, 1, "test", "//...", 1, 1, 0, "100m", "1024Mi", 1, 1, "test", "3.5.0", time.Now(), time.Now()),
			)
		mock.ExpectQuery("SELECT").WithArgs(1).
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "url", "clone_url", "name", "private", "created_at", "updated_at"}).AddRow(1, "https://github.com/f110/ops", "https://github.com/f110/ops.git", "ops", 1, time.Now(), nil),
			)

		builder := &MockBuilder{}

		s, err := NewApi("", builder, nil, dao.NewOptions(db), nil)
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

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		assert.True(t, builder.called)
	})

	t.Run("ClosedPullRequest", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		// PermitPullRequest
		mock.ExpectQuery("select .+ from permit_pull_request").WithArgs("f110/sandbox", 2).
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "repository", "number", "created_at", "updated_at"}).AddRow(1, "f110/sandbox", 2, time.Now(), nil),
			)
		// Delete PermitPullRequest
		mock.ExpectExec("DELETE FROM `permit_pull_request`").WithArgs(1).WillReturnResult(sqlmock.NewResult(0, 1))

		builder := &MockBuilder{}

		s, err := NewApi("", builder, nil, dao.NewOptions(db), nil)
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

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("CommentIssue", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		// TrustedUser will return a row.
		mock.ExpectQuery("select").WithArgs(2178441).WillReturnRows(sqlmock.NewRows([]string{"id", "github_id", "username", "created_at", "updated_at"}).AddRow(1, 1, "octocat", time.Now(), nil))
		// Insert to PermitPullRequest
		mock.ExpectExec("INSERT INTO `permit_pull_request`").WillReturnResult(sqlmock.NewResult(1, 1))
		// List SourceRepository by url
		mock.ExpectQuery(`select .+ from source_repository`).WithArgs("https://github.com/f110/ops").WillReturnRows(
			sqlmock.NewRows([]string{"id", "url", "clone_url", "name", "private", "created_at", "updated_at"}).
				AddRow(1, "https://github.com/f110/ops", "https://github.com/f110/ops.git", "ops", 0, time.Now(), time.Now()),
		)
		// List job by repository_id
		mock.ExpectQuery(`select .+ from job`).WithArgs(1).WillReturnRows(
			sqlmock.NewRows([]string{"id", "repository_id", "command", "target", "active", "all_revision", "github_status", "cpu_limit", "memory_limit", "exclusive", "sync", "config_name", "bazel_version", "created_at", "updated_at"}).
				AddRow(1, 1, "test", "//...", 1, 1, 1, "1000m", "1024Mi", 1, 1, "test", "3.5.0", time.Now(), time.Now()),
		)
		mock.ExpectQuery(`SELECT .+ FROM .source_repository`).WithArgs(1).WillReturnRows(
			sqlmock.NewRows([]string{"id", "url", "clone_url", "name", "private", "created_at", "updated_at"}).
				AddRow(1, "https://github.com/f110/ops", "https://github.com/f110/ops.git", "ops", 0, time.Now(), time.Now()),
		)

		builder := &MockBuilder{}

		mockTransport := &MockTransport{res: &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(strings.NewReader("{}")),
		}}

		s, err := NewApi("", builder, nil, dao.NewOptions(db), github.NewClient(&http.Client{Transport: mockTransport}))
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

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		assert.True(t, builder.called)
	})
}
