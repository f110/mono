package gomodule

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/logger"
)

type ProxyServer struct {
	s     *http.Server
	rr    *httputil.ReverseProxy
	r     *mux.Router
	proxy *ModuleProxy
}

func NewProxyServer(addr string, upstream *url.URL, proxy *ModuleProxy) *ProxyServer {
	s := &ProxyServer{
		r:     mux.NewRouter(),
		rr:    httputil.NewSingleHostReverseProxy(upstream),
		proxy: proxy,
	}
	s.s = &http.Server{
		Addr:    addr,
		Handler: s.r,
	}

	s.r.Methods(http.MethodGet).Path("/{module:.+}/@v/list").HandlerFunc(s.handle(s.list))
	s.r.Methods(http.MethodGet).Path("/{module:.+}/@v/{version}.info").HandlerFunc(s.handle(s.info))
	s.r.Methods(http.MethodGet).Path("/{module:.+}/@v/{version}.mod").HandlerFunc(s.handle(s.mod))
	s.r.Methods(http.MethodGet).Path("/{module:.+}/@v/{version}.zip").HandlerFunc(s.handle(s.zip))
	s.r.Methods(http.MethodGet).Path("/{module:.+}/@latest").HandlerFunc(s.handle(s.latest))
	s.r.Use(middlewareAccessLog)
	s.r.Use(middlewareDebugInfo)

	return s
}

func (s *ProxyServer) Start() error {
	logger.Log.Info("Start proxy", zap.String("addr", s.s.Addr))
	if err := s.s.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			return nil
		}

		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (s *ProxyServer) Stop(ctx context.Context) error {
	return s.s.Shutdown(ctx)
}

func (s *ProxyServer) handle(h func(w http.ResponseWriter, req *http.Request, module, version string)) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		if v, ok := vars["module"]; !ok || v == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if s.proxy.IsProxy(vars["module"]) {
			h(w, req, vars["module"], vars["version"])
			return
		}

		s.rr.ServeHTTP(w, req)
	}
}

func (s *ProxyServer) list(w http.ResponseWriter, req *http.Request, module, _ string) {
	vers, err := s.proxy.Versions(req.Context(), module)
	if err != nil {
		logger.Log.Info("Failed to get version list", zap.Error(err))
		http.Error(w, "", http.StatusNotFound)
		return
	}

	for _, v := range vers {
		fmt.Fprintln(w, v)
	}
}

func (s *ProxyServer) info(w http.ResponseWriter, req *http.Request, module, version string) {
	info, err := s.proxy.GetInfo(req.Context(), module, version)
	if err != nil {
		logger.Log.Info("Failed to get module info", zap.Error(err))
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	if err := json.NewEncoder(w).Encode(info); err != nil {
		logger.Log.Info("Failed to encode to json", zap.Error(err))
		return
	}
}

func (s *ProxyServer) mod(w http.ResponseWriter, req *http.Request, module, version string) {
	mod, err := s.proxy.GetGoMod(req.Context(), module, version)
	if err != nil {
		logger.Log.Info("Failed to get go.mod", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	_, err = io.WriteString(w, mod)
	if err != nil {
		logger.Log.Info("Failed to write a buffer to ResponseWriter", zap.Error(err))
	}
}

func (s *ProxyServer) zip(w http.ResponseWriter, req *http.Request, module, version string) {
	err := s.proxy.GetZip(req.Context(), w, module, version)
	if err != nil {
		logger.Log.Info("Failed to create zip", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}

func (s *ProxyServer) latest(w http.ResponseWriter, req *http.Request, module, _ string) {
	info, err := s.proxy.GetLatestVersion(req.Context(), module)
	if err != nil {
		logger.Log.Info("Failed to get latest module version", zap.Error(err))
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	if err := json.NewEncoder(w).Encode(info); err != nil {
		logger.Log.Info("Failed to encode to json", zap.Error(err))
		return
	}
}

func middlewareAccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		logger.Log.Info("access",
			zap.String("host", req.Host),
			zap.String("protocol", req.Proto),
			zap.String("method", req.Method),
			zap.String("path", req.URL.Path),
			zap.String("remote_addr", req.RemoteAddr),
			zap.String("ua", req.Header.Get("User-Agent")),
		)

		next.ServeHTTP(w, req)
	})
}

func middlewareDebugInfo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		logger.Log.Debug("Debug info", zap.Any("vars", vars))

		next.ServeHTTP(w, req)
	})
}
