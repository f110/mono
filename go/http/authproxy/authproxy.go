// Package authproxy authenticates the requests which come through heimdallr.
// heimdallr signs the token of the user with ES256 and sets it to the request header.
package authproxy

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/logger/slogger"
)

// TokenHeaderName is the header which heimdallr sets the token to.
const TokenHeaderName = "X-Auth-Token"

// Claims is the claims of the token which heimdallr signs. The subject is the user id of heimdallr.
type Claims struct {
	jwt.RegisteredClaims
	Roles []string `json:"roles,omitempty"`
}

type claimsKey struct{}

// ClaimsFrom returns the claims of the token which Authenticate verified.
func ClaimsFrom(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsKey{}).(*Claims)
	return c, ok
}

// Authenticate returns the handler which verifies the token with publicKey and calls next with the context which
// carries the claims. The request without a valid token is responded with 401.
func Authenticate(publicKey *ecdsa.PublicKey, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		token := req.Header.Get(TokenHeaderName)
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		claims := &Claims{}
		_, err := jwt.ParseWithClaims(
			token,
			claims,
			func(*jwt.Token) (any, error) { return publicKey, nil },
			jwt.WithValidMethods([]string{jwt.SigningMethodES256.Alg()}),
			jwt.WithExpirationRequired(),
		)
		if err != nil || claims.Subject == "" {
			slogger.Log.Debug("Failed to verify the token", slogger.E(err))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), claimsKey{}, claims)))
	})
}

// WithSubject returns the handler which treats every request as the request of the user of subject. It is used instead
// of Authenticate when the application runs without heimdallr.
func WithSubject(subject string, next http.Handler) http.Handler {
	claims := &Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: subject}}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		next.ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), claimsKey{}, claims)))
	})
}

// FetchPublicKey fetches the public key to verify the token from url which is the internal API of heimdallr
// (/internal/publickey).
func FetchPublicKey(ctx context.Context, url string) (*ecdsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, xerrors.Definef("failed to fetch the public key: %s", res.Status).WithStack()
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	block, _ := pem.Decode(b)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, xerrors.Define("the response is not PEM encoded public key").WithStack()
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	publicKey, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return nil, xerrors.Define("the public key is not ECDSA").WithStack()
	}
	return publicKey, nil
}
