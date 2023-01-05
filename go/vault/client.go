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
	"sync"
	"time"

	"go.f110.dev/xerrors"
)

var (
	ErrDataNotFound          = xerrors.New("the path is not found")
	ErrOperationNotPermitted = xerrors.New("operation not permitted")
)

type ClientOpt func(*opOpt)

func Version(v int) ClientOpt {
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

func NewClient(addr, token string) (*Client, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return &Client{addr: u, token: token, httpClient: &http.Client{}}, nil
}

// Get is a function that retrieve a secret from K/V version2 engine.
func (c *Client) Get(ctx context.Context, mountPath, dataPath, key string, opts ...ClientOpt) (string, error) {
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
