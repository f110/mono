package main

import (
	"net"

	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.f110.dev/mono/go/pkg/docutil"
	"go.f110.dev/mono/go/pkg/fsm"
	"go.f110.dev/mono/go/pkg/git"
	"go.f110.dev/mono/go/pkg/logger"
)

type docSearchService struct {
	*fsm.FSM
	s *grpc.Server

	Listen         string
	GitDataService string
	Insecure       bool

	gitData git.GitDataClient
}

const (
	stateInit fsm.State = iota
	stateStartServer
	stateShutDown
)

func newDocSearchService() *docSearchService {
	c := &docSearchService{}
	c.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:        c.init,
			stateStartServer: c.startServer,
			stateShutDown:    c.shutDown,
		},
		stateInit,
		stateShutDown,
	)

	return c
}

func (c *docSearchService) init() (fsm.State, error) {
	var opts []grpc.DialOption
	if c.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	conn, err := grpc.Dial(c.GitDataService, opts...)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	c.gitData = git.NewGitDataClient(conn)

	s := grpc.NewServer()
	service := docutil.NewDocSearchService(c.gitData)
	docutil.RegisterDocSearchServer(s, service)
	c.s = s

	logger.Log.Debug("Initialize cache")
	if err := service.Initialize(c.Context()); err != nil {
		return fsm.Error(err)
	}

	return fsm.Next(stateStartServer)
}

func (c *docSearchService) startServer() (fsm.State, error) {
	lis, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}

	logger.Log.Info("Start listen", zap.String("addr", c.Listen))
	go func() {
		if err := c.s.Serve(lis); err != nil {
			logger.Log.Error("gRPC server returns error", logger.Error(err))
		}
	}()

	return fsm.Wait()
}

func (c *docSearchService) shutDown() (fsm.State, error) {
	if c.s != nil {
		logger.Log.Debug("Graceful stopping")
		c.s.GracefulStop()
		logger.Log.Info("Stop server")
	}

	return fsm.Finish()
}

func (c *docSearchService) Flags(fs *pflag.FlagSet) {
	fs.StringVar(&c.Listen, "listen", ":8057", "Listen addr")
	fs.StringVar(&c.GitDataService, "git-data-service", "127.0.0.1:9010", "The url of git-data-service")
	fs.BoolVar(&c.Insecure, "insecure", false, "Insecure access to backend")

}
