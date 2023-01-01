package main

import (
	"context"
	"net"
	"time"

	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"go.f110.dev/mono/go/docutil"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/grpcutil"
	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/storage"
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

func (c *docSearchService) init() (fsm.State, error) {
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
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Minute)
	defer cancel()
	if err := service.Initialize(ctx, c.Workers, c.MaxConns); err != nil {
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
	fs.IntVar(&c.Workers, "workers", 1, "The number of workers to fetching page title")
	fs.IntVar(&c.MaxConns, "max-conns", 1, "The total number of connections to fetch the external page title")
	fs.BoolVar(&c.Insecure, "insecure", false, "Insecure access to backend")
	fs.StringVar(&c.StorageEndpoint, "storage-endpoint", c.StorageEndpoint, "The endpoint of the object storage")
	fs.StringVar(&c.StorageRegion, "storage-region", c.StorageRegion, "The region name")
	fs.StringVar(&c.Bucket, "bucket", c.Bucket, "The bucket name that will be used")
	fs.StringVar(&c.StorageAccessKey, "storage-access-key", c.StorageAccessKey, "The access key for the object storage")
	fs.StringVar(&c.StorageSecretAccessKey, "storage-secret-access-key", c.StorageSecretAccessKey, "The secret access key for the object storage")
	fs.StringVar(&c.StorageCAFile, "storage-ca-file", "", "File path that contains CA certificate")
}
