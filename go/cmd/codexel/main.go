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
	"go.f110.dev/mono/go/codexel/pullrequest"
	"go.f110.dev/mono/go/codexel/review"
	"go.f110.dev/mono/go/codexel/server"
	"go.f110.dev/mono/go/codexel/static"
	"go.f110.dev/mono/go/herdr"
	"go.f110.dev/mono/go/logger"
)

const mcpServerName = "codexel"

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
	pullRequestsPath, err := pullrequest.DefaultPath()
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
	return serve(ctx, l, server.New(herdr.NewClient(socket), config.NewFile(confPath), review.NewStore(reviewDir), pullrequest.NewFile(pullRequestsPath), static.FS(), claudeProjectsDir, missing))
}

// Setup registers the MCP of the server listening on --addr to Claude Code.
func (c *command) Setup(ctx context.Context) error {
	u, err := mcpURL(c.Addr)
	if err != nil {
		return err
	}
	return registerMCP(ctx, u)
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

// mcpURL returns the URL of the MCP endpoint of the server listening on addr. The unspecified host is replaced with
// localhost because the agents connect from the same host.
func mcpURL(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	if host == "" || net.ParseIP(host).IsUnspecified() {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port) + "/mcp", nil
}

// registerMCP registers u as the MCP server to Claude Code in the user scope. The existing registration is replaced so
// that the URL follows the current address. The error of the removal is ignored because it fails if the server is not
// registered, and the addition fails if the registration still remains.
func registerMCP(ctx context.Context, u string) error {
	_ = exec.CommandContext(ctx, "claude", "mcp", "remove", "--scope", "user", mcpServerName).Run()

	cmd := exec.CommandContext(ctx, "claude", "mcp", "add", "--transport", "http", "--scope", "user", mcpServerName, u)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
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
	cmd.AddCommand(&cli.Command{
		Use:   "setup",
		Short: "Register the MCP of Codexel to Claude Code",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return c.Setup(ctx)
		},
	})

	if err := cmd.Execute(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
