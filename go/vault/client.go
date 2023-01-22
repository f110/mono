package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"go.f110.dev/xerrors"
)

var (
	ErrDataNotFound          = Error{Message: "path is not found"}
	ErrOperationNotPermitted = Error{Message: "operation not permitted"}
	ErrLoginFailed           = Error{Message: "login failed"}
)

type ClientOpt func(*clientOpt)

type clientOpt struct {
	HttpClient *http.Client
}

func HttpClient(c *http.Client) ClientOpt {
	return func(opt *clientOpt) {
		opt.HttpClient = c
	}
}

type OpOpt func(*opOpt)

func Version(v int) OpOpt {
	return func(opt *opOpt) {
		opt.Version = v
	}
}

type opOpt struct {
	Version int
}

type Client struct {
	addr       *url.URL
	token      string
	httpClient *http.Client
}

// NewClient makes a client for Vault with a static token.
func NewClient(addr, token string, opts ...ClientOpt) (*Client, error) {
	opt := &clientOpt{}
	for _, v := range opts {
		v(opt)
	}

	u, err := url.Parse(addr)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	httpClient := opt.HttpClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &Client{addr: u, token: token, httpClient: httpClient}, nil
}

// NewClientAsK8SServiceAccount makes a client for Vault as a service account of Kubernetes.
func NewClientAsK8SServiceAccount(ctx context.Context, addr, enginePath, role, token string, opts ...ClientOpt) (*Client, error) {
	c, err := NewClient(addr, "", opts...)
	if err != nil {
		return nil, err
	}
	if err := c.LoginAsK8SServiceAccount(ctx, enginePath, role, token); err != nil {
		return nil, err
	}
	return c, nil
}

// Addr returns Vault server address.
func (c *Client) Addr() string {
	return c.addr.String()
}

// Get is a function that retrieves a secret from K/V version2 engine.
func (c *Client) Get(ctx context.Context, mountPath, dataPath, key string, opts ...OpOpt) (string, error) {
	opt := &opOpt{}
	for _, v := range opts {
		v(opt)
	}

	apiPath := path.Join("v1", mountPath, "data", dataPath)
	cache := ctx.Value(contextCacheKey{})
	if cache != nil {
		val, err := cache.(*Cache).Get(apiPath, key)
		if err == nil {
			return val, nil
		}
	}

	u := &url.URL{}
	*u = *c.addr
	u.Path = apiPath
	if opt.Version > 0 {
		query := url.Values{}
		query.Add("version", fmt.Sprintf("%d", opt.Version))
		u.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	req.Header.Set("X-Vault-Token", c.token)
	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusForbidden, http.StatusNotFound:
		return "", xerrors.WithStack(ErrDataNotFound)
	}

	kv := &KV{}
	if err := json.NewDecoder(res.Body).Decode(kv); err != nil {
		return "", xerrors.WithStack(err)
	}
	val := kv.Data.Data[key]

	if cache != nil {
		cache.(*Cache).Set(dataPath, key, val)
	}
	return val, nil
}

// Set creates or updates a secret with data.
// If a secret path already exists, it will be replaced.
func (c *Client) Set(ctx context.Context, mountPath, dataPath string, data map[string]string) error {
	apiPath := path.Join("v1", mountPath, "data", dataPath)

	d := &KVData{Data: data}
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(d); err != nil {
		return xerrors.WithStack(err)
	}

	u := &url.URL{}
	*u = *c.addr
	u.Path = apiPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), buf)
	if err != nil {
		return xerrors.WithStack(err)
	}
	req.Header.Set("X-Vault-Token", c.token)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusForbidden:
		return xerrors.WithStack(ErrOperationNotPermitted)
	}

	return nil
}

func (c *Client) LoginAsK8SServiceAccount(ctx context.Context, enginePath, role, token string) error {
	payload := map[string]string{
		"role": role,
		"jwt":  token,
	}
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(payload); err != nil {
		return xerrors.WithStack(err)
	}

	u := &url.URL{}
	*u = *c.addr
	u.Path = path.Join("v1", enginePath, "login")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), buf)
	if err != nil {
		return xerrors.WithStack(err)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	default:
		var msgs ErrMessage
		_ = json.NewDecoder(res.Body).Decode(&msgs)
		return xerrors.WithStack(Error{Message: ErrLoginFailed.Message, StatusCode: res.StatusCode, VerboseMessage: strings.Join(msgs.Errors, ", ")})
	}

	login := &Login{}
	if err := json.NewDecoder(res.Body).Decode(login); err != nil {
		return xerrors.WithStack(err)
	}
	c.token = login.Auth.ClientToken
	return nil
}

type Cache struct {
	mu   sync.RWMutex
	data map[string]map[string]string
}

var errMiss = errors.New("cache miss")

type contextCacheKey = struct{}

func NewCache(ctx context.Context) context.Context {
	c := &Cache{data: make(map[string]map[string]string)}
	return context.WithValue(ctx, contextCacheKey{}, c)
}

func (c *Cache) Get(p, key string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if data, ok := c.data[p]; ok {
		if val, ok := data[key]; ok {
			return val, nil
		}
		return "", nil
	}

	return "", errMiss
}

func (c *Cache) Set(p, key string, val string) {
	c.mu.Lock()
	if _, ok := c.data[p]; !ok {
		c.data[p] = make(map[string]string)
	}
	c.data[p][key] = val
	c.mu.Unlock()
}

// KV represents the response body of reading secret data
type KV struct {
	RequestId     string `json:"request_id"`
	LeaseId       string `json:"lease_id"`
	Renewable     bool   `json:"renewable"`
	LeaseDuration int    `json:"lease_duration"`
	Data          struct {
		Data     map[string]string `json:"data"`
		Metadata struct {
			CreatedTime    time.Time         `json:"created_time"`
			CustomMetadata map[string]string `json:"custom_metadata"`
			DeletionTime   string            `json:"deletion_time"`
			Destroyed      bool              `json:"destroyed"`
			Version        int               `json:"version"`
		} `json:"metadata"`
	} `json:"data"`
}

type KVData struct {
	Options struct {
		CAS int `json:"cas"`
	} `json:"options"`
	Data map[string]string `json:"data"`
}

type Login struct {
	Auth struct {
		ClientToken string   `json:"client_token"`
		Accessor    string   `json:"accessor"`
		Policies    []string `json:"policies"`
		Metadata    struct {
			Role                     string `json:"role"`
			ServiceAccountName       string `json:"service_account_name"`
			ServiceAccountNamespace  string `json:"service_account_namespace"`
			ServiceAccountSecretName string `json:"service_account_secret_name"`
			ServiceAccountUID        string `json:"service_account_uid"`
		} `json:"metadata"`
		LeaseDuration int  `json:"lease_duration"`
		Renewable     bool `json:"renewable"`
	} `json:"auth"`
}

type ErrMessage struct {
	Errors []string
}

type Error struct {
	Message        string
	StatusCode     int
	VerboseMessage string
}

func (e Error) Error() string {
	return e.Message
}

func (e Error) Verbose() string {
	if e.VerboseMessage == "" {
		return fmt.Sprintf("%d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("%d: %s", e.StatusCode, e.VerboseMessage)
}
