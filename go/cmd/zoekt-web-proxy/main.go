package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"

	"github.com/spf13/pflag"
	"golang.org/x/xerrors"
)

func zoektWebProxy(args []string) error {
	listen := ":8080"
	webAddr := ":6070"
	statusAddr := ":7070"
	fs := pflag.NewFlagSet("zoekt-web-proxy", pflag.ContinueOnError)
	fs.StringVar(&listen, "listen", listen, "Listen addr")
	fs.StringVar(&webAddr, "web-addr", webAddr, "Listen addr by zoekt-webserver")
	fs.StringVar(&statusAddr, "status-addr", statusAddr, "Listen addr by status server")
	if err := fs.Parse(args); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	p := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			// The path that starting with underscore such as /_status will be transfer to status server.
			// Other path will be transfer to zoekt-webserver.
			if len(req.URL.Path) > 2 && req.URL.Path[1] == '_' {
				req.URL.Host = statusAddr
			} else {
				req.URL.Host = webAddr
			}
		},
	}

	if err := http.ListenAndServe(listen, p); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func main() {
	if err := zoektWebProxy(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
