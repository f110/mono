package webhook

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"

	"go.f110.dev/xerrors"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"go.f110.dev/mono/go/logger"
)

// ValidateFunc validates the object in the request. The request is denied if it returns an error.
type ValidateFunc func(req *admissionv1.AdmissionRequest) error

// Server is the server of the validating admission webhook.
type Server struct {
	server     *http.Server
	cert       string
	key        string
	validators map[schema.GroupKind]ValidateFunc
}

func NewServer(addr, cert, key string) *Server {
	s := &Server{
		cert:       cert,
		key:        key,
		validators: make(map[schema.GroupKind]ValidateFunc),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/validate", s.validate)
	s.server = &http.Server{Addr: addr, Handler: mux}
	return s
}

// Register registers the function which validates the objects of the kind.
func (s *Server) Register(gk schema.GroupKind, fn ValidateFunc) {
	s.validators[gk] = fn
}

// Start listens on the address and serves in the background.
func (s *Server) Start() error {
	cert, err := tls.LoadX509KeyPair(s.cert, s.key)
	if err != nil {
		return xerrors.WithStack(err)
	}
	s.server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}

	l, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return xerrors.WithStack(err)
	}

	logger.Log.Info("Start webhook server", slog.String("addr", s.server.Addr))
	go func() {
		if err := s.server.ServeTLS(l, "", ""); err != nil && err != http.ErrServerClosed {
			logger.Log.Error("Failed to serve webhook", logger.E(err))
		}
	}()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) validate(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	review := &admissionv1.AdmissionReview{}
	if err := json.NewDecoder(req.Body).Decode(review); err != nil || review.Request == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	res := &admissionv1.AdmissionReview{
		TypeMeta: review.TypeMeta,
		Response: &admissionv1.AdmissionResponse{UID: review.Request.UID, Allowed: true},
	}
	if fn, ok := s.validators[schema.GroupKind{Group: review.Request.Kind.Group, Kind: review.Request.Kind.Kind}]; ok {
		if err := fn(review.Request); err != nil {
			res.Response.Allowed = false
			res.Response.Result = &metav1.Status{Status: metav1.StatusFailure, Message: err.Error()}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(res); err != nil {
		logger.Log.Warn("Failed to encode the response", logger.E(err))
	}
}
