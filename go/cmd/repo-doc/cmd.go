package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/docutil"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/grpcutil"
	"go.f110.dev/mono/go/logger"
)

type repoDocCommand struct {
	*fsm.FSM
	gitData   git.GitDataClient
	docSearch docutil.DocSearchClient
	s         *http.Server

	Listen          string
	GitDataService  string
	Insecure        bool
	StaticDirectory string
	GlobalTitle     string
	MaxDepthToC     int
	SearchService   string
}

const (
	stateInit fsm.State = iota
	stateShuttingDown
)

func newRepoDocCommand() *repoDocCommand {
	c := &repoDocCommand{}
	c.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:         c.init,
			stateShuttingDown: c.shuttingDown,
		},
		stateInit,
		stateShuttingDown,
	)
	c.CloseContext = func() (context.Context, context.CancelFunc) {
		return ctxutil.WithTimeout(context.Background(), 5*time.Second)
	}

	return c
}

func (c *repoDocCommand) Flags(fs *cli.FlagSet) {
	fs.String("listen", "Listen addr").Var(&c.Listen).Default(":8016")
	fs.String("git-data-service", "The url of git-data-service").Var(&c.GitDataService)
	fs.Bool("insecure", "Insecure access to backend").Var(&c.Insecure)
	fs.String("static-dir", "Directory path that contains will be served as static file").Var(&c.StaticDirectory)
	fs.String("title", "General title").Var(&c.GlobalTitle).Default("repo-doc")
	fs.Int("max-depth-toc", "Maximum depth of table of content").Var(&c.MaxDepthToC).Default(2)
	fs.String("search-service", "The url of search-service").Var(&c.SearchService)
}

func (c *repoDocCommand) init(ctx context.Context) (fsm.State, error) {
	var opts []grpc.DialOption
	if c.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	opts = append(opts, grpcutil.WithLogging())
	gitDataConn, err := grpc.Dial(c.GitDataService, opts...)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	c.gitData = git.NewGitDataClient(gitDataConn)

	if c.SearchService != "" {
		docSearchConn, err := grpc.Dial(c.SearchService, opts...)
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		c.docSearch = docutil.NewDocSearchClient(docSearchConn)
	}

	handler, err := newHttpHandler(ctx, c.gitData, c.docSearch, c.GlobalTitle, c.StaticDirectory, c.MaxDepthToC)
	if err != nil {
		return fsm.Error(err)
	}

	c.s = &http.Server{
		Handler:  handler,
		ErrorLog: logger.StandardLogger("http"),
	}
	lis, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	go func() {
		logger.Log.Info("Start listen", zap.String("addr", c.Listen))
		if err := c.s.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Warn("Serve returns error", logger.Error(err))
		}
	}()

	return fsm.Wait()
}

func (c *repoDocCommand) shuttingDown(ctx context.Context) (fsm.State, error) {
	if c.s != nil {
		if err := c.s.Shutdown(ctx); err != nil {
			return fsm.Error(err)
		}
	}
	return fsm.Finish()
}
