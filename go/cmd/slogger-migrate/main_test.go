package main

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/testing/assertion"
)

// TestMigrateSource covers every rewrite the tool is expected to perform.
// Each case is a focused, minimal Go source program demonstrating one
// pattern; the table compares the formatted output of migrateSource against
// the expected post-migration form byte-for-byte.
//
// Use this test as the regression bench: when a new pattern needs to be
// supported, add a case here first, watch it fail, then change main.go.
func TestMigrateSource(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		changed bool
	}{
		{
			name: "logger.Log + logger.Error",
			input: `package p

import "go.f110.dev/mono/go/logger"

func f(err error) {
	logger.Log.Warn("msg", logger.Error(err))
}
`,
			want: `package p

import (
	"go.f110.dev/mono/go/logger/slogger"
)

func f(err error) {
	slogger.Log.Warn("msg", slogger.E(err))
}
`,
			changed: true,
		},
		{
			name: "zap.Error inside logger.Log call",
			input: `package p

import (
	"go.uber.org/zap"

	"go.f110.dev/mono/go/logger"
)

func f(err error) {
	logger.Log.Info("msg", zap.Error(err))
}
`,
			want: `package p

import (
	"go.f110.dev/mono/go/logger/slogger"
)

func f(err error) {
	slogger.Log.Info("msg", slogger.E(err))
}
`,
			changed: true,
		},
		{
			name: "zap.String / zap.Int / zap.Bool / zap.Duration / zap.Time / zap.Any",
			input: `package p

import (
	"time"

	"go.uber.org/zap"

	"go.f110.dev/mono/go/logger"
)

func f() {
	logger.Log.Info("msg",
		zap.String("s", "x"),
		zap.Int("i", 1),
		zap.Bool("b", true),
		zap.Duration("d", time.Second),
		zap.Time("t", time.Now()),
		zap.Any("a", struct{}{}),
	)
}
`,
			want: `package p

import (
	"log/slog"
	"time"

	"go.f110.dev/mono/go/logger/slogger"
)

func f() {
	slogger.Log.Info("msg",
		slog.String("s", "x"),
		slog.Int("i", 1),
		slog.Bool("b", true),
		slog.Duration("d", time.Second),
		slog.Time("t", time.Now()),
		slog.Any("a", struct{}{}),
	)
}
`,
			changed: true,
		},
		{
			name: "zap.Int32 wraps second arg with int()",
			input: `package p

import (
	"go.uber.org/zap"

	"go.f110.dev/mono/go/logger"
)

func f(id int32) {
	logger.Log.Debug("msg", zap.Int32("id", id))
}
`,
			want: `package p

import (
	"log/slog"

	"go.f110.dev/mono/go/logger/slogger"
)

func f(id int32) {
	slogger.Log.Debug("msg", slog.Int("id", int(id)))
}
`,
			changed: true,
		},
		{
			name: "zap.Int64 / zap.Uint64 keep native slog form",
			input: `package p

import (
	"go.uber.org/zap"

	"go.f110.dev/mono/go/logger"
)

func f(a int64, b uint64) {
	logger.Log.Debug("msg", zap.Int64("a", a), zap.Uint64("b", b))
}
`,
			want: `package p

import (
	"log/slog"

	"go.f110.dev/mono/go/logger/slogger"
)

func f(a int64, b uint64) {
	slogger.Log.Debug("msg", slog.Int64("a", a), slog.Uint64("b", b))
}
`,
			changed: true,
		},
		{
			name: "zap.Uint casts to uint64",
			input: `package p

import (
	"go.uber.org/zap"

	"go.f110.dev/mono/go/logger"
)

func f(n uint) {
	logger.Log.Info("msg", zap.Uint("n", n))
}
`,
			want: `package p

import (
	"log/slog"

	"go.f110.dev/mono/go/logger/slogger"
)

func f(n uint) {
	slogger.Log.Info("msg", slog.Uint64("n", uint64(n)))
}
`,
			changed: true,
		},
		{
			name: "zap.Strings / zap.Array fall back to slog.Any",
			input: `package p

import (
	"go.uber.org/zap"

	"go.f110.dev/mono/go/logger"
)

func f(ss []string) {
	logger.Log.Info("msg", zap.Strings("ss", ss))
}
`,
			want: `package p

import (
	"log/slog"

	"go.f110.dev/mono/go/logger/slogger"
)

func f(ss []string) {
	slogger.Log.Info("msg", slog.Any("ss", ss))
}
`,
			changed: true,
		},
		{
			name: "logger.String / logger.Stringf",
			input: `package p

import "go.f110.dev/mono/go/logger"

func f(s string) {
	logger.Log.Info("msg",
		logger.String("k", s),
		logger.Stringf("file", "pack-%s.idx", s),
	)
}
`,
			want: `package p

import (
	"fmt"
	"log/slog"

	"go.f110.dev/mono/go/logger/slogger"
)

func f(s string) {
	slogger.Log.Info("msg",
		slog.String("k", s),
		slog.String("file", fmt.Sprintf("pack-%s.idx", s)),
	)
}
`,
			changed: true,
		},
		{
			name: "logger.Stringf without prior fmt import",
			input: `package p

import "go.f110.dev/mono/go/logger"

func f(s string) {
	logger.Log.Info("msg", logger.Stringf("file", "x-%s", s))
}
`,
			want: `package p

import (
	"fmt"
	"log/slog"

	"go.f110.dev/mono/go/logger/slogger"
)

func f(s string) {
	slogger.Log.Info("msg", slog.String("file", fmt.Sprintf("x-%s", s)))
}
`,
			changed: true,
		},
		{
			name: "Stringf does not duplicate existing fmt import",
			input: `package p

import (
	"fmt"

	"go.f110.dev/mono/go/logger"
)

func f(s string) string {
	logger.Log.Info("msg", logger.Stringf("file", "x-%s", s))
	return fmt.Sprintf("%s", s)
}
`,
			want: `package p

import (
	"fmt"
	"log/slog"

	"go.f110.dev/mono/go/logger/slogger"
)

func f(s string) string {
	slogger.Log.Info("msg", slog.String("file", fmt.Sprintf("x-%s", s)))
	return fmt.Sprintf("%s", s)
}
`,
			changed: true,
		},
		{
			name: "logger.StackTrace dropped when sibling Error is present",
			input: `package p

import "go.f110.dev/mono/go/logger"

func f(err error) {
	logger.Log.Error("msg", logger.Error(err), logger.StackTrace(err))
}
`,
			want: `package p

import (
	"go.f110.dev/mono/go/logger/slogger"
)

func f(err error) {
	slogger.Log.Error("msg", slogger.E(err))
}
`,
			changed: true,
		},
		{
			name: "logger.StackTrace dropped when sibling zap.Error is present",
			input: `package p

import (
	"go.uber.org/zap"

	"go.f110.dev/mono/go/logger"
)

func f(err error) {
	logger.Log.Error("msg", zap.Error(err), logger.StackTrace(err))
}
`,
			want: `package p

import (
	"go.f110.dev/mono/go/logger/slogger"
)

func f(err error) {
	slogger.Log.Error("msg", slogger.E(err))
}
`,
			changed: true,
		},
		{
			name: "lone logger.StackTrace becomes slogger.E to keep error info",
			input: `package p

import "go.f110.dev/mono/go/logger"

func f(err error) {
	logger.Log.Info("Failed", logger.StackTrace(err))
}
`,
			want: `package p

import (
	"go.f110.dev/mono/go/logger/slogger"
)

func f(err error) {
	slogger.Log.Info("Failed", slogger.E(err))
}
`,
			changed: true,
		},
		{
			name: "logger.Verbose / logger.KubernetesObject route through slogger",
			input: `package p

import (
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/logger"
)

func f(err error, obj runtime.Object) {
	logger.Log.Debug("msg", logger.Verbose(err), logger.KubernetesObject("obj", obj))
}
`,
			want: `package p

import (
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/logger/slogger"
)

func f(err error, obj runtime.Object) {
	slogger.Log.Debug("msg", slogger.Verbose(err), slogger.KubernetesObject("obj", obj))
}
`,
			changed: true,
		},
		{
			name: "logger.Init / logger.Enabled rewritten to slogger equivalents",
			input: `package p

import "go.f110.dev/mono/go/logger"

func main() {
	if err := logger.Init(); err != nil {
		return
	}
	_ = logger.Enabled()
}
`,
			want: `package p

import (
	"go.f110.dev/mono/go/logger/slogger"
)

func main() {
	if err := slogger.Init(); err != nil {
		return
	}
	_ = slogger.Enabled()
}
`,
			changed: true,
		},
		{
			name: "preserved logger.* helpers keep the logger import",
			input: `package p

import (
	"github.com/spf13/pflag"

	"go.f110.dev/mono/go/logger"
)

func f(fs *pflag.FlagSet) {
	logger.Flags(fs)
	logger.SetLogLevel("debug")
	logger.OutputStderr()
	_ = logger.OverrideKlog
	_ = logger.HijackStandardLogrus
	_ = logger.NewNamedWriter
}
`,
			// Source unchanged: every reference is to a preserved helper.
			want: `package p

import (
	"github.com/spf13/pflag"

	"go.f110.dev/mono/go/logger"
)

func f(fs *pflag.FlagSet) {
	logger.Flags(fs)
	logger.SetLogLevel("debug")
	logger.OutputStderr()
	_ = logger.OverrideKlog
	_ = logger.HijackStandardLogrus
	_ = logger.NewNamedWriter
}
`,
			changed: false,
		},
		{
			name: "preserved helpers coexist with rewritten calls",
			input: `package p

import (
	"github.com/spf13/pflag"

	"go.f110.dev/mono/go/logger"
)

func f(fs *pflag.FlagSet, err error) {
	logger.Flags(fs)
	logger.Log.Warn("msg", logger.Error(err))
}
`,
			want: `package p

import (
	"github.com/spf13/pflag"

	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/logger/slogger"
)

func f(fs *pflag.FlagSet, err error) {
	logger.Flags(fs)
	slogger.Log.Warn("msg", slogger.E(err))
}
`,
			changed: true,
		},
		{
			name: "file without logger / zap imports is left untouched",
			input: `package p

import "fmt"

func f() { fmt.Println("hello") }
`,
			want: `package p

import "fmt"

func f() { fmt.Println("hello") }
`,
			changed: false,
		},
		{
			name: "nested logger.Log call inside other arguments is rewritten",
			input: `package p

import (
	"go.uber.org/zap"

	"go.f110.dev/mono/go/logger"
)

func f(err error, name string) {
	logger.Log.Warn("Failed",
		logger.Error(err),
		zap.String("name", name),
	)
}
`,
			want: `package p

import (
	"log/slog"

	"go.f110.dev/mono/go/logger/slogger"
)

func f(err error, name string) {
	slogger.Log.Warn("Failed",
		slogger.E(err),
		slog.String("name", name),
	)
}
`,
			changed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, changed, err := migrateSource("input.go", []byte(tc.input))
			assertion.MustNoError(t, err)
			assertion.Equal(t, tc.changed, changed)
			assertion.Equal(t, tc.want, string(got))
		})
	}
}

// TestMigrateSource_Idempotent runs the tool twice and ensures the second
// pass is a no-op for every case in TestMigrateSource. Catches transforms
// that would re-rewrite their own output (e.g. accidentally treating
// `slogger.Log.X` as `logger.Log.X`).
func TestMigrateSource_Idempotent(t *testing.T) {
	cases := []string{
		`package p

import "go.f110.dev/mono/go/logger"

func f(err error) { logger.Log.Warn("m", logger.Error(err)) }
`,
		`package p

import (
	"go.uber.org/zap"

	"go.f110.dev/mono/go/logger"
)

func f(id int32) { logger.Log.Debug("m", zap.Int32("id", id)) }
`,
	}
	for _, src := range cases {
		first, _, err := migrateSource("input.go", []byte(src))
		assertion.MustNoError(t, err)
		second, changed, err := migrateSource("input.go", first)
		assertion.MustNoError(t, err)
		assertion.False(t, changed)
		assertion.Equal(t, string(first), string(second))
	}
}

// TestMigratorWalk_MissingPath exercises the CLI entry point with a
// non-existent path. The tool should surface a clean fs.ErrNotExist (no
// runtime panic, no stack trace), so callers can present a friendly
// message.
func TestMigratorWalk_MissingPath(t *testing.T) {
	m := &migrator{}
	missing := filepath.Join(t.TempDir(), "does-not-exist.go")
	err := m.walk(missing)
	assertion.MustError(t, err)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected fs.ErrNotExist, got %v", err)
	}
	// xerrors.WithStack prepends a stack trace that the main printer
	// (%+v) dumps to stderr — for "user gave a bad path" we don't want
	// that noise. Verify the error has no stack frames attached.
	if hasStack(err) {
		t.Errorf("error for missing path carries a stack trace: %+v", err)
	}
}

// TestMigratorWalk_MissingDirectory is the directory variant of
// MissingPath. Same expectation: a clean error, no stack dump.
func TestMigratorWalk_MissingDirectory(t *testing.T) {
	m := &migrator{}
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	err := m.walk(missing)
	assertion.MustError(t, err)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected fs.ErrNotExist, got %v", err)
	}
	if hasStack(err) {
		t.Errorf("error for missing directory carries a stack trace: %+v", err)
	}
}

// hasStack reports whether err (or any error it wraps) carries an xerrors
// stack trace. Used to assert that user-facing errors don't dump developer
// frames when printed with %+v.
func hasStack(err error) bool {
	return len(xerrors.StackTrace(err)) > 0
}
