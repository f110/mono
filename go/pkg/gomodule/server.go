package gomodule

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

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
	targetQuery := upstream.RawQuery
	director := func(req *http.Request) {
		req.URL.Scheme = upstream.Scheme
		req.URL.Host = upstream.Host
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}
		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			req.Header.Set("User-Agent", "")
		}
		req.Host = upstream.Host
	}

	s := &ProxyServer{
		r:     mux.NewRouter(),
		rr:    &httputil.ReverseProxy{Director: director},
		proxy: proxy,
	}
	s.s = &http.Server{
		Addr:    addr,
		Handler: s.r,
	}

	// Endpoints for Go module proxy
	s.r.Methods(http.MethodGet).Path("/{module:.+}/@v/list").HandlerFunc(s.handle(s.list))
	s.r.Methods(http.MethodGet).Path("/{module:.+}/@v/{version}.info").HandlerFunc(s.handle(s.info))
	s.r.Methods(http.MethodGet).Path("/{module:.+}/@v/{version}.mod").HandlerFunc(s.handle(s.mod))
	s.r.Methods(http.MethodGet).Path("/{module:.+}/@v/{version}.zip").HandlerFunc(s.handle(s.zip))
	s.r.Methods(http.MethodGet).Path("/{module:.+}/@latest").HandlerFunc(s.handle(s.latest))

	// Endpoints for frontend
	s.r.Methods(http.MethodGet).Path("/").HandlerFunc(s.index)
	s.r.Methods(http.MethodGet).Path("/{module:.+}/@v/invalidate").HandlerFunc(s.handle(s.invalidate))
	s.r.Methods(http.MethodPost).Path("/flush_all").HandlerFunc(s.flushAll) // This endpoint is hidden.

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

func (s *ProxyServer) index(w http.ResponseWriter, _ *http.Request) {
	cachedModuleRoots, err := s.proxy.CachedModuleRoots()
	if err != nil {
		logger.Log.Info("failed to get a list of modules", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	io.WriteString(w, "<html><head></head><body>")
	io.WriteString(w, "<h3>Cached modules</h3>\n")
	io.WriteString(w, "<ul>")
	for _, v := range cachedModuleRoots {
		io.WriteString(w, "<li>")
		fmt.Fprintf(w, "%s <a href=\"/%s/@v/invalidate\">[invalidate]</a>", v.RootPath, v.RootPath)
		io.WriteString(w, "<ul>\n")
		for _, v := range v.Modules {
			fmt.Fprintf(w, "<li>%s</li>", v.Path)
		}
		io.WriteString(w, "</ul>\n")
		io.WriteString(w, "</li>\n")
	}
	io.WriteString(w, "</ul></body></html>")
}

func (s *ProxyServer) invalidate(w http.ResponseWriter, req *http.Request, module, _ string) {
	if err := s.proxy.InvalidateCache(module); err != nil {
		logger.Log.Info("Failed invalidate cache", zap.Error(err), zap.String("module", module))
		http.Error(w, "failed invalidate cache", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, req, "/", http.StatusSeeOther)
}

func (s *ProxyServer) flushAll(w http.ResponseWriter, _ *http.Request) {
	if err := s.proxy.FlushAllCache(); err != nil {
		http.Error(w, fmt.Sprintf("failed flush all cache: %v", err), http.StatusInternalServerError)
		return
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
		t1 := time.Now()

		next.ServeHTTP(w, req)

		logger.Log.Info("access",
			zap.String("host", req.Host),
			zap.String("protocol", req.Proto),
			zap.String("method", req.Method),
			zap.String("path", req.URL.Path),
			zap.String("remote_addr", req.RemoteAddr),
			zap.String("ua", req.Header.Get("User-Agent")),
			zap.Duration("response_time", time.Since(t1)),
		)
	})
}

func middlewareDebugInfo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		if len(vars) != 0 {
			logger.Log.Debug("Debug info", zap.Any("vars", vars))
		}

		next.ServeHTTP(w, req)
	})
}
