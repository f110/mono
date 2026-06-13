package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/database/dao/daotest"
	"go.f110.dev/mono/go/build/webhook"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/testing/assertion"
)

// TestWebhookEndpoint verifies that the /webhook HTTP endpoint persists a
// PENDING github_event row and returns 200 without touching the Builder.
// All business logic lives in the eventbus reconcilers (tested separately),
// so the api layer only needs to confirm the wiring.
func TestWebhookEndpoint(t *testing.T) {
	logger.SetLogLevel("debug")
	slogger.Init()

	const repoURL = "https://github.com/f110/ops"

	repo := daotest.NewSourceRepository()
	repo.RegisterListByUrl(repoURL, []*database.SourceRepository{{Id: 1, Url: repoURL, Name: "ops", CreatedAt: time.Now()}}, nil)
	ghEvent := daotest.NewGithubEvent()
	daos := dao.Options{
		Repository:        repo,
		Task:              daotest.NewTask(),
		TrustedUser:       daotest.NewTrustedUser(),
		PermitPullRequest: daotest.NewPermitPullRequest(),
		GithubEvent:       ghEvent,
	}

	notifier := webhook.NewNotifier()
	s, err := NewApi("", nil, daos, nil, nil, nil, "", notifier, nil)
	assertion.MustNoError(t, err)

	body, err := os.ReadFile("testdata/pull_request_opened.json")
	assertion.MustNoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/webhook", bytes.NewReader(body))
	req.Header.Set("X-Github-Event", "pull_request")
	req.Header.Set("X-Github-Delivery", "test-delivery-1")
	s.Server.Handler.ServeHTTP(w, req)

	assertion.Equal(t, w.Code, http.StatusOK)

	called := ghEvent.Called("Create")
	assertion.MustLen(t, called, 1)
	ev := called[0].Args["githubEvent"].(*database.GithubEvent)
	assertion.Equal(t, ev.EventType, "pull_request")
	assertion.Equal(t, ev.Action, "opened")
	assertion.Equal(t, ev.DeliveryId, "test-delivery-1")
	assertion.Equal(t, ev.State, database.GithubEventStatePending)
}
