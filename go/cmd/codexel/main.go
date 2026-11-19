package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/claudecode"
	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/cli/flag"
	"go.f110.dev/mono/go/codexel/config"
	"go.f110.dev/mono/go/codexel/review"
	"go.f110.dev/mono/go/codexel/server"
	"go.f110.dev/mono/go/codexel/static"
	"go.f110.dev/mono/go/herdr"
	"go.f110.dev/mono/go/logger"
)

type command struct {
	Addr        string
	NoBrowser   bool
	HerdrSocket string
	Config      string
}

func (c *command) Flags(fs *flag.FlagSet) {
	fs.String("addr", "Listen address").Var(&c.Addr).Default("localhost:7480")
	fs.Bool("no-browser", "Don't open the browser").Var(&c.NoBrowser)
	fs.String("herdr-socket", "Path of the socket of herdr. If not specified, $HERDR_SOCKET_PATH or ~/.config/herdr/herdr.sock").Var(&c.HerdrSocket)
	fs.String("config", "Path of the config file. If not specified, ~/.config/codexel/config.json").Var(&c.Config)
}

func (c *command) Run(ctx context.Context) error {
	var err error
	socket := c.HerdrSocket
	if socket == "" {
		socket, err = herdr.DefaultSocketPath()
		if err != nil {
			return err
		}
	}
	confPath := c.Config
	if confPath == "" {
		confPath, err = config.DefaultPath()
		if err != nil {
			return err
		}
	}
	reviewDir, err := review.DefaultDir()
	if err != nil {
		return err
	}
	claudeProjectsDir, err := claudecode.DefaultProjectsDir()
	if err != nil {
		return err
	}

	missing := missingBinaries("herdr", "jj")
	for _, v := range missing {
		logger.Log.Warn("Binary is not found in PATH", slog.String("name", v))
	}

	l, err := net.Listen("tcp", c.Addr)
	if err != nil {
		return xerrors.WithStack(err)
	}
	logger.Log.Info("Start server", slog.String("addr", l.Addr().String()))
	if !c.NoBrowser {
		if err := openBrowser(ctx, "http://"+l.Addr().String()); err != nil {
			logger.Log.Info("Failed to open the browser", logger.E(err))
		}
	}
	return serve(ctx, l, server.New(herdr.NewClient(socket), config.NewFile(confPath), review.NewStore(reviewDir), static.FS(), claudeProjectsDir, missing))
}

// serve serves h on l until ctx is canceled. The requests are canceled with ctx because Shutdown waits for the
// streaming requests (e.g. WatchAgents) which never finish by themselves.
func serve(ctx context.Context, l net.Listener, h http.Handler) error {
	s := &http.Server{Handler: h, BaseContext: func(net.Listener) context.Context { return ctx }}
	errCh := make(chan error, 1)
	go func() {
		if err := s.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- xerrors.WithStack(err)
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Shutdown(shutdownCtx); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

// missingBinaries returns the binaries which are not found in PATH.
func missingBinaries(names ...string) []string {
	var missing []string
	for _, v := range names {
		if _, err := exec.LookPath(v); err != nil {
			missing = append(missing, v)
		}
	}
	return missing
}

func openBrowser(ctx context.Context, u string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	if err := exec.CommandContext(ctx, name, u).Run(); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func main() {
	c := &command{}
	cmd := &cli.Command{
		Use:   "codexel",
		Short: "Manage the coding agents and the repositories on the local host",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return c.Run(ctx)
		},
	}
	c.Flags(cmd.Flags())

	if err := cmd.Execute(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
