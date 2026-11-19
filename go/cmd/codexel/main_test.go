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

func TestMCPURL(t *testing.T) {
	cases := []struct {
		addr string
		want string
	}{
		{addr: "localhost:7480", want: "http://localhost:7480/mcp"},
		{addr: "127.0.0.1:8080", want: "http://127.0.0.1:8080/mcp"},
		{addr: ":7480", want: "http://localhost:7480/mcp"},
		{addr: "0.0.0.0:7480", want: "http://localhost:7480/mcp"},
		{addr: "[::]:7480", want: "http://localhost:7480/mcp"},
		{addr: "[::1]:7480", want: "http://[::1]:7480/mcp"},
	}
	for _, tc := range cases {
		t.Run(tc.addr, func(t *testing.T) {
			got, err := mcpURL(tc.addr)
			assertion.MustNoError(t, err)
			assertion.Equal(t, got, tc.want)
		})
	}

	_, err := mcpURL("localhost")
	assertion.MustError(t, err)
}

func TestRegisterMCP(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "log")
	stub := "#!/bin/sh\necho \"$@\" >> " + log + "\nif [ \"$2\" = remove ]; then exit 1; fi\n"
	assertion.MustNoError(t, os.WriteFile(filepath.Join(dir, "claude"), []byte(stub), 0755))
	t.Setenv("PATH", dir)

	assertion.MustNoError(t, registerMCP(t.Context(), "http://localhost:7480/mcp"))
	buf, err := os.ReadFile(log)
	assertion.MustNoError(t, err)
	assertion.Equal(t, string(buf), "mcp remove --scope user codexel\nmcp add --transport http --scope user codexel http://localhost:7480/mcp\n")
}

func TestRegisterMCPFailsToAdd(t *testing.T) {
	dir := t.TempDir()
	assertion.MustNoError(t, os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\nexit 1\n"), 0755))
	t.Setenv("PATH", dir)

	assertion.MustError(t, registerMCP(t.Context(), "http://localhost:7480/mcp"))
}
