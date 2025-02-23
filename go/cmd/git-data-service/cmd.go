package main

import (
	"context"
	"net"
	"os"
	"strings"
	"time"

	goGit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"go.f110.dev/go-memcached/client"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/githubutil"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

type gitDataServiceCommand struct {
	*fsm.FSM
	s             *grpc.Server
	updater       *repositoryUpdater
	storageClient *storage.S3

	Listen                string
	RepositoryInitTimeout time.Duration
	GitHubClient          *githubutil.GitHubClientFactory

	StorageEndpoint            string
	StorageRegion              string
	StorageAccessKey           string
	StorageSecretAccessKey     string
	StorageSecretAccessKeyFile string
	StorageCAFile              string
	MemcachedEndpoint          string
	ListenWebhookReceiver      string

	Bucket string

	Repositories           []string
	LockFilePath           string
	RefreshInterval        time.Duration
	RefreshTimeout         time.Duration
	RefreshWorkers         int
	DisableInflatePackFile bool

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
	c.FSM.CloseContext = func() (context.Context, context.CancelFunc) {
		return ctxutil.WithTimeout(context.Background(), 5*time.Second)
	}

	return c
}

func (c *gitDataServiceCommand) Flags(fs *cli.FlagSet) {
	fs.String("listen", "Listen addr").Var(&c.Listen).Default(":8056")
	fs.String("storage-endpoint", "The endpoint of the object storage").Var(&c.StorageEndpoint)
	fs.String("storage-region", "The region name").Var(&c.StorageRegion)
	fs.String("bucket", "The bucket name that will be used").Var(&c.Bucket)
	fs.String("storage-access-key", "The access key for the object storage").Var(&c.StorageAccessKey)
	fs.String("storage-secret-access-key", "The secret access key for the object storage").Var(&c.StorageSecretAccessKey)
	fs.String("storage-secret-access-key-file", "The file path that containing the secret access key for the object storage").Var(&c.StorageSecretAccessKeyFile)
	fs.String("storage-ca-file", "File path that contains CA certificate").Var(&c.StorageCAFile)
	fs.String("memcached-endpoint", "The endpoint of memcached").Var(&c.MemcachedEndpoint)
	fs.String("listen-webhook-receiver", "Listen addr of webhook receiver.").Var(&c.ListenWebhookReceiver)

	fs.StringArray("repository", "The repository name that will be served."+
		"The value consists three elements separated by a vertical bar. The first element is the repository name. "+
		"The second element is a url for the repository. "+
		"The third element is a prefix in an object storage. (e.g. go|https://github.com/golang/go.git|golang/go)").Var(&c.Repositories)
	fs.String("lock-file-path", "The path of the lock file.  If not set the value, don't get the lock").Var(&c.LockFilePath)
	fs.Duration("refresh-interval", "The interval time for updating the repository"+
		"If set zero, interval updating is disabled.").Var(&c.RefreshInterval)
	fs.Duration("refresh-timeout", "The duration for timeout to updating repository").Var(&c.RefreshTimeout).Default(1 * time.Minute)
	fs.Int("refresh-workers", "The number of workers for updating repository").Var(&c.RefreshWorkers).Default(1)
	fs.Bool("disable-inflate-packfile", "Disable inflating packfile").Var(&c.DisableInflatePackFile)

	fs.Duration("repository-init-timeout", "The duration for timeout to initializing repository").Var(&c.RepositoryInitTimeout).Default(5 * time.Minute)
}

func (c *gitDataServiceCommand) ValidateFlagValue() error {
	if len(c.Repositories) == 0 {
		return xerrors.Define("--repository is mandatory").WithStack()
	}
	var repositories []*Repository
	for _, v := range c.Repositories {
		if strings.Index(v, "|") == -1 {
			return xerrors.Definef("--repository=%s is invalid", v).WithStack()
		}
		s := strings.Split(v, "|")
		repositories = append(repositories, &Repository{Name: s[0], URL: s[1], Prefix: s[2]})
	}
	c.repositories = repositories

	return nil
}

func (c *gitDataServiceCommand) init(ctx context.Context) (fsm.State, error) {
	if err := c.GitHubClient.Init(); err != nil {
		return fsm.Error(err)
	}

	secretAccessKey := c.StorageSecretAccessKey
	if c.StorageSecretAccessKeyFile != "" {
		b, err := os.ReadFile(c.StorageSecretAccessKeyFile)
		if err != nil {
			return fsm.Error(err)
		}
		secretAccessKey = strings.TrimSpace(string(b))
	}
	opt := storage.NewS3OptionToExternal(c.StorageEndpoint, c.StorageRegion, c.StorageAccessKey, secretAccessKey)
	opt.PathStyle = true
	opt.CACertFile = c.StorageCAFile
	storageClient := storage.NewS3(c.Bucket, opt)
	c.storageClient = storageClient

	var cachePool *client.SinglePool
	if c.MemcachedEndpoint != "" {
		cacheServer, err := client.NewServerWithMetaProtocol(ctx, "cache-1", "tcp", c.MemcachedEndpoint)
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
			ctx, cancel := ctxutil.WithTimeout(ctx, c.RepositoryInitTimeout)

			logger.Log.Info("Init repository", zap.String("name", r.Name), zap.String("url", r.URL), zap.String("prefix", r.Prefix))
			var auth *http.BasicAuth
			if v, err := c.GitHubClient.TokenProvider.Token(ctx); err == nil {
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
		if !c.DisableInflatePackFile && storer.IncludePackFile(ctx) {
			logger.Log.Info("Inflate packfile", zap.String("name", r.Name))
			if err := storer.InflatePackFile(ctx); err != nil {
				return fsm.Error(err)
			}
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

func (c *gitDataServiceCommand) startUpdater(ctx context.Context) (fsm.State, error) {
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
	go updater.Run(ctx, c.RefreshInterval)

	c.updater = updater
	return fsm.Next(stateStartWebhookReceiver)
}

func (c *gitDataServiceCommand) startWebhookReceiver(_ context.Context) (fsm.State, error) {
	if c.ListenWebhookReceiver == "" {
		return fsm.Next(stateStartServer)
	}

	go c.updater.ListenWebhookReceiver(c.ListenWebhookReceiver)
	return fsm.Next(stateStartServer)
}

func (c *gitDataServiceCommand) startServer(_ context.Context) (fsm.State, error) {
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

func (c *gitDataServiceCommand) shuttingDown(ctx context.Context) (fsm.State, error) {
	if c.s != nil {
		logger.Log.Debug("Graceful stopping gRPC server")
		c.s.GracefulStop()
		logger.Log.Info("Stop gRPC server")
	}
	if c.updater != nil {
		logger.Log.Debug("Stopping updater")
		c.updater.Stop(ctx)

	}

	return fsm.Finish()
}
