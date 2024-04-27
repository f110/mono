package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
)

const (
	TokenHeaderName = "X-Auth-Token"
)

type Config struct {
	Listen        string
	Upstream      string
	Username      string
	Password      string
	Insecure      bool
	PublicKeyFile string
	Debug         bool
}

func authByHeader(publicKey crypto.PublicKey, h http.HandlerFunc, debug bool) http.HandlerFunc {
	if debug {
		return func(w http.ResponseWriter, req *http.Request) {
			log.Print("Skip verify the token header. because currently running under debug mode.")
			h(w, req)
		}
	}

	return func(w http.ResponseWriter, req *http.Request) {
		log.Printf("%+v", req)
		tokenHeader := req.Header.Get(TokenHeaderName)
		if tokenHeader == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		token, err := jwt.Parse(tokenHeader, func(token *jwt.Token) (i interface{}, err error) {
			if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
				return nil, xerrors.Definef("unexpected signature algorithm: %s", token.Header["alg"]).WithStack()
			}

			return publicKey, nil
		})
		if err != nil {
			log.Printf("%+v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if claims, ok := token.Claims.(jwt.MapClaims); !ok || !token.Valid {
			log.Printf("Failed verify token: %s", tokenHeader)
			w.WriteHeader(http.StatusUnauthorized)
			return
		} else {
			log.Printf("Authorized access to %s", claims["jti"])
		}

		// Authorized
		h(w, req)
	}
}

const (
	RCOk    = "ok"
	RCError = "error"

	SessionCookieName = "unifises"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Meta struct {
	RC      string `json:"rc"`
	Message string `json:"msg"`
}

type LoginResponse struct {
	Meta Meta `json:"meta"`
}

type InjectAuthRoundTripper struct {
	http.RoundTripper
	Username string
	Password string

	upstream *url.URL
	mutex    sync.RWMutex
	cookie   *http.Cookie
	client   *http.Client
}

func NewInjectAuthRoundTripper(upstream *url.URL, username, password string, roundTripper http.RoundTripper) (*InjectAuthRoundTripper, error) {
	rt := &InjectAuthRoundTripper{
		RoundTripper: roundTripper,
		Username:     username,
		Password:     password,
		upstream:     upstream,
		client:       &http.Client{Transport: roundTripper},
	}
	if _, err := rt.login(); err != nil {
		return nil, xerrors.WithStack(err)
	}

	return rt, nil
}

func (rt *InjectAuthRoundTripper) login() (*http.Cookie, error) {
	param := &LoginRequest{rt.Username, rt.Password}
	buf, err := json.Marshal(param)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s://%s/api/login", rt.upstream.Scheme, rt.upstream.Host), bytes.NewReader(buf))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()
	req = req.WithContext(ctx)

	res, err := rt.client.Do(req)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	if err := res.Body.Close(); err != nil {
		return nil, xerrors.WithStack(err)
	}

	resBody := &LoginResponse{}
	if err := json.Unmarshal(body, resBody); err != nil {
		return nil, xerrors.WithStack(err)
	}

	if resBody.Meta.RC != RCOk {
		return nil, xerrors.New(resBody.Meta.Message)
	}

	var sessionCookie *http.Cookie
	for _, v := range res.Cookies() {
		if v.Name == SessionCookieName {
			sessionCookie = v
			break
		}
	}
	if sessionCookie == nil {
		return nil, xerrors.New("Cookie not found")
	}

	rt.mutex.Lock()
	rt.cookie = sessionCookie
	rt.mutex.Unlock()

	return sessionCookie, nil
}

func (rt *InjectAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.mutex.RLock()
	cookie := rt.cookie
	rt.mutex.RUnlock()
	req.AddCookie(cookie)

	res, err := rt.RoundTripper.RoundTrip(req)
	if err != nil {
		return res, err
	}

	if res.StatusCode == http.StatusUnauthorized {
		cookie, err = rt.login()
		if err != nil {
			return nil, err
		}

		cookies := req.Cookies()
		req.Header.Del("Cookie")
		for _, v := range cookies {
			if v.Name != rt.cookie.Name {
				req.AddCookie(v)
			}
		}
		req.AddCookie(cookie)
		res, err = rt.RoundTrip(req)
	}

	return res, err
}

func handleSignal(ctx context.Context, ch chan os.Signal, shutdown func()) {
	for {
		select {
		case <-ch:
			shutdown()
		case <-ctx.Done():
			return
		}
	}
}

func uniFiProxy(args []string) error {
	conf := &Config{}
	fs := pflag.NewFlagSet("unifi-proxy", pflag.ContinueOnError)
	fs.StringVar(&conf.Upstream, "upstream", "", "Upstream address")
	fs.StringVar(&conf.Listen, "listen", "127.0.0.1:7000", "Listen address")
	fs.StringVar(&conf.PublicKeyFile, "public-key", "", "PEM encoded public key file path. It used to verify a request.")
	fs.BoolVar(&conf.Insecure, "insecure", false, "Skip verify server certification")
	fs.BoolVar(&conf.Debug, "debug", false, "Enable debug mode")
	if err := fs.Parse(args); err != nil {
		return xerrors.WithStack(err)
	}
	conf.Username = os.Getenv("UNIFI_USERNAME")
	conf.Password = os.Getenv("UNIFI_PASSWORD")

	server, err := newProxy(conf)
	if err != nil {
		return xerrors.WithStack(err)
	}

	signalCh := make(chan os.Signal)
	signal.Notify(signalCh, os.Interrupt, os.Kill)
	go handleSignal(context.Background(), signalCh, func() {
		server.Shutdown(context.Background())
	})

	log.Printf("Start listening: %s", server.Addr)
	err = server.ListenAndServe()
	if err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func newProxy(conf *Config) (*http.Server, error) {
	u, err := url.Parse(conf.Upstream)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	buf, err := ioutil.ReadFile(conf.PublicKeyFile)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	block, _ := pem.Decode(buf)
	if block.Type != "PUBLIC KEY" {
		return nil, xerrors.New("PEM type is not public key")
	}
	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	rp := httputil.NewSingleHostReverseProxy(u)
	if conf.Insecure {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	roundTripper, err := NewInjectAuthRoundTripper(u, conf.Username, conf.Password, http.DefaultTransport)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	rp.Transport = roundTripper
	s := &http.Server{
		Addr:    conf.Listen,
		Handler: authByHeader(publicKey, rp.ServeHTTP, conf.Debug),
	}

	return s, nil
}

func main() {
	if err := uniFiProxy(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%+v", err)
		os.Exit(1)
	}
}
