package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
	"go.f110.dev/protoc-ddl/probe"
	"go.f110.dev/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	buildapi "go.f110.dev/mono/go/build/api"
	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/git"
	githubutil "go.f110.dev/mono/go/github"
	"go.f110.dev/mono/go/http/authproxy"
	"go.f110.dev/mono/go/issue"
	"go.f110.dev/mono/go/issue/database"
	"go.f110.dev/mono/go/issue/database/dao"
	"go.f110.dev/mono/go/issue/server"
	"go.f110.dev/mono/go/issue/webhook"
	"go.f110.dev/mono/go/logger/slogger"
)

type serverCommand struct {
	Addr           string
	DSN            string
	PublicKeyURL   string
	Subject        string
	BuildAPI       string
	GitDataService string
	GitHub         *githubutil.GitHubClientFactory

	EventReconcileInterval     time.Duration
	EventMaxProcessingDuration time.Duration
}

func (c *serverCommand) Flags(fs *cli.FlagSet) {
	fs.String("addr", "Listen address").Var(&c.Addr).Default(":8080")
	fs.String("dsn", "Data source name of MySQL").Var(&c.DSN).Required()
	fs.String("public-key-url", "URL of the public key of heimdallr to verify the token. It is /internal/publickey of the internal API. If not specified, the server runs without heimdallr").Var(&c.PublicKeyURL)
	fs.String("subject", "Subject of the user of every request when --public-key-url is not specified").Var(&c.Subject).Default("dev@example.com")
	fs.String("build-api", "Address of the gRPC API of Build which manages the repositories. If not specified, the webhook is not handled").Var(&c.BuildAPI)
	fs.String("git-data-service", "Address of git-data-service to look up the change id of the commits. If not specified, the change id is not looked up").Var(&c.GitDataService)
	c.GitHub.Flags(fs)
	fs.Duration("event-reconcile-interval", "Interval between scans of the github_event table for PENDING/FAILED rows").Var(&c.EventReconcileInterval).Default(30 * time.Second)
	fs.Duration("event-max-processing-duration", "Time after `created_at` at which an unfinished github_event row is moved to EXPIRED").Var(&c.EventMaxProcessingDuration).Default(30 * time.Minute)
}

func (c *serverCommand) Run(ctx context.Context) error {
	if err := c.GitHub.Init(); err != nil {
		return err
	}
	authenticate, err := c.authenticator(ctx)
	if err != nil {
		return err
	}
	conn, err := openDatabase(c.DSN)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := waitMigration(ctx, conn); err != nil {
		return err
	}

	daos := dao.NewOptions(conn)
	tracker := issue.NewTracker(daos)
	mux := http.NewServeMux()
	mux.Handle("/", server.New(tracker, authenticate))

	// The repositories are managed by Build, so the webhook and the change id need Build.
	if c.BuildAPI == "" {
		slogger.Log.Info("The webhook is not handled because --build-api is not specified")
	} else {
		buildConn, err := grpc.NewClient(c.BuildAPI, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return xerrors.WithStack(err)
		}
		defer buildConn.Close()
		repositories := webhook.NewBuildRepositories(buildapi.NewAPIClient(buildConn))
		eventProcessor := webhook.NewEventProcessor(daos, tracker, repositories, c.GitHub.REST, c.EventReconcileInterval, c.EventMaxProcessingDuration)
		go eventProcessor.Run(ctx)
		mux.Handle("/webhook", eventProcessor)

		if c.GitDataService == "" {
			slogger.Log.Info("The change id of the commits is not looked up because --git-data-service is not specified")
		} else {
			gitDataConn, err := grpc.NewClient(c.GitDataService, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return xerrors.WithStack(err)
			}
			defer gitDataConn.Close()
			go webhook.NewChangeIDFiller(tracker, repositories, git.NewGitDataClient(gitDataConn)).Run(ctx)
		}
	}
	mux.HandleFunc("/_livez", func(w http.ResponseWriter, _ *http.Request) {})
	mux.HandleFunc("/_readyz", func(w http.ResponseWriter, req *http.Request) {
		if !probe.NewProbe(conn).Ready(req.Context(), database.SchemaHash) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
	s := &http.Server{Addr: c.Addr, Handler: mux}

	errCh := make(chan error, 1)
	go func() {
		slogger.Log.Info("Start server", slog.String("addr", c.Addr))
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- xerrors.WithStack(err)
		}
		close(errCh)
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.Shutdown(shutdownCtx); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

// authenticator returns the middleware which authenticates the requests by the token of heimdallr. Without
// --public-key-url, the server runs without heimdallr and every request is the request of the user of Subject.
func (c *serverCommand) authenticator(ctx context.Context) (func(http.Handler) http.Handler, error) {
	if c.PublicKeyURL == "" {
		slogger.Log.Info("Run without heimdallr", slog.String("subject", c.Subject))
		return func(next http.Handler) http.Handler { return authproxy.WithSubject(c.Subject, next) }, nil
	}
	publicKey, err := authproxy.FetchPublicKey(ctx, c.PublicKeyURL)
	if err != nil {
		return nil, err
	}
	return func(next http.Handler) http.Handler { return authproxy.Authenticate(publicKey, next) }, nil
}

func openDatabase(dsn string) (*sql.DB, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	cfg.ParseTime = true
	cfg.Loc = time.Local
	conn, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return conn, nil
}

// waitMigration waits for the schema of the database to be migrated by the migrate job.
func waitMigration(ctx context.Context, conn *sql.DB) error {
	p := probe.NewProbe(conn)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	timeout := time.After(5 * time.Minute)
	for !p.Ready(ctx, database.SchemaHash) {
		slogger.Log.Info("Waiting for the migration")
		select {
		case <-ticker.C:
		case <-timeout:
			return xerrors.Define("waiting for the migration was timed out").WithStack()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func main() {
	c := &serverCommand{GitHub: githubutil.NewGitHubClientFactory("issue-server", false)}
	cmd := &cli.Command{
		Use:   "issue-server",
		Short: "Start the server of the issue tracker",
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
