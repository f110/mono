package main

import (
	"context"
	"net"
	"time"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/docutil"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/grpcutil"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

type docSearchService struct {
	*fsm.FSM
	s *grpc.Server

	Listen                 string
	GitDataService         string
	Insecure               bool
	Workers                int
	MaxConns               int
	Bucket                 string
	StorageEndpoint        string
	StorageRegion          string
	StorageAccessKey       string
	StorageSecretAccessKey string
	StorageCAFile          string

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

func (c *docSearchService) init(ctx context.Context) (fsm.State, error) {
	var opts = []grpc.DialOption{grpcutil.WithLogging()}
	if c.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	conn, err := grpc.Dial(c.GitDataService, opts...)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	c.gitData = git.NewGitDataClient(conn)
	opt := storage.NewS3OptionToExternal(c.StorageEndpoint, c.StorageRegion, c.StorageAccessKey, c.StorageSecretAccessKey)
	opt.PathStyle = true
	opt.CACertFile = c.StorageCAFile
	storageClient := storage.NewS3(c.Bucket, opt)

	s := grpc.NewServer()
	service := docutil.NewDocSearchService(c.gitData, storageClient)
	docutil.RegisterDocSearchServer(s, service)
	healthSvc := health.NewServer()
	healthSvc.SetServingStatus("doc-search", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(s, healthSvc)
	c.s = s

	logger.Log.Debug("Initialize cache")
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if err := service.Initialize(ctx, c.Workers, c.MaxConns); err != nil {
		return fsm.Error(err)
	}

	return fsm.Next(stateStartServer)
}

func (c *docSearchService) startServer(_ context.Context) (fsm.State, error) {
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

func (c *docSearchService) shutDown(_ context.Context) (fsm.State, error) {
	if c.s != nil {
		logger.Log.Debug("Graceful stopping")
		c.s.GracefulStop()
		logger.Log.Info("Stop server")
	}

	return fsm.Finish()
}

func (c *docSearchService) Flags(fs *cli.FlagSet) {
	fs.String("listen", "Listen addr").Var(&c.Listen).Default(":8057")
	fs.String("git-data-service", "The url of git-data-service").Var(&c.GitDataService).Default("127.0.0.1:9010")
	fs.Int("workers", "The number of workers to fetching page title").Var(&c.Workers).Default(1)
	fs.Int("max-conns", "The total number of connections to fetch the external page title").Var(&c.MaxConns).Default(1)
	fs.Bool("insecure", "Insecure access to backend").Var(&c.Insecure)
	fs.String("storage-endpoint", "The endpoint of the object storage").Var(&c.StorageEndpoint)
	fs.String("storage-region", "The region name").Var(&c.StorageRegion)
	fs.String("bucket", "The bucket name that will be used").Var(&c.Bucket)
	fs.String("storage-access-key", "The access key for the object storage").Var(&c.StorageAccessKey)
	fs.String("storage-secret-access-key", "The secret access key for the object storage").Var(&c.StorageSecretAccessKey)
	fs.String("storage-ca-file", "File path that contains CA certificate").Var(&c.StorageCAFile)
}
