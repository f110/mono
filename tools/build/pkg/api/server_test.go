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

func (m *MockBuilder) Build(_ context.Context, job *database.Job, revision, via string) (*database.Task, error) {
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

			s, err := NewApi("", builder, nil, dao.NewOptions(db), 0, 0, "")
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

			mock.ExpectQuery("SELECT").WithArgs(2178441).WillReturnError(sql.ErrNoRows)
			mock.ExpectQuery("SELECT").WithArgs("f110/ops", 28).WillReturnError(sql.ErrNoRows)

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
			mock.ExpectQuery("SELECT").WithArgs(2178441).WillReturnRows(sqlmock.NewRows([]string{"id", "username", "created_at", "updated_at"}).AddRow(1, "octocat", time.Now(), nil))
			// SourceRepository
			mock.ExpectQuery("SELECT").WithArgs("https://github.com/f110/ops").
				WillReturnRows(
					sqlmock.NewRows([]string{"id", "clone_url", "name", "created_at", "updated_at"}).AddRow(1, "https://github.com/f110/ops.git", "ops", time.Now(), nil),
				)
			// Job
			mock.ExpectQuery("SELECT .+ FROM `job`").WithArgs(1).
				WillReturnRows(
					sqlmock.NewRows([]string{"id", "command", "target", "active", "all_revision", "github_status"}).AddRow(1, "test", "//...", 1, 1, 0),
				)
			// SourceRepository
			mock.ExpectQuery("SELECT").WithArgs(1).
				WillReturnRows(
					sqlmock.NewRows([]string{"url", "clone_url", "name", "created_at", "updated_at"}).AddRow("https://github.com/f110/ops", "https://github.com/f110/ops.git", "ops", time.Now(), nil),
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

			// TrustedUser will not return any row.
			mock.ExpectQuery("SELECT").WithArgs(2178441).WillReturnError(sql.ErrNoRows)
			// PermitPullRequest will return a row.
			mock.ExpectQuery("SELECT").WithArgs("f110/ops", 28).WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(1, time.Now(), nil))
			// SourceRepository will return a row
			mock.ExpectQuery("SELECT").WithArgs("https://github.com/f110/ops").
				WillReturnRows(
					sqlmock.NewRows([]string{"id", "clone_url", "name", "created_at", "updated_at"}).AddRow(1, "https://github.com/f110/ops.git", "ops", time.Now(), nil),
				)
			// Job will return a row
			mock.ExpectQuery("SELECT .+ FROM `job`").WithArgs(1).
				WillReturnRows(
					sqlmock.NewRows([]string{"id", "command", "target", "active", "all_revision", "github_status"}).AddRow(1, "test", "//...", 1, 1, 0),
				)
			mock.ExpectQuery("SELECT").WithArgs(1).
				WillReturnRows(
					sqlmock.NewRows([]string{"url", "clone_url", "name", "created_at", "updated_at"}).AddRow("https://github.com/f110/ops", "https://github.com/f110/ops.git", "ops", time.Now(), nil),
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

		mock.ExpectQuery("SELECT").WithArgs(2178441).WillReturnRows(sqlmock.NewRows([]string{"id", "username", "created_at", "updated_at"}).AddRow(1, "octocat", time.Now(), nil))
		mock.ExpectQuery("SELECT").WithArgs("https://github.com/f110/ops").
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "clone_url", "name", "created_at", "updated_at"}).AddRow(1, "https://github.com/f110/ops.git", "ops", time.Now(), nil),
			)
		// Job
		mock.ExpectQuery("SELECT .+ FROM `job`").WithArgs(1).
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "command", "target", "active", "all_revision", "github_status"}).AddRow(1, "test", "//...", 1, 1, 0),
			)
		mock.ExpectQuery("SELECT").WithArgs(1).
			WillReturnRows(
				sqlmock.NewRows([]string{"url", "clone_url", "name", "created_at", "updated_at"}).AddRow("https://github.com/f110/ops", "https://github.com/f110/ops.git", "ops", time.Now(), nil),
			)

		builder := &MockBuilder{}

		s, err := NewApi("", builder, nil, dao.NewOptions(db), 0, 0, "")
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

	t.Run("CommentIssue", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		// TrustedUser will return a row.
		mock.ExpectQuery("SELECT").WithArgs(2178441).WillReturnRows(sqlmock.NewRows([]string{"id", "username", "created_at", "updated_at"}).AddRow(1, "octocat", time.Now(), nil))
		// Insert to PermitPullRequest
		mock.ExpectExec("INSERT INTO `permit_pull_request`").WillReturnResult(sqlmock.NewResult(1, 1))

		s, err := NewApi("", nil, nil, dao.NewOptions(db), 0, 0, "")
		if err != nil {
			t.Fatal(err)
		}
		s.githubClient = &github.Client{}
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
	})
}
