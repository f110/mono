package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/http/pprof"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/nissy/bon"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/http/httpserver"
	"go.f110.dev/mono/go/logger"
)

type SimpleHTTPServer struct {
	*fsm.FSM

	config  *Config
	servers []*http.Server

	// flags
	configFile string
}

const (
	stateInit fsm.State = iota
	stateStartServer
	stateShutdown
)

func NewSimpleHTTPServer() *SimpleHTTPServer {
	s := &SimpleHTTPServer{}
	s.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:        s.init,
			stateStartServer: s.startServer,
			stateShutdown:    s.shutdown,
		},
		stateInit,
		stateShutdown,
	)
	s.FSM.CloseContext = func() (context.Context, context.CancelFunc) {
		return ctxutil.WithTimeout(context.Background(), 30*time.Second)
	}
	s.FSM.DisableErrorOutput = true
	return s
}

func (s *SimpleHTTPServer) SetFlags(fs *cli.FlagSet) {
	fs.String("config", "Config file path").Shorthand("c").Var(&s.configFile).Required()
}

func (s *SimpleHTTPServer) init(_ context.Context) (fsm.State, error) {
	if err := s.readConfig(); err != nil {
		return fsm.Error(err)
	}

	return fsm.Next(stateStartServer)
}

func (s *SimpleHTTPServer) readConfig() error {
	conf, err := readConfigFile(s.configFile)
	if err != nil {
		return err
	}
	for _, v := range conf.Servers() {
		switch c := v.Path.(type) {
		case map[string]any:
			for p, e := range c {
				val, ok := e.(map[string]any)
				if !ok {
					continue
				}
				var proxy, root, accessLog string
				if v, ok := val["proxy"]; ok {
					proxy = v.(string)
				}
				if v, ok := val["root"]; ok {
					root = v.(string)
				}
				if v, ok := val["access_log"]; ok {
					accessLog = v.(string)
				}
				v.path = append(v.path, &PathConfig{
					Path:      p,
					Proxy:     proxy,
					Root:      root,
					AccessLog: accessLog,
				})
			}
		case []any:
			for _, e := range c {
				entry, ok := e.(map[string]any)
				if !ok {
					continue
				}
				for p, va := range entry {
					val := va.(map[string]any)
					var proxy, root, accessLog string
					if v, ok := val["proxy"]; ok {
						proxy = v.(string)
					}
					if v, ok := val["root"]; ok {
						root = v.(string)
					}
					if v, ok := val["access_log"]; ok {
						accessLog = v.(string)
					}
					v.path = append(v.path, &PathConfig{
						Path:      p,
						Proxy:     proxy,
						Root:      root,
						AccessLog: accessLog,
					})
				}
			}
		default:
			log.Printf("%T", c)
		}
	}
	s.config = conf

	return nil
}

var allMethods = []string{
	http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
	http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace,
}

func (s *SimpleHTTPServer) startServer(ctx context.Context) (fsm.State, error) {
	if len(s.config.Servers()) == 0 {
		return fsm.Error(errors.New("there is no server"))
	}

	accessLogger := make(map[string]bon.Middleware)
	for _, v := range s.config.Servers() {
		router := bon.NewRouter()
		server := &http.Server{
			Addr:    v.Listen,
			Handler: router,
		}
		var middle bon.Middleware
		if v.AccessLog != "" {
			l, ok := accessLogger[v.AccessLog]
			if !ok {
				newLogger, err := NewMiddlewareAccessLog(v.AccessLog)
				if err != nil {
					return fsm.Error(err)
				}
				l = newLogger
				accessLogger[v.AccessLog] = l
			}
			middle = l
		}

		for _, p := range v.path {
			var handler http.Handler
			if p.Root != "" {
				handler = httpserver.SinglePageApplication(p.Root)
			}
			if p.Proxy != "" {
				u, err := url.Parse(p.Proxy)
				if err != nil {
					return fsm.Error(err)
				}
				handler = httputil.NewSingleHostReverseProxy(u)
			}
			var middlewares []bon.Middleware
			if p.AccessLog != "" {
				l, ok := accessLogger[p.AccessLog]
				if !ok {
					newLogger, err := NewMiddlewareAccessLog(p.AccessLog)
					if err != nil {
						return fsm.Error(err)
					}
					l = newLogger
					accessLogger[p.AccessLog] = newLogger
				}
				middlewares = append(middlewares, l)
			}
			if middle != nil && p.AccessLog == "" {
				middlewares = append(middlewares, middle)
			}

			for _, m := range allMethods {
				router.Handle(m, p.Path, handler, middlewares...)
			}
		}
		go func() {
			logger.Log.Info("Start server", zap.String("addr", server.Addr))
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Print(err)
			}
		}()
		s.servers = append(s.servers, server)
	}

	if s.config.Pprof != "" {
		router := bon.NewRouter()
		router.Handle(http.MethodGet, "/debug/pprof/*", http.HandlerFunc(pprof.Index))
		router.Handle(http.MethodGet, "/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		router.Handle(http.MethodGet, "/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		router.Handle(http.MethodGet, "/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		router.Handle(http.MethodGet, "/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
		server := &http.Server{
			Addr:    s.config.Pprof,
			Handler: router,
		}
		go func() {
			logger.Log.Info("Start profile server", zap.String("addr", server.Addr))
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Print(err)
			}
		}()
	}
	return fsm.Wait()
}

func (s *SimpleHTTPServer) shutdown(ctx context.Context) (fsm.State, error) {
	for _, v := range s.servers {
		if err := v.Shutdown(ctx); err != nil {
			return fsm.Error(err)
		}
	}
	return fsm.Finish()
}

type loggedResponseWriter struct {
	http.ResponseWriter
	http.Hijacker
	http.Flusher

	status int
}

func (w *loggedResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func NewMiddlewareAccessLog(p string) (bon.Middleware, error) {
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return func(next http.Handler) http.Handler {
		fn := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			writer := &loggedResponseWriter{ResponseWriter: w}
			if h, ok := w.(http.Hijacker); ok {
				writer.Hijacker = h
			}
			if f, ok := w.(http.Flusher); ok {
				writer.Flusher = f
			}

			next.ServeHTTP(writer, req)

			contentLength := w.Header().Get("Content-Length")
			var size int
			if contentLength != "" {
				size, _ = strconv.Atoi(contentLength)
			}
			fmt.Fprintf(f, "%s [%s] \"%s %s %s\" %d %d\n", req.RemoteAddr, time.Now().Format(time.RFC3339), req.Method, req.URL.Path, req.Proto, writer.status, size)
		})
		return fn
	}, nil
}
