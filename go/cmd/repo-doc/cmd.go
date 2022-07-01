package main

import (
	"errors"
	"net"
	"net/http"

	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/pkg/fsm"
	"go.f110.dev/mono/go/pkg/git"
	"go.f110.dev/mono/go/pkg/logger"
)

type repoDocCommand struct {
	*fsm.FSM
	gitData git.GitDataClient
	s       *http.Server

	Listen         string
	GitDataService string
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
}

func (c *repoDocCommand) init() (fsm.State, error) {
	conn, err := grpc.Dial(c.GitDataService)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	c.gitData = git.NewGitDataClient(conn)

	c.s = &http.Server{
		Handler:  newHttpHandler(c.gitData),
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
	if err := c.s.Shutdown(c.FSM.Context()); err != nil {
		return fsm.Error(err)
	}
	return fsm.Finish()
}
