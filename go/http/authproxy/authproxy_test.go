package authproxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestAuthenticate(t *testing.T) {
	slogger.Init()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assertion.MustNoError(t, err)
	otherKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assertion.MustNoError(t, err)

	var got *Claims
	handler := Authenticate(&key.PublicKey, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		got, _ = ClaimsFrom(req.Context())
	}))
	serve := func(token string) int {
		got = nil
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if token != "" {
			req.Header.Set(TokenHeaderName, token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code
	}
	sign := func(t *testing.T, k *ecdsa.PrivateKey, claims *Claims) string {
		t.Helper()
		token, err := jwt.NewWithClaims(jwt.SigningMethodES256, claims).SignedString(k)
		assertion.MustNoError(t, err)
		return token
	}
	valid := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: "alice@example.com", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))},
		Roles:            []string{"admin"},
	}

	t.Run("the claims are carried by the context", func(t *testing.T) {
		assertion.Equal(t, serve(sign(t, key, valid)), http.StatusOK)
		assertion.Equal(t, got.Subject, "alice@example.com")
		assertion.Equal(t, got.Roles, []string{"admin"})
	})

	cases := []struct {
		name  string
		token func(t *testing.T) string
	}{
		{name: "no token", token: func(*testing.T) string { return "" }},
		{name: "signed by the other key", token: func(t *testing.T) string { return sign(t, otherKey, valid) }},
		{name: "expired", token: func(t *testing.T) string {
			return sign(t, key, &Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "alice@example.com", ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute))}})
		}},
		{name: "without the expiration", token: func(t *testing.T) string {
			return sign(t, key, &Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "alice@example.com"}})
		}},
		{name: "without the subject", token: func(t *testing.T) string {
			return sign(t, key, &Claims{RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}})
		}},
		{name: "not signed by ES256", token: func(t *testing.T) string {
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, valid).SignedString([]byte("secret"))
			assertion.MustNoError(t, err)
			return token
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertion.Equal(t, serve(tc.token(t)), http.StatusUnauthorized)
			assertion.True(t, got == nil)
		})
	}
}

func TestFetchPublicKey(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assertion.MustNoError(t, err)
	b, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	assertion.MustNoError(t, err)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/internal/publickey" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		pem.Encode(w, &pem.Block{Type: "PUBLIC KEY", Bytes: b})
	}))
	t.Cleanup(s.Close)

	got, err := FetchPublicKey(t.Context(), s.URL+"/internal/publickey")
	assertion.MustNoError(t, err)
	assertion.True(t, got.Equal(&key.PublicKey))

	_, err = FetchPublicKey(t.Context(), s.URL+"/not-found")
	assertion.MustError(t, err)
}

func TestWithSubject(t *testing.T) {
	var got *Claims
	handler := WithSubject("alice@example.com", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		got, _ = ClaimsFrom(req.Context())
	}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	assertion.Equal(t, w.Code, http.StatusOK)
	assertion.Equal(t, got.Subject, "alice@example.com")
}
