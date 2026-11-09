package slogger

import (
	"bytes"
	"testing"

	"k8s.io/klog/v2"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestOverrideKlog(t *testing.T) {
	buf := new(bytes.Buffer)
	Log = NewBufferLogger(buf)
	t.Cleanup(func() {
		klog.ClearLogger()
		Log = nil
	})

	assertion.MustNoError(t, OverrideKlog())
	klog.Info("information level")
	klog.Error("error level")
	klog.Flush()

	assertion.Contains(t, buf.String(), "information level")
	assertion.Contains(t, buf.String(), "error level")
	assertion.Contains(t, buf.String(), "logger=klog")
}
