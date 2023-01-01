package main

import (
	"errors"
	"net"
	"net/http"

	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.f110.dev/mono/go/docutil"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/pkg/grpcutil"
	"go.f110.dev/mono/go/pkg/logger"
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

	return c
}

func (c *repoDocCommand) Flags(fs *pflag.FlagSet) {
	fs.StringVar(&c.Listen, "listen", ":8016", "Listen addr")
	fs.StringVar(&c.GitDataService, "git-data-service", "", "The url of git-data-service")
	fs.BoolVar(&c.Insecure, "insecure", false, "Insecure access to backend")
	fs.StringVar(&c.StaticDirectory, "static-dir", "", "Directory path that contains will be served as static file")
	fs.StringVar(&c.GlobalTitle, "title", "repo-doc", "General title")
	fs.IntVar(&c.MaxDepthToC, "max-depth-toc", 2, "Maximum depth of table of content")
	fs.StringVar(&c.SearchService, "search-service", "", "The url of search-service")
}

func (c *repoDocCommand) init() (fsm.State, error) {
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

	handler, err := newHttpHandler(c.Context(), c.gitData, c.docSearch, c.GlobalTitle, c.StaticDirectory, c.MaxDepthToC)
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

func (c *repoDocCommand) shuttingDown() (fsm.State, error) {
	if c.s != nil {
		if err := c.s.Shutdown(c.FSM.Context()); err != nil {
			return fsm.Error(err)
		}
	}
	return fsm.Finish()
}
