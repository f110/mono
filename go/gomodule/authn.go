package gomodule

import (
	"crypto"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-github/v73/github"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.f110.dev/go-memcached/client"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/githubutil"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/stringsutil"
)

const (
	jwtIssuerName = "gomodule-proxy"
	jwtExpiration = 30 * 24 * time.Hour
)

type UserAuthentication struct {
	ghClientFactory  *githubutil.GitHubClientFactory
	signingMethod    jwt.SigningMethod
	signingKey       crypto.PrivateKey
	signingPublicKey crypto.PublicKey
	cache            *client.SinglePool
}

func NewUserAuthentication(privKey crypto.PrivateKey, cache *client.SinglePool) *UserAuthentication {
	var signingMethod jwt.SigningMethod
	var pubKey crypto.PublicKey
	switch key := privKey.(type) {
	case *ecdsa.PrivateKey:
		switch key.Params().BitSize {
		case 256:
			signingMethod = jwt.SigningMethodES256
		}
		pubKey = key.Public()
	}
	return &UserAuthentication{
		signingMethod:    signingMethod,
		signingKey:       privKey,
		signingPublicKey: pubKey,
		cache:            cache,
	}
}

func (a *UserAuthentication) BuildJWT(userID int64, userName string) (string, error) {
	token := jwt.NewWithClaims(a.signingMethod, jwt.MapClaims{
		"iss": jwtIssuerName,                                     // Issuer
		"sub": userName,                                          // Subject
		"iat": jwt.NewNumericDate(time.Now()),                    // IssuedAt
		"exp": jwt.NewNumericDate(time.Now().Add(jwtExpiration)), // ExpiresAt
		"jti": uuid.New().String(),                               // ID
		"gid": userID,                                            // user-defined claim: GitHub user id
	})

	tokenString, err := token.SignedString(a.signingKey)
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	return tokenString, nil
}

func (a *UserAuthentication) VerifyJWT(tokenString string) (int64, string, error) {
	token, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
		return a.signingPublicKey, nil
	})
	if err != nil {
		return -1, "", xerrors.WithStack(err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return -1, "", xerrors.New("invalid token")
	}
	userID, ok := claims["gid"].(int64)
	if !ok {
		return -1, "", xerrors.New("invalid user id")
	}
	sub, err := claims.GetSubject()
	if err != nil {
		return -1, "", xerrors.WithStack(err)
	}
	return userID, sub, nil
}

func (a *UserAuthentication) RegisterHttpMux(mux *mux.Router) {
	// Endpoints for command line tool
	mux.Methods(http.MethodGet).Path("/start").HandlerFunc(a.start)
	mux.Methods(http.MethodPost).Path("/token").HandlerFunc(a.token)

	// Endpoints for the browser
	mux.Methods(http.MethodGet).Path("/login").HandlerFunc(a.login)
	mux.Methods(http.MethodGet).Path("/callback").HandlerFunc(a.loginCallback)
}

func (a *UserAuthentication) start(w http.ResponseWriter, _ *http.Request) {
	deviceCode := stringsutil.RandomString(64)
	userCode := stringsutil.RandomStringWithCharset(9, []rune("BCDFGHJKLMNPQRSTVWXZ"))
	err := a.cache.Set(&client.Item{
		Key:        fmt.Sprintf("userCode/%s", userCode),
		Expiration: 10 * 60, // 10 minutes
	})
	err = a.cache.Set(&client.Item{
		Key:        fmt.Sprintf("deviceCode/%s", deviceCode),
		Value:      []byte(userCode),
		Expiration: 10 * 60, // 10 minutes
	})
	if err != nil {
		http.Error(w, "failed to start authentication", http.StatusInternalServerError)
		return
	}

	res := struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURIComplete string `json:"verification_url_complete"`
		ExpiresIn               int    `json:"expires_in"`
		Interval                int    `json:"interval"`
	}{
		DeviceCode:              deviceCode,
		UserCode:                userCode,
		VerificationURIComplete: fmt.Sprintf("/login?user_code=%s", userCode),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(res); err != nil {
		logger.Log.Info("failed to encode json", logger.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (a *UserAuthentication) token(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		logger.Log.Info("failed to parse form", logger.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	deviceCode := req.PostForm.Get("device_code")
	item, err := a.cache.Get(fmt.Sprintf("deviceCode/%s", deviceCode))
	if err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	userCodeItem, err := a.cache.Get(fmt.Sprintf("userCode/%s", item.Value))
	if err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	if i := strings.IndexRune(string(userCodeItem.Value), ','); i > 0 {
		s, loginName := string(userCodeItem.Value[:i]), string(userCodeItem.Value[i+1:])
		userID, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		token, err := a.BuildJWT(userID, loginName)
		if err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		res := struct {
			AccessToken string `json:"access_token"`
		}{
			AccessToken: token,
		}
		if err := json.NewEncoder(w).Encode(res); err != nil {
			logger.Log.Info("failed to encode json", logger.Error(err))
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}

const cookieName = "gomodule-proxy-authn"

func (a *UserAuthentication) login(w http.ResponseWriter, req *http.Request) {
	userCode := req.URL.Query().Get("user_code")
	if _, err := a.cache.Get(fmt.Sprintf("userCode/%s", userCode)); err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    userCode,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		Expires:  time.Now().Add(10 * time.Minute),
	})

	config := *a.ghClientFactory.OAuthConfig
	config.Scopes = []string{"read:user"}
	redirectURL := a.ghClientFactory.OAuthConfig.AuthCodeURL("")
	http.Redirect(w, req, redirectURL, http.StatusSeeOther)
}

func (a *UserAuthentication) loginCallback(w http.ResponseWriter, req *http.Request) {
	if req.URL.Query().Get("code") == "" {
		http.Error(w, "no code", http.StatusBadRequest)
		return
	}
	token, err := a.ghClientFactory.OAuthConfig.Exchange(req.Context(), req.URL.Query().Get("code"))
	if err != nil {
		logger.Log.Info("Failed exchange token", logger.StackTrace(err))
		http.Error(w, "failed exchange token", http.StatusInternalServerError)
		return
	}
	ghClient := github.NewClient(nil).WithAuthToken(token.AccessToken)
	myself, _, err := ghClient.Users.Get(req.Context(), "")
	if err != nil {
		logger.Log.Info("Failed get user info", logger.StackTrace(err))
		http.Error(w, "failed get user info", http.StatusInternalServerError)
		return
	}

	cookie, err := req.Cookie(cookieName)
	if err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	userCode := cookie.Value
	err = a.cache.Set(&client.Item{
		Key:   fmt.Sprintf("userCode/%s", userCode),
		Value: []byte(fmt.Sprintf("%d,%s", myself.GetID(), myself.GetLogin())),
	})
	if err != nil {
		logger.Log.Info("Failed to set the value", logger.StackTrace(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}
