package logger

import (
	"log"

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
