package logger

import (
	"bytes"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"k8s.io/klog/v2"
)

type levelWriter struct {
	fn func(msg string, field ...zap.Field)
}

func (w *levelWriter) Write(p []byte) (int, error) {
	s := bytes.SplitAfterN(p, []byte(" "), 6)
	w.fn(string(bytes.TrimRight(s[len(s)-1], "\n")))
	return len(p), nil
}

func OverrideKlog() error {
	if err := Init(); err != nil {
		return xerrors.WithStack(err)
	}

	l := Log.Named("klog").WithOptions(zap.AddCallerSkip(5))
	klog.SetOutputBySeverity("INFO", &levelWriter{fn: l.Info})
	klog.SetOutputBySeverity("WARNING", &levelWriter{fn: l.Info})
	klog.SetOutputBySeverity("ERROR", &levelWriter{fn: l.Info})
	klog.SetOutputBySeverity("FATAL", &levelWriter{fn: l.Info})
	return nil
}
