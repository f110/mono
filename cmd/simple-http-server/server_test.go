package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/netutil"
)

func TestSimpleHTTPServer_init(t *testing.T) {
	conf := `
server {
	listen = ":8081";

	path "/" {
		proxy = "127.0.0.1:10000";
	}
}

server {
	listen = ":8082";

	path "/static" {
		root = "/static";
	}

	path "/api" {
		proxy = "127.0.0.1:10001";
	}
}`
	f, err := os.CreateTemp(t.TempDir(), "")
	require.NoError(t, err)
	f.WriteString(conf)

	s := NewSimpleHTTPServer()
	s.configFile = f.Name()
	_, err = s.init(context.Background())
	require.NoError(t, err)
}

func TestSimpleHTTPServer(t *testing.T) {
	logger.Init()
	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprint(w, "proxy ok")
	}))
	t.Cleanup(func() {
		s1.Close()
	})
	s1URL, err := url.Parse(s1.URL)
	require.NoError(t, err)
	documentRoot := t.TempDir()
	err = os.WriteFile(filepath.Join(documentRoot, "foo"), []byte("file ok"), 0644)
	require.NoError(t, err)

	port1, err := netutil.FindUnusedPort()
	require.NoError(t, err)
	port2, err := netutil.FindUnusedPort()
	require.NoError(t, err)

	conf := `
server {
	listen = "127.0.0.1:%d";

	path "/*" {
		proxy = "http://%s";
	}
}

server {
	listen = "127.0.0.1:%d";

	path "/*" {
		root = "%s";
	}
}`
	renderedConf := fmt.Sprintf(conf, port1, s1URL.Host, port2, documentRoot)
	f, err := os.CreateTemp(t.TempDir(), "")
	require.NoError(t, err)
	_, err = f.WriteString(renderedConf)
	require.NoError(t, err)

	s := NewSimpleHTTPServer()
	s.configFile = f.Name()
	closeCh := make(chan struct{})
	go func() {
		err := s.LoopContext(context.Background())
		require.NoError(t, err)
		close(closeCh)
	}()
	err = netutil.WaitListen(fmt.Sprintf("127.0.0.1:%d", port1), time.Second)
	require.NoError(t, err)
	err = netutil.WaitListen(fmt.Sprintf("127.0.0.1:%d", port2), time.Second)
	require.NoError(t, err)

	res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/ok", port1))
	require.NoError(t, err)
	buf, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.NoError(t, res.Body.Close())
	assert.Equal(t, []byte("proxy ok"), buf)

	res, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/foo", port2))
	require.NoError(t, err)
	buf, err = io.ReadAll(res.Body)
	require.NoError(t, err)
	require.NoError(t, res.Body.Close())
	assert.Equal(t, []byte("file ok"), buf)

	s.FSM.Shutdown()
	select {
	case <-time.After(time.Second):
		require.Fail(t, "timed out")
	case <-closeCh:
	}
}
