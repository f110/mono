package webhook

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestHandler(t *testing.T) {
	logger.SetLogLevel("debug")
	slogger.Init()

	// wantInsertedEv captures the subset of fields a successful insert must
	// have. Unset to assert no insert happened.
	type wantInsert struct {
		eventType  string
		action     string
		deliveryID string
		state      database.GithubEventState
	}

	const opsURL = "https://github.com/f110/ops"

	cases := []struct {
		name       string
		eventType  string
		deliveryID string
		body       []byte
		setup      func(d *testDAO)
		wantStatus int
		wantInsert *wantInsert
		wantKick   bool
	}{
		{
			name:       "valid pull_request delivery is persisted, kicks the scheduler, returns 200",
			eventType:  "pull_request",
			deliveryID: "abc-123",
			body:       loadPayload(t, "pull_request_opened.json"),
			setup: func(d *testDAO) {
				d.Repository.RegisterListByUrl(opsURL, []*database.SourceRepository{repoFixture(opsURL, "ops")}, nil)
			},
			wantStatus: http.StatusOK,
			wantInsert: &wantInsert{
				eventType:  "pull_request",
				action:     "opened",
				deliveryID: "abc-123",
				state:      database.GithubEventStatePending,
			},
			wantKick: true,
		},
		{
			name:       "delivery from an unmanaged repository is dropped and returns 200",
			eventType:  "pull_request",
			deliveryID: "abc-124",
			body:       loadPayload(t, "pull_request_opened.json"),
			setup: func(d *testDAO) {
				d.Repository.RegisterListByUrl(opsURL, nil, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "delivery with no repository field is dropped and returns 200",
			eventType:  "ping",
			deliveryID: "abc-125",
			body:       []byte(`{"zen":"hi"}`),
			wantStatus: http.StatusOK,
		},
		{
			name:       "request missing both webhook headers is rejected with 400",
			body:       []byte("{}"),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "request with only X-GitHub-Event is rejected with 400",
			eventType:  "pull_request",
			body:       []byte("{}"),
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newTestDAO()
			if tc.setup != nil {
				tc.setup(d)
			}
			notifier := NewNotifier()
			kick := make(chan struct{}, 1)
			notifier.Register(kick)
			h := NewHandler(d.toOptions(), notifier)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "http://localhost/webhook", bytes.NewReader(tc.body))
			if tc.eventType != "" {
				req.Header.Set("X-GitHub-Event", tc.eventType)
			}
			if tc.deliveryID != "" {
				req.Header.Set("X-GitHub-Delivery", tc.deliveryID)
			}
			h.ServeHTTP(w, req)

			assertion.Equal(t, w.Code, tc.wantStatus)

			called := d.GithubEvent.Called("Create")
			if tc.wantInsert == nil {
				assertion.MustLen(t, called, 0)
				return
			}
			assertion.MustLen(t, called, 1)
			ev := called[0].Args["githubEvent"].(*database.GithubEvent)
			assertion.Equal(t, ev.EventType, tc.wantInsert.eventType)
			assertion.Equal(t, ev.Action, tc.wantInsert.action)
			assertion.Equal(t, ev.DeliveryId, tc.wantInsert.deliveryID)
			assertion.Equal(t, ev.State, tc.wantInsert.state)

			select {
			case <-kick:
				if !tc.wantKick {
					t.Fatalf("expected no kick but the notifier was signalled")
				}
			default:
				if tc.wantKick {
					t.Fatalf("expected the notifier to be kicked but no signal arrived")
				}
			}
		})
	}
}
