package slogger

import (
	"log/slog"

	"go.f110.dev/xerrors"
	"k8s.io/klog/v2"
)

// OverrideKlog makes klog write its logs through Log with a "logger=klog" attribute.
func OverrideKlog() error {
	if err := Init(); err != nil {
		return xerrors.WithStack(err)
	}

	klog.SetSlogLogger(Log.With(slog.String("logger", "klog")))
	return nil
}
