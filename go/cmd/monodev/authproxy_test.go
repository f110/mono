package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"go.f110.dev/mono/go/http/authproxy"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/netutil"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestAuthProxy(t *testing.T) {
	assertion.MustNoError(t, slogger.Init())
	t.Setenv("BUILD_WORKING_DIRECTORY", t.TempDir())

	tokens := make(chan string, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		tokens <- req.Header.Get(authproxy.TokenHeaderName)
		io.WriteString(w, req.URL.Path)
	}))
	t.Cleanup(backend.Close)
	u, err := url.Parse(backend.URL)
	assertion.MustNoError(t, err)
	backendPort, err := strconv.Atoi(u.Port())
	assertion.MustNoError(t, err)
	listen, err := netutil.FindUnusedPort()
	assertion.MustNoError(t, err)

	p := &authProxy{Name: "test-auth-proxy", Listen: listen, Backend: backendPort, KeyDir: ".test", Subject: "dev@example.com"}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go p.Run(ctx)
	deadline := time.Now().Add(5 * time.Second)
	for !p.Ready() {
		if time.Now().After(deadline) {
			t.Fatal("the proxy is not ready")
		}
		time.Sleep(10 * time.Millisecond)
	}

	res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/mono.issue.Issue/WhoAmI", listen))
	assertion.MustNoError(t, err)
	body, err := io.ReadAll(res.Body)
	assertion.MustNoError(t, err)
	res.Body.Close()
	assertion.Equal(t, string(body), "/mono.issue.Issue/WhoAmI")

	publicKey, err := authproxy.FetchPublicKey(ctx, fmt.Sprintf("http://127.0.0.1:%d%s", listen, PublicKeyPath))
	assertion.MustNoError(t, err)
	claims := &authproxy.Claims{}
	_, err = jwt.ParseWithClaims(<-tokens, claims, func(*jwt.Token) (any, error) { return publicKey, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodES256.Alg()}), jwt.WithExpirationRequired())
	assertion.MustNoError(t, err)
	assertion.Equal(t, claims.Subject, "dev@example.com")
}
