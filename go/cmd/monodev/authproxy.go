package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli/flag"
	"go.f110.dev/mono/go/http/authproxy"
	"go.f110.dev/mono/go/logger"
)

// PublicKeyPath is the path of the public key which is the same as the internal API of heimdallr.
const PublicKeyPath = "/internal/publickey"

// authProxy is the reverse proxy which works as heimdallr. It forwards the requests to Backend with the token of
// Subject signed by ES256. The private key is generated in KeyDir, and the public key to verify the token is served at
// PublicKeyPath like the internal API of heimdallr.
type authProxy struct {
	Name    string
	Listen  int
	Backend int
	KeyDir  string
	Subject string
}

var _ component = &authProxy{}

func (c *authProxy) GetName() string {
	return c.Name
}

func (c *authProxy) GetType() componentType {
	return componentTypeService
}

func (c *authProxy) GetDeps() []component {
	return nil
}

func (c *authProxy) Run(ctx context.Context) {
	key, err := c.privateKey()
	if err != nil {
		logger.Log.Error("Failed to prepare the key", logger.E(err))
		return
	}

	target := &url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", c.Backend)}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(req *httputil.ProxyRequest) {
			req.SetURL(target)
			token, err := jwt.NewWithClaims(jwt.SigningMethodES256, &authproxy.Claims{RegisteredClaims: jwt.RegisteredClaims{
				Subject:   c.Subject,
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			}}).SignedString(key)
			if err != nil {
				logger.Log.Error("Failed to sign the token", logger.E(err))
				return
			}
			req.Out.Header.Set(authproxy.TokenHeaderName, token)
		},
	}
	publicKey, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		logger.Log.Error("Failed to marshal the public key", logger.E(err))
		return
	}
	mux := http.NewServeMux()
	mux.Handle("/", proxy)
	mux.HandleFunc("GET "+PublicKeyPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-pem-file")
		pem.Encode(w, &pem.Block{Type: "PUBLIC KEY", Bytes: publicKey})
	})
	s := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", c.Listen), Handler: mux}
	go func() {
		<-ctx.Done()
		s.Shutdown(context.Background())
	}()

	logger.Log.Info("Start "+c.Name, slog.Int("listen", c.Listen), slog.Int("backend", c.Backend), slog.String("subject", c.Subject))
	if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Log.Error("Failed to serve", logger.E(err))
	}
}

func (c *authProxy) Ready() bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", c.Listen), 100*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (c *authProxy) Flags(fs *flag.FlagSet) {
	fs.String(c.Name+"-subject", "User id of the requests").Var(&c.Subject).Default(c.Subject)
}

func (c *authProxy) keyDir() string {
	return filepath.Join(os.Getenv("BUILD_WORKING_DIRECTORY"), c.KeyDir)
}

// privateKey returns the key in KeyDir. The key is generated if it does not exist.
func (c *authProxy) privateKey() (*ecdsa.PrivateKey, error) {
	privateKeyFile := filepath.Join(c.keyDir(), "private.pem")
	if b, err := os.ReadFile(privateKeyFile); err == nil {
		block, _ := pem.Decode(b)
		if block == nil {
			return nil, xerrors.Definef("%s is not PEM", privateKeyFile).WithStack()
		}
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		return key, nil
	} else if !os.IsNotExist(err) {
		return nil, xerrors.WithStack(err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	privateKey, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	if err := os.MkdirAll(c.keyDir(), 0755); err != nil {
		return nil, xerrors.WithStack(err)
	}
	if err := os.WriteFile(privateKeyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateKey}), 0600); err != nil {
		return nil, xerrors.WithStack(err)
	}
	logger.Log.Info("Generated the key", slog.String("dir", c.keyDir()))
	return key, nil
}
