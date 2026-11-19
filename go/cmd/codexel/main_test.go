package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestServeStopsWithStreamingRequest(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	assertion.MustNoError(t, err)
	// The handler keeps the response open until the request is canceled like WatchAgents.
	h := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-req.Context().Done()
	})

	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)
	go func() { errCh <- serve(ctx, l, h) }()

	res, err := http.Get("http://" + l.Addr().String())
	assertion.MustNoError(t, err)
	defer res.Body.Close()

	cancel()
	select {
	case err := <-errCh:
		assertion.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("serve did not return after the context was canceled")
	}
}

func TestMissingBinaries(t *testing.T) {
	dir := t.TempDir()
	assertion.MustNoError(t, os.WriteFile(filepath.Join(dir, "jj"), nil, 0755))
	t.Setenv("PATH", dir)

	assertion.Equal(t, missingBinaries("herdr", "jj"), []string{"herdr"})
}
