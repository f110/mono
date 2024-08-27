package logger

import (
	"io"
	"log"
	"net/url"

	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Log *zap.Logger

	logLevel string
	output   = "stdout"
)

// Flags sets the flag of logger.
// We can't receive *cli.FlagSet directly due to avoid cycle dependency.
func Flags(fs *pflag.FlagSet) {
	fs.StringVar(&logLevel, "log-level", "info", "Log level")
}

func SetLogLevel(level string) {
	logLevel = level
}

func OutputStderr() {
	output = "stderr"
}

func Enabled() bool {
	return Log != nil
}

func Init() error {
	if err := initLogger(); err != nil {
		return err
	}

	return nil
}

func StandardLogger(name string) *log.Logger {
	return zap.NewStdLog(Log.Named(name))
}

type customWriter struct {
	io.Writer
}

func (cw customWriter) Close() error {
	return nil
}
func (cw customWriter) Sync() error {
	return nil
}

func NewBufferLogger(w io.Writer) *zap.Logger {
	err := zap.RegisterSink("buffer", func(_ *url.URL) (zap.Sink, error) {
		return customWriter{w}, nil
	})
	if err != nil {
		panic(err)
	}

	encoderConf := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	zapConf := &zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
		Development:      false,
		Sampling:         nil, // disable sampling
		Encoding:         "console",
		EncoderConfig:    encoderConf,
		OutputPaths:      []string{"buffer:whatever"},
		ErrorOutputPaths: []string{"buffer:whatever"},
	}

	zapLogger, err := zapConf.Build()
	if err != nil {
		panic(err)
	}
	return zapLogger
}

func initLogger() error {
	if Log != nil {
		return nil
	}
	encoderConf := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	level := zap.InfoLevel
	switch logLevel {
	case "debug":
		level = zap.DebugLevel
	case "warn":
		level = zap.WarnLevel
	case "error":
		level = zap.ErrorLevel
	}
	encoding := "console"

	zapConf := &zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      false,
		Sampling:         nil, // disable sampling
		Encoding:         encoding,
		EncoderConfig:    encoderConf,
		OutputPaths:      []string{output},
		ErrorOutputPaths: []string{"stderr"},
	}

	l, err := zapConf.Build()
	if err != nil {
		return xerrors.WithStack(err)
	}

	Log = l
	return nil
}
