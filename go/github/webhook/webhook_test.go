package webhook

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestMain(m *testing.M) {
	logger.SetLogLevel("debug")
	slogger.Init()
	os.Exit(m.Run())
}

type fakeStore struct {
	mu        sync.Mutex
	events    map[int32]*Event
	nextId    int32
	createErr error
	listCalls int
}

var _ Store = (*fakeStore)(nil)

func newFakeStore(events ...*Event) *fakeStore {
	s := &fakeStore{events: make(map[int32]*Event)}
	for _, ev := range events {
		s.events[ev.Id] = ev
		s.nextId = max(s.nextId, ev.Id)
	}
	return s
}

func (s *fakeStore) Create(_ context.Context, ev *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createErr != nil {
		return s.createErr
	}
	for _, v := range s.events {
		if v.DeliveryId == ev.DeliveryId {
			return ErrDuplicateDelivery
		}
	}
	s.nextId++
	ev.Id = s.nextId
	c := *ev
	s.events[c.Id] = &c
	return nil
}

func (s *fakeStore) ListByState(_ context.Context, states ...State) ([]*Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listCalls++
	var res []*Event
	for _, v := range s.events {
		for _, st := range states {
			if v.State == st {
				c := *v
				res = append(res, &c)
			}
		}
	}
	return res, nil
}

func (s *fakeStore) Claim(_ context.Context, ev *Event) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.events[ev.Id]
	if v.State != StatePending && v.State != StateFailed {
		return false, nil
	}
	v.State = StateProcessing
	return true, nil
}

func (s *fakeStore) Update(_ context.Context, ev *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.events[ev.Id]
	v.State = ev.State
	v.Status = ev.Status
	v.LastError = ev.LastError
	return nil
}

func (s *fakeStore) get(id int32) *Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := *s.events[id]
	return &c
}

const testPayload = `{"ref":"refs/heads/master"}`

type fakePayload struct {
	Ref string `json:"ref"`
}

type fakeReconciler struct {
	eventType string
	fn        func(ev *Event) error
	called    int
	payload   *fakePayload
}

var _ Reconciler[fakePayload] = (*fakeReconciler)(nil)

func (r *fakeReconciler) EventType() string { return r.eventType }

func (r *fakeReconciler) Reconcile(_ context.Context, ev *Event, payload *fakePayload) error {
	r.called++
	r.payload = payload
	if r.fn == nil {
		return nil
	}
	return r.fn(ev)
}

func TestEventProcessorServeHTTP(t *testing.T) {
	const body = `{"action":"opened","repository":{"html_url":"https://github.com/f110/ops"}}`

	cases := []struct {
		name       string
		eventType  string
		deliveryID string
		body       string
		existing   []*Event
		createErr  error
		wantStatus int
		wantAction string
		wantStored bool
	}{
		{
			name:       "valid delivery is persisted and queued",
			eventType:  "pull_request",
			deliveryID: "abc-123",
			body:       body,
			wantStatus: http.StatusOK,
			wantAction: "opened",
			wantStored: true,
		},
		{
			name:       "delivery with no repository field is persisted",
			eventType:  "ping",
			deliveryID: "abc-125",
			body:       `{"zen":"hi"}`,
			wantStatus: http.StatusOK,
			wantStored: true,
		},
		{
			name:       "redelivery is accepted without storing again",
			eventType:  "pull_request",
			deliveryID: "abc-126",
			body:       body,
			existing:   []*Event{{Id: 1, DeliveryId: "abc-126", State: StateSucceeded}},
			wantStatus: http.StatusOK,
		},
		{
			name:       "failure of the store returns 500",
			eventType:  "pull_request",
			deliveryID: "abc-127",
			body:       body,
			createErr:  xerrors.New("db down"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "request missing both webhook headers is rejected",
			body:       "{}",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "request with only X-GitHub-Event is rejected",
			eventType:  "pull_request",
			body:       "{}",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore(tc.existing...)
			store.createErr = tc.createErr
			h := NewEventProcessor(store, time.Minute, time.Hour)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "http://localhost/webhook", bytes.NewReader([]byte(tc.body)))
			if tc.eventType != "" {
				req.Header.Set("X-GitHub-Event", tc.eventType)
			}
			if tc.deliveryID != "" {
				req.Header.Set("X-GitHub-Delivery", tc.deliveryID)
			}
			h.ServeHTTP(w, req)

			assertion.Equal(t, w.Code, tc.wantStatus)
			if !tc.wantStored {
				assertion.Len(t, store.events, len(tc.existing))
				assertion.Len(t, h.queue, 0)
				return
			}
			events, err := store.ListByState(context.Background(), StatePending)
			assertion.MustNoError(t, err)
			assertion.MustLen(t, events, 1)
			assertion.Equal(t, events[0].EventType, tc.eventType)
			assertion.Equal(t, events[0].Action, tc.wantAction)
			assertion.Equal(t, events[0].DeliveryId, tc.deliveryID)
			assertion.MustLen(t, h.queue, 1)
			queued := <-h.queue
			assertion.Equal(t, queued.Id, events[0].Id)
		})
	}
}

func TestEventProcessorRun(t *testing.T) {
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	newEventProcessor := func(store Store, recs ...*fakeReconciler) *EventProcessor {
		s := NewEventProcessor(store, time.Minute, time.Hour)
		for _, v := range recs {
			s.Register[fakePayload](v)
		}
		s.now = func() time.Time { return now }
		return s
	}

	t.Run("succeeded", func(t *testing.T) {
		store := newFakeStore(&Event{Id: 1, EventType: "push", Payload: []byte(testPayload), State: StatePending, CreatedAt: now})
		rec := &fakeReconciler{eventType: "push", fn: func(ev *Event) error {
			assertion.Equal(t, ev.State, StateProcessing)
			return ev.WriteStatus(map[string]bool{"done": true})
		}}
		newEventProcessor(store, rec).runOnce(context.Background())

		ev := store.get(1)
		assertion.Equal(t, rec.called, 1)
		assertion.Equal(t, ev.State, StateSucceeded)
		assertion.Equal(t, rec.payload.Ref, "refs/heads/master")
		assertion.Equal(t, string(ev.Status), `{"done":true}`)
	})

	t.Run("reconcile error marks failed and is retried", func(t *testing.T) {
		store := newFakeStore(&Event{Id: 1, EventType: "push", Payload: []byte(testPayload), State: StatePending, CreatedAt: now})
		calls := 0
		rec := &fakeReconciler{eventType: "push", fn: func(ev *Event) error {
			calls++
			if calls == 1 {
				return xerrors.New("boom")
			}
			return nil
		}}
		s := newEventProcessor(store, rec)
		s.runOnce(context.Background())
		ev := store.get(1)
		assertion.Equal(t, ev.State, StateFailed)
		assertion.Equal(t, ev.LastError, "boom")

		s.runOnce(context.Background())
		ev = store.get(1)
		assertion.Equal(t, ev.State, StateSucceeded)
		assertion.Equal(t, ev.LastError, "")
	})

	t.Run("state set by the reconciler is preserved", func(t *testing.T) {
		store := newFakeStore(&Event{Id: 1, EventType: "push", Payload: []byte(testPayload), State: StatePending, CreatedAt: now})
		rec := &fakeReconciler{eventType: "push", fn: func(ev *Event) error {
			ev.State = StateSkipped
			return nil
		}}
		newEventProcessor(store, rec).runOnce(context.Background())
		assertion.Equal(t, store.get(1).State, StateSkipped)
	})

	t.Run("event claimed by another worker is not reconciled", func(t *testing.T) {
		store := newFakeStore(&Event{Id: 1, EventType: "push", Payload: []byte(testPayload), State: StatePending, CreatedAt: now})
		rec := &fakeReconciler{eventType: "push"}
		s := newEventProcessor(store, rec)
		events, err := store.ListByState(context.Background(), StatePending)
		assertion.MustNoError(t, err)
		ok, err := store.Claim(context.Background(), events[0])
		assertion.MustNoError(t, err)
		assertion.True(t, ok)

		s.process(context.Background(), events[0])
		assertion.Equal(t, rec.called, 0)
		assertion.Equal(t, store.get(1).State, StateProcessing)
	})

	t.Run("unknown event type is skipped", func(t *testing.T) {
		store := newFakeStore(&Event{Id: 1, EventType: "star", State: StatePending, CreatedAt: now})
		newEventProcessor(store).runOnce(context.Background())
		ev := store.get(1)
		assertion.Equal(t, ev.State, StateSkipped)
		assertion.Equal(t, ev.LastError, "no reconciler registered for event_type=star")
	})

	t.Run("old event is expired", func(t *testing.T) {
		store := newFakeStore(&Event{Id: 1, EventType: "push", Payload: []byte(testPayload), State: StateFailed, CreatedAt: now.Add(-2 * time.Hour)})
		rec := &fakeReconciler{eventType: "push"}
		newEventProcessor(store, rec).runOnce(context.Background())
		assertion.Equal(t, rec.called, 0)
		assertion.Equal(t, store.get(1).State, StateExpired)
	})

	t.Run("only stuck PROCESSING event is recovered", func(t *testing.T) {
		store := newFakeStore(
			&Event{Id: 1, EventType: "push", Payload: []byte(testPayload), State: StateProcessing, CreatedAt: now, UpdatedAt: now.Add(-stuckProcessingTimeout - time.Second)},
			&Event{Id: 2, EventType: "push", Payload: []byte(testPayload), State: StateProcessing, CreatedAt: now, UpdatedAt: now.Add(-time.Minute)},
		)
		rec := &fakeReconciler{eventType: "push"}
		newEventProcessor(store, rec).runOnce(context.Background())
		assertion.Equal(t, rec.called, 1)
		assertion.Equal(t, store.get(1).State, StateSucceeded)
		assertion.Equal(t, store.get(2).State, StateProcessing)
	})
}

func TestEventProcessorRunInvalidPayload(t *testing.T) {
	now := time.Now()
	store := newFakeStore(&Event{Id: 1, EventType: "push", Payload: []byte("{"), State: StatePending, CreatedAt: now})
	rec := &fakeReconciler{eventType: "push"}
	s := NewEventProcessor(store, time.Minute, time.Hour)
	s.Register[fakePayload](rec)
	s.runOnce(context.Background())

	assertion.Equal(t, rec.called, 0)
	assertion.Equal(t, store.get(1).State, StateFailed)
}

func TestEventProcessorRunProcessesReceivedEventWithoutPolling(t *testing.T) {
	store := newFakeStore()
	done := make(chan struct{})
	rec := &fakeReconciler{eventType: "push", fn: func(ev *Event) error {
		close(done)
		return nil
	}}
	p := NewEventProcessor(store, time.Hour, time.Hour)
	p.Register[fakePayload](rec)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go p.Run(ctx)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://localhost/webhook", bytes.NewReader([]byte(testPayload)))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", "abc-123")
	p.ServeHTTP(w, req)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the received event was not processed")
	}
	cancel()
	store.mu.Lock()
	listCalls := store.listCalls
	store.mu.Unlock()
	assertion.Equal(t, listCalls, 2)
}
