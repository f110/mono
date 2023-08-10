package logger

import (
	"io"

	"github.com/sirupsen/logrus"
)

type logrusHook struct{}

var _ logrus.Hook = logrusHook{}

func (logrusHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.PanicLevel, logrus.FatalLevel, logrus.ErrorLevel, logrus.WarnLevel, logrus.InfoLevel, logrus.DebugLevel}
}

func (logrusHook) Fire(e *logrus.Entry) error {
	switch e.Level {
	case logrus.PanicLevel:
		Log.Panic(e.Message)
	case logrus.FatalLevel:
		Log.Fatal(e.Message)
	case logrus.ErrorLevel:
		Log.Error(e.Message)
	case logrus.WarnLevel:
		Log.Warn(e.Message)
	case logrus.InfoLevel:
		Log.Info(e.Message)
	case logrus.DebugLevel:
		Log.Debug(e.Message)
	}
	return nil
}

func HijackStandardLogrus() {
	HijackLogrus(logrus.StandardLogger())
}

func HijackLogrus(logger *logrus.Logger) {
	logger.AddHook(logrusHook{})
	logger.SetOutput(io.Discard)
}
