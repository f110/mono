package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/go-sql-driver/mysql"
	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/logger/slogger"
)

// mysqlDuplicateEntry is MySQL/MariaDB's error number for a UNIQUE constraint
// violation. We treat it as a benign no-op on the webhook path so retried
// deliveries from GitHub return 200 idempotently.
const mysqlDuplicateEntry = 1062

// Notifier lets the webhook handler wake the scheduler when it inserts a row.
// In multi-process deployments only the leader's scheduler is wired up; other
// processes hold a no-op Notifier and rely on the leader's poll interval.
type Notifier struct {
	mu       sync.Mutex
	channels []chan struct{}
}

func NewNotifier() *Notifier {
	return &Notifier{}
}

// Register binds a channel that the scheduler reads from. Called once by the
// scheduler at startup.
func (n *Notifier) Register(ch chan struct{}) {
	n.mu.Lock()
	n.channels = append(n.channels, ch)
	n.mu.Unlock()
}

// Notify wakes the attached scheduler if any. Non-blocking: if a wake is
// already pending it is coalesced.
func (n *Notifier) Notify() {
	n.mu.Lock()
	channels := n.channels
	n.mu.Unlock()
	if len(channels) == 0 {
		return
	}
	for _, ch := range channels {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Handler ingests GitHub webhook deliveries. It does no business logic beyond
// extracting routing fields and writing a row; reconciliation happens
// asynchronously in the scheduler.
type Handler struct {
	dao      dao.Options
	notifier *Notifier
}

func NewHandler(daoOptions dao.Options, notifier *Notifier) *Handler {
	return &Handler{dao: daoOptions, notifier: notifier}
}

// minimalPayload pulls only the fields the handler itself needs out of the
// webhook payload. The full payload is persisted raw for the reconciler.
type minimalPayload struct {
	Action string `json:"action"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
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
		// GitHub always sets X-GitHub-Delivery. A missing header is a sign of
		// an unauthenticated / malformed request.
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var m minimalPayload
	// Ignore JSON errors — `action` is optional and absent for some events
	// (e.g. push). We still record the row with empty action.
	_ = json.Unmarshal(payload, &m)

	if err := h.insert(req.Context(), deliveryID, eventType, m.Action, payload); err != nil {
		if isDuplicateEntry(err) {
			slogger.Log.Info("Duplicate webhook delivery", slog.String("delivery_id", deliveryID))
			w.WriteHeader(http.StatusOK)
			return
		}
		slogger.Log.Error("Failed to persist webhook event", slogger.E(err), slog.String("delivery_id", deliveryID), slog.String("event_type", eventType))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.notifier.Notify()
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) insert(ctx context.Context, deliveryID, eventType, action string, payload []byte) error {
	_, err := h.dao.GithubEvent.Create(ctx, &database.GithubEvent{
		DeliveryId: deliveryID,
		EventType:  eventType,
		Action:     action,
		Payload:    payload,
		State:      database.GithubEventStatePending,
		Status:     []byte{},
		LastError:  "",
	})
	if err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func isDuplicateEntry(err error) bool {
	if me, ok := errors.AsType[*mysql.MySQLError](err); ok {
		return me.Number == mysqlDuplicateEntry
	}
	// In tests the duplicate is reported as a plain string error. Match
	// loosely to keep the handler robust against driver swap-outs.
	return err != nil && strings.Contains(err.Error(), "Duplicate entry")
}
