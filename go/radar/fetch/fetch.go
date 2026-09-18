// Package fetch provides the HTTP client that the sources share.
//
// It limits the concurrency and the rate and retries the temporary failures. A fetcher never
// interprets the body it fetched. Even a byte sequence that fails to decode is returned as it is
// and the judgement is left to the parser, so the raw data that was stored can be parsed again
// when the parser is fixed.
package fetch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.f110.dev/xerrors"
)

var (
	// ErrNotAvailable means that the source does not have the data of the time or the unit.
	ErrNotAvailable = xerrors.Define("fetch: data is not available")
)

// ClientOpt is the option of Client.
type ClientOpt struct {
	// Concurrency is the maximum number of the requests that run at the same time.
	Concurrency int
	// MinInterval is the minimum interval between the requests. The counterpart is a public
	// infrastructure so the load is limited voluntarily.
	MinInterval time.Duration
	// Retries is the maximum number of the retries. 429, 5xx and the timeout are retried.
	// A nil value means the default of 3. Set it to 0 to turn the retry off.
	Retries *int
	// RetryWait is the wait before the first retry. It is doubled on every retry.
	RetryWait time.Duration
	Timeout   time.Duration
	// UserAgent is sent on every request. It is not sent when it is empty.
	//
	// Some of the sources answer 403 to a request whose User-Agent does not look like a browser,
	// so the value has to be configurable per source.
	UserAgent string
	// Transport is for the test. http.DefaultTransport is used when it is nil.
	Transport http.RoundTripper
}

// BrowserUserAgent is a User-Agent of a browser.
const BrowserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"

const defaultRetries = 3

func (o *ClientOpt) setDefault() {
	if o.Concurrency <= 0 {
		o.Concurrency = 4
	}
	if o.MinInterval <= 0 {
		o.MinInterval = 100 * time.Millisecond
	}
	if o.Retries == nil {
		o.Retries = new(defaultRetries)
	} else if *o.Retries < 0 {
		*o.Retries = 0
	}
	if o.RetryWait <= 0 {
		o.RetryWait = 500 * time.Millisecond
	}
	if o.Timeout <= 0 {
		o.Timeout = 30 * time.Second
	}
}

// Client is the HTTP client that the fetchers share.
//
// Each source has its own Client so that the rate limit of a source does not affect the others.
//
// The conditional request is not used. Both of the sources put the time in the URL, so a URL is
// fetched exactly once and a validator cache would only grow without ever hitting.
type Client struct {
	opt    ClientOpt
	client *http.Client
	sem    chan struct{}

	mu   sync.Mutex
	last time.Time
}

// Response is the result of a request.
type Response struct {
	Body        []byte
	ContentType string
}

func NewClient(opt ClientOpt) *Client {
	opt.setDefault()
	return &Client{
		opt:    opt,
		client: &http.Client{Transport: opt.Transport, Timeout: opt.Timeout},
		sem:    make(chan struct{}, opt.Concurrency),
	}
}

// Get fetches the URL. It returns ErrNotAvailable when the server answered 404.
func (c *Client) Get(ctx context.Context, url string) (*Response, error) {
	return c.do(ctx, http.MethodGet, url)
}

// Exists reports whether the URL is available. It is used to find the latest published time.
func (c *Client) Exists(ctx context.Context, url string) (bool, error) {
	_, err := c.do(ctx, http.MethodHead, url)
	if err != nil {
		if errors.Is(err, ErrNotAvailable) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) do(ctx context.Context, method, url string) (*Response, error) {
	// The context has to be checked before the select below. When both the semaphore and the done
	// channel are ready the select picks one at random, so a canceled context would still send a
	// request every now and then.
	if err := ctx.Err(); err != nil {
		return nil, xerrors.WithStack(err)
	}
	select {
	case c.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, xerrors.WithStack(ctx.Err())
	}
	defer func() { <-c.sem }()

	wait := c.opt.RetryWait
	var lastErr error
	for i := 0; i <= *c.opt.Retries; i++ {
		if i > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, xerrors.WithStack(ctx.Err())
			}
			wait *= 2
		}
		if err := c.waitForRate(ctx); err != nil {
			return nil, err
		}

		res, retryAfter, err := c.request(ctx, method, url)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if retryAfter < 0 {
			return nil, err
		}
		// The server told how long to wait. Honour it instead of the exponential backoff.
		if retryAfter > 0 {
			wait = retryAfter
		}
	}
	return nil, lastErr
}

// waitForRate blocks until the minimum interval from the last request has passed.
func (c *Client) waitForRate(ctx context.Context) error {
	c.mu.Lock()
	now := time.Now()
	next := c.last.Add(c.opt.MinInterval)
	if next.After(now) {
		c.last = next
		c.mu.Unlock()
		select {
		case <-time.After(next.Sub(now)):
		case <-ctx.Done():
			return xerrors.WithStack(ctx.Err())
		}
		return nil
	}
	c.last = now
	c.mu.Unlock()
	return nil
}

// request sends a single request.
//
// The second return value tells the caller how to retry. A negative value means that retrying does
// not help, zero means retry with the exponential backoff and a positive value is the wait that
// the server asked for.
func (c *Client) request(ctx context.Context, method, url string) (*Response, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, -1, xerrors.WithStack(err)
	}
	if c.opt.UserAgent != "" {
		req.Header.Set("User-Agent", c.opt.UserAgent)
	}

	res, err := c.client.Do(req)
	if err != nil {
		// A network error and the timeout are worth retrying.
		return nil, 0, xerrors.WithStack(err)
	}
	defer res.Body.Close()

	switch {
	case res.StatusCode == http.StatusNotFound:
		// The unit may not have the data. Retrying does not help.
		return nil, -1, xerrors.WithMessagef(ErrNotAvailable, "%s: %d", url, res.StatusCode)
	case res.StatusCode == http.StatusTooManyRequests, res.StatusCode >= 500:
		// The counterpart said it is too much, or it is having a bad time. Back off.
		return nil, retryAfter(res), xerrors.NewfWithStack("fetch: %s: %d", url, res.StatusCode)
	case res.StatusCode != http.StatusOK:
		return nil, -1, xerrors.NewfWithStack("fetch: %s: %d", url, res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, 0, xerrors.WithStack(err)
	}
	return &Response{Body: body, ContentType: res.Header.Get("Content-Type")}, -1, nil
}

// retryAfter reads the Retry-After header. It returns 0 when the header is absent or unusable so
// that the caller falls back to the exponential backoff.
func retryAfter(res *http.Response) time.Duration {
	v := res.Header.Get("Retry-After")
	if v == "" {
		return 0
	}
	if sec, err := strconv.Atoi(v); err == nil && sec >= 0 {
		return time.Duration(sec) * time.Second
	}
	if at, err := http.ParseTime(v); err == nil {
		if d := time.Until(at); d > 0 {
			return d
		}
	}
	return 0
}
