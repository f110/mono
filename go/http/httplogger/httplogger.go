package httplogger

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
)

type LoggerRoundTripper struct {
	internal http.RoundTripper
	body     bool
}

func New(rt http.RoundTripper, body bool) http.RoundTripper {
	return &LoggerRoundTripper{internal: rt, body: body}
}

func (t *LoggerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	fmt.Fprintln(os.Stderr, "---")
	defer fmt.Fprintln(os.Stderr, "---")
	buf, err := httputil.DumpRequest(req, t.body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dump request error: %v\n", err)
	} else {
		fmt.Fprintln(os.Stderr, string(buf))
	}
	res, err := t.internal.RoundTrip(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Response error: %v\n", err)
		return nil, err
	}

	buf, err = httputil.DumpResponse(res, t.body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dump response error: %v\n", err)
	} else {
		fmt.Fprintln(os.Stderr, string(buf))
	}
	return res, err
}
