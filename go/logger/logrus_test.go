package logger

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestLogrus(t *testing.T) {
	buf := new(bytes.Buffer)
	logger := logrus.New()
	logger.SetOutput(buf)
	logger.SetLevel(logrus.DebugLevel)
	logger.Info("foobar")
	assert.Greater(t, buf.Len(), 10)
	bufSize := buf.Len()

	loggerOut := new(bytes.Buffer)
	encoder := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
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
	})
	zCore := zapcore.NewCore(encoder, zapcore.AddSync(loggerOut), zapcore.DebugLevel)
	stubLogger := zap.New(zCore)
	Log = stubLogger
	HijackLogrus(logger)

	logger.Info("information level")
	assert.Equal(t, bufSize, buf.Len(), "Don't write any bytes to io.Writer after hijacking")
	logger.Error("error level")
	logger.Warn("warning level")
	logger.Debug("debug log level")
	assert.Contains(t, loggerOut.String(), "information level")
	assert.Contains(t, loggerOut.String(), "error level")
	assert.Contains(t, loggerOut.String(), "warning level")
	assert.Contains(t, loggerOut.String(), "debug log level")
}
