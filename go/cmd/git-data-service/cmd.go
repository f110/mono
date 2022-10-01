package main

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	goGit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/spf13/pflag"
	"go.f110.dev/go-memcached/client"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"go.f110.dev/mono/go/pkg/fsm"
	"go.f110.dev/mono/go/pkg/git"
	"go.f110.dev/mono/go/pkg/githubutil"
	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/storage"
)

type gitDataServiceCommand struct {
	*fsm.FSM
	s             *grpc.Server
	updater       *repositoryUpdater
	storageClient *storage.S3

	Listen                string
	RepositoryInitTimeout time.Duration
	GitHubClient          *githubutil.GitHubClientFactory

	StorageEndpoint        string
	StorageRegion          string
	StorageAccessKey       string
	StorageSecretAccessKey string
	StorageCAFile          string
	MemcachedEndpoint      string
	ListenWebhookReceiver  string

	Bucket string

	Repositories    []string
	LockFilePath    string
	RefreshInterval time.Duration
	RefreshTimeout  time.Duration
	RefreshWorkers  int

	repositories []*Repository
}

type Repository struct {
	Name   string
	URL    string
	Prefix string

	GoGit *goGit.Repository
}

const (
	stateInit fsm.State = iota
	stateStartUpdater
	stateStartWebhookReceiver
	stateStartServer
	stateShuttingDown
)

func newGitDataServiceCommand() *gitDataServiceCommand {
	c := &gitDataServiceCommand{GitHubClient: githubutil.NewGitHubClientFactory("", false)}
	c.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:                 c.init,
			stateStartUpdater:         c.startUpdater,
			stateStartWebhookReceiver: c.startWebhookReceiver,
			stateStartServer:          c.startServer,
			stateShuttingDown:         c.shuttingDown,
		},
		stateInit,
		stateShuttingDown,
	)

	return c
}

func (c *gitDataServiceCommand) init() (fsm.State, error) {
	if err := c.GitHubClient.Init(); err != nil {
		return fsm.Error(err)
	}

	opt := storage.NewS3OptionToExternal(c.StorageEndpoint, c.StorageRegion, c.StorageAccessKey, c.StorageSecretAccessKey)
	opt.PathStyle = true
	opt.CACertFile = c.StorageCAFile
	storageClient := storage.NewS3(c.Bucket, opt)
	c.storageClient = storageClient

	var cachePool *client.SinglePool
	if c.MemcachedEndpoint != "" {
		cacheServer, err := client.NewServerWithMetaProtocol(c.Context(), "cache-1", "tcp", c.MemcachedEndpoint)
		if err != nil {
			return fsm.Error(err)
		}
		cachePool, err = client.NewSinglePool(cacheServer)
		if err != nil {
			return fsm.Error(err)
		}
	}

	repo := make(map[string]*goGit.Repository)
	for _, r := range c.repositories {
		storer := git.NewObjectStorageStorer(storageClient, r.Prefix, cachePool)

		if ok, err := storer.Exist(); !ok && err == nil {
			ctx, cancel := context.WithTimeout(c.Context(), c.RepositoryInitTimeout)

			logger.Log.Info("Init repository", zap.String("name", r.Name), zap.String("url", r.URL), zap.String("prefix", r.Prefix))
			var auth *http.BasicAuth
			if v, err := c.GitHubClient.TokenProvider.Token(); err == nil {
				auth = &http.BasicAuth{
					Username: "octocat",
					Password: v,
				}
			}
			if _, err := git.InitObjectStorageRepository(ctx, storageClient, r.URL, r.Prefix, auth); err != nil {
				cancel()
				return fsm.Error(err)
			}
			cancel()
		} else if err != nil {
			return fsm.Error(err)
		}

		gitRepo, err := goGit.Open(storer, nil)
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}

		repo[r.Name] = gitRepo
		r.GoGit = gitRepo
	}

	s := grpc.NewServer()
	service, err := git.NewDataService(repo)
	if err != nil {
		return fsm.Error(err)
	}
	git.RegisterGitDataServer(s, service)
	healthSvc := health.NewServer()
	healthSvc.SetServingStatus("git-data", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(s, healthSvc)
	c.s = s

	return fsm.Next(stateStartUpdater)
}

func (c *gitDataServiceCommand) startUpdater() (fsm.State, error) {
	if c.RefreshInterval == 0 {
		return fsm.Next(stateStartWebhookReceiver)
	}

	updater, err := newRepositoryUpdater(
		c.storageClient,
		c.repositories,
		c.RefreshTimeout,
		c.LockFilePath,
		c.GitHubClient.TokenProvider,
		c.RefreshWorkers,
	)
	if err != nil {
		return fsm.Error(err)
	}
	logger.Log.Info("Start updater", zap.Duration("refresh_interval", c.RefreshInterval), zap.Int("workers", c.RefreshWorkers))
	go updater.Run(c.Context(), c.RefreshInterval)

	c.updater = updater
	return fsm.Next(stateStartWebhookReceiver)
}

func (c *gitDataServiceCommand) startWebhookReceiver() (fsm.State, error) {
	if c.ListenWebhookReceiver == "" {
		return fsm.Next(stateStartServer)
	}

	go c.updater.ListenWebhookReceiver(c.ListenWebhookReceiver)
	return fsm.Next(stateStartServer)
}

func (c *gitDataServiceCommand) startServer() (fsm.State, error) {
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

func (c *gitDataServiceCommand) shuttingDown() (fsm.State, error) {
	if c.s != nil {
		logger.Log.Debug("Graceful stopping gRPC server")
		c.s.GracefulStop()
		logger.Log.Info("Stop gRPC server")
	}
	if c.updater != nil {
		logger.Log.Debug("Stopping updater")
		c.updater.Stop(c.Context())

	}

	return fsm.Finish()
}

func (c *gitDataServiceCommand) Flags(fs *pflag.FlagSet) {
	fs.StringVar(&c.Listen, "listen", ":8056", "Listen addr")
	fs.StringVar(&c.StorageEndpoint, "storage-endpoint", c.StorageEndpoint, "The endpoint of the object storage")
	fs.StringVar(&c.StorageRegion, "storage-region", c.StorageRegion, "The region name")
	fs.StringVar(&c.Bucket, "bucket", c.Bucket, "The bucket name that will be used")
	fs.StringVar(&c.StorageAccessKey, "storage-access-key", c.StorageAccessKey, "The access key for the object storage")
	fs.StringVar(&c.StorageSecretAccessKey, "storage-secret-access-key", c.StorageSecretAccessKey, "The secret access key for the object storage")
	fs.StringVar(&c.StorageCAFile, "storage-ca-file", "", "File path that contains CA certificate")
	fs.StringVar(&c.MemcachedEndpoint, "memcached-endpoint", c.MemcachedEndpoint, "The endpoint of memcached")
	fs.StringVar(&c.ListenWebhookReceiver, "listen-webhook-receiver", "", "Listen addr of webhook receiver.")

	fs.StringSliceVar(&c.Repositories, "repository", nil, "The repository name that will be served."+
		"The value consists three elements separated by a vertical bar. The first element is the repository name. "+
		"The second element is a url for the repository. "+
		"The third element is a prefix in an object storage. (e.g. go|https://github.com/golang/go.git|golang/go)")
	fs.StringVar(&c.LockFilePath, "lock-file-path", "", "The path of the lock file.  If not set the value, don't get the lock")
	fs.DurationVar(&c.RefreshInterval, "refresh-interval", 0, "The interval time for updating the repository"+
		"If set zero, interval updating is disabled.")
	fs.DurationVar(&c.RefreshTimeout, "refresh-timeout", 1*time.Minute, "The duration for timeout to updating repository")
	fs.IntVar(&c.RefreshWorkers, "refresh-workers", 1, "The number of workers for updating repository")

	fs.DurationVar(&c.RepositoryInitTimeout, "repository-init-timeout", 5*time.Minute, "The duration for timeout to initializing repository")
}

func (c *gitDataServiceCommand) ValidateFlagValue() error {
	if len(c.Repositories) == 0 {
		return errors.New("--repository is mandatory")
	}
	var repositories []*Repository
	for _, v := range c.Repositories {
		if strings.Index(v, "|") == -1 {
			return xerrors.Newf("--repository=%s is invalid", v)
		}
		s := strings.Split(v, "|")
		repositories = append(repositories, &Repository{Name: s[0], URL: s[1], Prefix: s[2]})
	}
	c.repositories = repositories

	return nil
}
