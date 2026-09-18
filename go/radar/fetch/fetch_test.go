package fetch

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"

	"go.f110.dev/mono/go/testing/assertion"
)

func testClient(tr http.RoundTripper, opt ClientOpt) *Client {
	opt.Transport = tr
	if opt.MinInterval == 0 {
		opt.MinInterval = time.Nanosecond
	}
	if opt.RetryWait == 0 {
		opt.RetryWait = time.Millisecond
	}
	return NewClient(opt)
}

func TestClient_Get(t *testing.T) {
	t.Run("body", func(t *testing.T) {
		tr := httpmock.NewMockTransport()
		tr.RegisterResponder(http.MethodGet, "https://example.invalid/a.png",
			httpmock.NewBytesResponder(http.StatusOK, []byte("body")))

		c := testClient(tr, ClientOpt{})
		res, err := c.Get(context.Background(), "https://example.invalid/a.png")
		assertion.MustNoError(t, err)
		assertion.Equal(t, string(res.Body), "body")
	})

	t.Run("retry", func(t *testing.T) {
		cases := []struct {
			name         string
			status       int
			okAfter      int
			retries      int
			wantRequests int
			wantErr      bool
			wantNotAvail bool
		}{
			{name: "not found is not retried", status: http.StatusNotFound, retries: 3, wantRequests: 1, wantErr: true, wantNotAvail: true},
			{name: "client error is not retried", status: http.StatusForbidden, retries: 3, wantRequests: 1, wantErr: true},
			{name: "server error is retried", status: http.StatusInternalServerError, okAfter: 3, retries: 3, wantRequests: 3},
			{name: "too many requests is retried", status: http.StatusTooManyRequests, okAfter: 2, retries: 3, wantRequests: 2},
			{name: "gives up after the retries", status: http.StatusInternalServerError, retries: 2, wantRequests: 3, wantErr: true},
			{name: "no retry when it is turned off", status: http.StatusInternalServerError, retries: 0, wantRequests: 1, wantErr: true},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				tr := httpmock.NewMockTransport()
				var requests int
				tr.RegisterNoResponder(func(*http.Request) (*http.Response, error) {
					requests++
					if tc.okAfter > 0 && requests >= tc.okAfter {
						return httpmock.NewStringResponse(http.StatusOK, "ok"), nil
					}
					return httpmock.NewStringResponse(tc.status, ""), nil
				})

				c := testClient(tr, ClientOpt{Retries: new(tc.retries)})
				res, err := c.Get(context.Background(), "https://example.invalid/a.png")
				if tc.wantErr {
					assertion.Error(t, err)
					assertion.Equal(t, errors.Is(err, ErrNotAvailable), tc.wantNotAvail)
				} else {
					assertion.MustNoError(t, err)
					assertion.Equal(t, string(res.Body), "ok")
				}
				assertion.Equal(t, requests, tc.wantRequests)
			})
		}
	})

	t.Run("honours Retry-After", func(t *testing.T) {
		tr := httpmock.NewMockTransport()
		var requests int
		tr.RegisterNoResponder(func(*http.Request) (*http.Response, error) {
			requests++
			if requests >= 2 {
				return httpmock.NewStringResponse(http.StatusOK, "ok"), nil
			}
			res := httpmock.NewStringResponse(http.StatusTooManyRequests, "")
			res.Header.Set("Retry-After", "1")
			return res, nil
		})

		c := testClient(tr, ClientOpt{Retries: new(3)})
		start := time.Now()
		_, err := c.Get(context.Background(), "https://example.invalid/a.png")
		assertion.MustNoError(t, err)
		assertion.True(t, time.Since(start) >= time.Second)
	})

	t.Run("rate limit", func(t *testing.T) {
		tr := httpmock.NewMockTransport()
		tr.RegisterNoResponder(httpmock.NewStringResponder(http.StatusOK, ""))

		c := testClient(tr, ClientOpt{MinInterval: 20 * time.Millisecond})
		start := time.Now()
		for i := 0; i < 3; i++ {
			_, err := c.Get(context.Background(), "https://example.invalid/a.png")
			assertion.MustNoError(t, err)
		}
		assertion.True(t, time.Since(start) >= 40*time.Millisecond)
	})

	t.Run("canceled context", func(t *testing.T) {
		tr := httpmock.NewMockTransport()
		tr.RegisterNoResponder(httpmock.NewStringResponder(http.StatusOK, ""))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		c := testClient(tr, ClientOpt{})
		_, err := c.Get(ctx, "https://example.invalid/a.png")
		assertion.Error(t, err)
	})

	t.Run("user agent", func(t *testing.T) {
		cases := []struct {
			name string
			opt  ClientOpt
			want string
		}{
			{name: "sent when it is set", opt: ClientOpt{UserAgent: "test-agent/1.0"}, want: "test-agent/1.0"},
			{name: "not sent when it is empty", opt: ClientOpt{}, want: ""},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				tr := httpmock.NewMockTransport()
				got := make(map[string]string)
				tr.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
					got[req.Method] = req.Header.Get("User-Agent")
					return httpmock.NewStringResponse(http.StatusOK, ""), nil
				})

				c := testClient(tr, tc.opt)
				_, err := c.Get(context.Background(), "https://example.invalid/a.png")
				assertion.MustNoError(t, err)
				_, err = c.Exists(context.Background(), "https://example.invalid/a.png")
				assertion.MustNoError(t, err)

				assertion.Equal(t, got[http.MethodGet], tc.want)
				assertion.Equal(t, got[http.MethodHead], tc.want)
			})
		}
	})
}

func TestClient_Exists(t *testing.T) {
	tr := httpmock.NewMockTransport()
	tr.RegisterResponder(http.MethodHead, "https://example.invalid/a.png",
		httpmock.NewStringResponder(http.StatusOK, ""))
	tr.RegisterResponder(http.MethodHead, "https://example.invalid/b.png",
		httpmock.NewStringResponder(http.StatusNotFound, ""))

	cases := []struct {
		name string
		url  string
		want bool
	}{
		{name: "published", url: "https://example.invalid/a.png", want: true},
		{name: "not published", url: "https://example.invalid/b.png", want: false},
	}

	c := testClient(tr, ClientOpt{})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := c.Exists(context.Background(), tc.url)
			assertion.MustNoError(t, err)
			assertion.Equal(t, ok, tc.want)
		})
	}
}

func TestRetryAfter(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{name: "absent", header: "", want: 0},
		{name: "seconds", header: "2", want: 2 * time.Second},
		{name: "zero seconds", header: "0", want: 0},
		{name: "unparsable", header: "soon", want: 0},
		{name: "a date in the past", header: "Wed, 19 Aug 2020 07:00:00 GMT", want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := &http.Response{Header: http.Header{}}
			if tc.header != "" {
				res.Header.Set("Retry-After", tc.header)
			}
			assertion.Equal(t, retryAfter(res), tc.want)
		})
	}
}
