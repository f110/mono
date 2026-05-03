// Package slogger provides an slog-based logger that mirrors the configuration of go/logger.
// It exists as a parallel interface to allow gradual migration from the zap-based logger.
package slogger

import (
	"errors"
	"fmt"
	"io"
	stdlog "log"
	"log/slog"
	"os"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/logger"
)

var (
	Log *slog.Logger

	initMu sync.Mutex
)

// Init initializes Log using the level / encoding / output configured on the
// parent logger package. Call after flag parsing. Safe to call multiple times.
func Init() error {
	initMu.Lock()
	defer initMu.Unlock()
	if Log != nil {
		return nil
	}

	Log = slog.New(newHandler(selectWriter(logger.Output()), parseLevel(logger.LogLevel()), logger.LogEncoding()))
	return nil
}

func Enabled() bool {
	return Log != nil
}

// StandardLogger returns a *log.Logger that forwards into Log with a "logger=name" attribute.
func StandardLogger(name string) *stdlog.Logger {
	return slog.NewLogLogger(Log.With(slog.String("logger", name)).Handler(), slog.LevelInfo)
}

// Verbose extracts the Verbose() string from err (or any wrapped error) if available.
func Verbose(err error) slog.Attr {
	for {
		v, ok := err.(interface{ Verbose() string })
		if ok {
			return slog.String("verbose", v.Verbose())
		}
		err = errors.Unwrap(err)
		if err == nil {
			break
		}
	}
	return slog.Attr{}
}

// KubernetesObject formats a runtime.Object as "namespace/name".
func KubernetesObject(key string, obj runtime.Object) slog.Attr {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return slog.Attr{}
	}
	return slog.String(key, fmt.Sprintf("%s/%s", accessor.GetNamespace(), accessor.GetName()))
}

// NewBufferLogger returns a *slog.Logger that writes to w using a TextHandler at debug level.
func NewBufferLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func selectWriter(target string) io.Writer {
	if target == "stderr" {
		return os.Stderr
	}
	return os.Stdout
}

func newHandler(w io.Writer, level slog.Level, encoding string) slog.Handler {
	opts := &slog.HandlerOptions{Level: level}
	switch encoding {
	case "json":
		return slog.NewJSONHandler(w, opts)
	default:
		return slog.NewTextHandler(w, opts)
	}
}
