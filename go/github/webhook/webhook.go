// Package webhook persists incoming GitHub webhook deliveries and reconciles them asynchronously.
//
// EventProcessor stores each delivery as a PENDING Event and returns 200 immediately, then dispatches each Event to the
// Reconciler registered for its event type. The storage is abstracted by Store so that each application can keep the
// events in its own table.
package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/logger/slogger"
)

// State is the lifecycle state of an Event. The values are persisted as is.
type State int32

const (
	StatePending    State = 0
	StateProcessing State = 1
	StateSucceeded  State = 2
	StateFailed     State = 3
	StateExpired    State = 4
	StateSkipped    State = 5
)

// Event is a single webhook delivery.
type Event struct {
	Id         int32
	DeliveryId string
	EventType  string
	Action     string
	Payload    []byte
	State      State
	Status     []byte
	LastError  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Finalize decides the state of e after Reconcile returned recErr.
//
//   - recErr != nil: FAILED
//   - e.State is PROCESSING: SUCCEEDED
//   - otherwise: the state set by the reconciler is preserved
func (e *Event) Finalize(recErr error) {
	if recErr != nil {
		e.State = StateFailed
		e.LastError = recErr.Error()
		return
	}
	if e.State == StateProcessing {
		e.State = StateSucceeded
	}
	e.LastError = ""
}

// ReadStatus unmarshals the status of e into out. An empty status leaves out at its zero value.
func (e *Event) ReadStatus(out any) error {
	if len(e.Status) == 0 {
		return nil
	}
	if err := json.Unmarshal(e.Status, out); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

// WriteStatus marshals in into the status of e.
func (e *Event) WriteStatus(in any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return xerrors.WithStack(err)
	}
	e.Status = b
	return nil
}

// ErrDuplicateDelivery is returned by Store.Create when the delivery ID is already stored.
var ErrDuplicateDelivery = xerrors.New("duplicate delivery")

// Store persists Events.
type Store interface {
	// Create inserts ev and sets ev.Id. It returns ErrDuplicateDelivery if the delivery ID is already stored.
	Create(ctx context.Context, ev *Event) error
	// ListByState returns the events in any of the given states.
	ListByState(ctx context.Context, states ...State) ([]*Event, error)
	// Claim updates the state of ev to PROCESSING only if it is PENDING or FAILED, and reports whether it was updated.
	Claim(ctx context.Context, ev *Event) (bool, error)
	// Update persists State, Status and LastError of ev.
	Update(ctx context.Context, ev *Event) error
}

// Reconciler processes an Event of a single event type. payload is the payload of ev decoded into T.
//
// Implementations must be idempotent. Progress should be checkpointed into ev.Status with Event.WriteStatus so that
// re-running resumes the work instead of duplicating side effects. A reconciler may set ev.State to a terminal state
// (e.g. StateSkipped) or back to StatePending to be retried. Otherwise the state is decided by Event.Finalize.
type Reconciler[T any] interface {
	EventType() string
	Reconcile(ctx context.Context, ev *Event, payload *T) error
}

type reconcileFunc func(ctx context.Context, ev *Event) error

type minimalPayload struct {
	Action string `json:"action"`
}

// stuckProcessingTimeout is the duration after which a PROCESSING event is considered abandoned by a stopped worker.
const stuckProcessingTimeout = 10 * time.Minute

// queueSize is the number of received events that can wait for Run in memory. An event received while the queue is
// full is processed by polling.
const queueSize = 100

// EventProcessor receives GitHub webhook deliveries with ServeHTTP and reconciles them with Run.
//
// ServeHTTP stores each delivery as a PENDING event and hands it to Run in the same process. Run also polls the Store
// for PENDING and FAILED events to retry them and to process the events received by other processes. Each event is
// dispatched to the Reconciler registered for the event type.
//
// Run can be called concurrently in multiple processes against the same Store. An event is processed only by the
// process that claimed it. An event waiting longer than maxProcessingDuration since it was created is moved to
// EXPIRED. An event left in PROCESSING longer than stuckProcessingTimeout is moved back to PENDING.
type EventProcessor struct {
	store                 Store
	reconcilers           map[string]reconcileFunc
	interval              time.Duration
	maxProcessingDuration time.Duration

	queue chan *Event
	now   func() time.Time
}

func NewEventProcessor(store Store, interval, maxProcessingDuration time.Duration) *EventProcessor {
	return &EventProcessor{
		store:                 store,
		reconcilers:           make(map[string]reconcileFunc),
		interval:              interval,
		maxProcessingDuration: maxProcessingDuration,
		queue:                 make(chan *Event, queueSize),
		now:                   time.Now,
	}
}

// Register adds rec. An event whose payload can not be decoded into T is marked as FAILED.
func (p *EventProcessor) Register[T any](rec Reconciler[T]) {
	p.reconcilers[rec.EventType()] = func(ctx context.Context, ev *Event) error {
		payload := new(T)
		if err := json.Unmarshal(ev.Payload, payload); err != nil {
			return xerrors.WithMessagef(err, "failed to decode %s payload", ev.EventType)
		}
		return rec.Reconcile(ctx, ev, payload)
	}
}

// ServeHTTP stores a GitHub webhook delivery as a PENDING event.
func (p *EventProcessor) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	payload, err := io.ReadAll(req.Body)
	if err != nil {
		slogger.Log.Warn("Failed to read webhook body", slogger.E(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	eventType := req.Header.Get(github.EventTypeHeader)
	if eventType == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	deliveryID := req.Header.Get(github.DeliveryIDHeader)
	if deliveryID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var m minimalPayload
	_ = json.Unmarshal(payload, &m)

	ev := &Event{
		DeliveryId: deliveryID,
		EventType:  eventType,
		Action:     m.Action,
		Payload:    payload,
		State:      StatePending,
		Status:     []byte{},
		CreatedAt:  p.now(),
	}
	if err := p.store.Create(req.Context(), ev); err != nil {
		if errors.Is(err, ErrDuplicateDelivery) {
			slogger.Log.Info("Duplicate webhook delivery", slog.String("delivery_id", deliveryID))
			w.WriteHeader(http.StatusOK)
			return
		}
		slogger.Log.Error("Failed to persist webhook event", slogger.E(err), slog.String("delivery_id", deliveryID), slog.String("event_type", eventType))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	select {
	case p.queue <- ev:
	default:
	}
	w.WriteHeader(http.StatusOK)
}

// Run reconciles the stored events until ctx is cancelled.
func (p *EventProcessor) Run(ctx context.Context) {
	slogger.Log.Info("Start webhook scheduler", slog.Duration("interval", p.interval), slog.Duration("max_processing", p.maxProcessingDuration))

	t := time.NewTicker(p.interval)
	defer t.Stop()

	p.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.runOnce(ctx)
		case ev := <-p.queue:
			p.process(ctx, ev)
		}
	}
}

func (p *EventProcessor) runOnce(ctx context.Context) {
	if err := p.recoverStuck(ctx); err != nil {
		slogger.Log.Warn("Failed to recover stuck PROCESSING events", slogger.E(err))
	}

	events, err := p.store.ListByState(ctx, StatePending, StateFailed)
	if err != nil {
		slogger.Log.Warn("Failed to fetch reconcile targets", slogger.E(err))
		return
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Id < events[j].Id })
	for _, ev := range events {
		if ctx.Err() != nil {
			return
		}
		p.process(ctx, ev)
	}
}

func (p *EventProcessor) recoverStuck(ctx context.Context) error {
	events, err := p.store.ListByState(ctx, StateProcessing)
	if err != nil {
		return xerrors.WithStack(err)
	}
	for _, ev := range events {
		if p.now().Sub(ev.UpdatedAt) <= stuckProcessingTimeout {
			continue
		}
		ev.State = StatePending
		if err := p.store.Update(ctx, ev); err != nil {
			slogger.Log.Warn("Failed to reset stuck PROCESSING event", slog.Int("id", int(ev.Id)), slogger.E(err))
			continue
		}
		slogger.Log.Info("Reset stuck PROCESSING event", slog.Int("id", int(ev.Id)))
	}
	return nil
}

func (p *EventProcessor) process(ctx context.Context, ev *Event) {
	if p.maxProcessingDuration > 0 && p.now().Sub(ev.CreatedAt) > p.maxProcessingDuration {
		ev.State = StateExpired
		if ev.LastError == "" {
			ev.LastError = "exceeded max processing duration"
		}
		if err := p.store.Update(ctx, ev); err != nil {
			slogger.Log.Warn("Failed to mark event as EXPIRED", slog.Int("id", int(ev.Id)), slogger.E(err))
		}
		return
	}

	rec, ok := p.reconcilers[ev.EventType]
	if !ok {
		ev.State = StateSkipped
		ev.LastError = "no reconciler registered for event_type=" + ev.EventType
		if err := p.store.Update(ctx, ev); err != nil {
			slogger.Log.Warn("Failed to mark event as SKIPPED", slog.Int("id", int(ev.Id)), slogger.E(err))
		}
		return
	}

	claimed, err := p.store.Claim(ctx, ev)
	if err != nil {
		slogger.Log.Warn("Failed to claim event", slog.Int("id", int(ev.Id)), slogger.E(err))
		return
	}
	if !claimed {
		return
	}
	ev.State = StateProcessing

	recErr := rec(ctx, ev)
	if recErr != nil {
		slogger.Log.Warn("Reconcile failed", slog.Int("id", int(ev.Id)), slog.String("event_type", ev.EventType), slogger.E(recErr))
	}
	ev.Finalize(recErr)
	if err := p.store.Update(ctx, ev); err != nil {
		slogger.Log.Warn("Failed to persist reconcile state", slog.Int("id", int(ev.Id)), slogger.E(err))
	}
}
