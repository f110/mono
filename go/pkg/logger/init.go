package logger

import (
	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Log *zap.Logger

	logLevel string
)

func Flags(fs *pflag.FlagSet) {
	fs.StringVar(&logLevel, "log-level", "info", "Log level")
}

func SetLogLevel(level string) {
	logLevel = level
}

func Init() error {
	if err := initLogger(); err != nil {
		return err
	}

	return nil
}

func initLogger() error {
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
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	l, err := zapConf.Build()
	if err != nil {
		return xerrors.WithStack(err)
	}

	Log = l
	return nil
}
