package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"go.f110.dev/go-memcached/client"
	"go.f110.dev/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/githubutil"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/storage"
)

type gitDataServiceCommand struct {
	*fsm.FSM
	grpcServer    *grpc.Server
	updater       *git.Updater
	webhookServer *http.Server

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

	repositories []*git.RepositoryConfig
}

const (
	stateInit fsm.State = iota
	stateStartService
	stateShuttingDown
)

func newGitDataServiceCommand() *gitDataServiceCommand {
	c := &gitDataServiceCommand{GitHubClient: githubutil.NewGitHubClientFactory("", false)}
	c.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:         c.init,
			stateStartService: c.startService,
			stateShuttingDown: c.shuttingDown,
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
	var repositories []*git.RepositoryConfig
	for _, v := range c.Repositories {
		if strings.Index(v, "|") == -1 {
			return xerrors.Definef("--repository=%s is invalid", v).WithStack()
		}
		s := strings.Split(v, "|")
		repositories = append(repositories, &git.RepositoryConfig{Name: s[0], URL: s[1], Prefix: s[2]})
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

	repoMap := make(map[string]*git.RepositoryConfig)
	for _, v := range c.repositories {
		if err := v.Open(ctx, storageClient, cachePool, c.GitHubClient.TokenProvider, c.RepositoryInitTimeout, c.DisableInflatePackFile); err != nil {
			return fsm.Error(err)
		}
		repoMap[v.Name] = v
	}

	service, err := git.NewDataService(repoMap)
	if err != nil {
		return fsm.Error(err)
	}
	s := grpc.NewServer()
	git.RegisterGitDataServer(s, service)
	healthSvc := health.NewServer()
	healthSvc.SetServingStatus("git-data", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(s, healthSvc)
	c.grpcServer = s

	if c.RefreshInterval > 0 {
		u, err := git.NewUpdater(storageClient, c.GitHubClient.TokenProvider, c.repositories, c.LockFilePath, c.RefreshWorkers)
		if err != nil {
			return fsm.Error(err)
		}
		u.SetInterval(c.RefreshInterval).
			SetTimeout(c.RefreshTimeout).
			SetCachePool(cachePool).
			SetInitTimeout(c.RepositoryInitTimeout).
			SetDisableInflatePackFile(c.DisableInflatePackFile).
			SetDataService(service)
		c.updater = u
	}

	return fsm.Next(stateStartService)
}

func (c *gitDataServiceCommand) startService(ctx context.Context) (fsm.State, error) {
	lis, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	slogger.Log.Info("Start listen", slog.String("addr", c.Listen))
	go func() {
		if err := c.grpcServer.Serve(lis); err != nil {
			slogger.Log.Error("gRPC server returns error", slogger.E(err))
		}
	}()

	if c.updater != nil {
		go c.updater.Run(ctx)

		if c.ListenWebhookReceiver != "" {
			c.webhookServer = &http.Server{
				Addr:    c.ListenWebhookReceiver,
				Handler: c.updater,
			}
			slogger.Log.Info("Start webhook receiver", slog.String("addr", c.ListenWebhookReceiver))
			go func() {
				if err := c.webhookServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					slogger.Log.Info("Stop webhook receiver", slogger.E(err))
				}
			}()
		}
	}

	return fsm.Wait()
}

func (c *gitDataServiceCommand) shuttingDown(ctx context.Context) (fsm.State, error) {
	if c.grpcServer != nil {
		slogger.Log.Debug("Graceful stopping gRPC server")
		c.grpcServer.GracefulStop()
		slogger.Log.Info("Stop gRPC server")
	}
	if c.webhookServer != nil {
		c.webhookServer.Shutdown(ctx)
	}
	return fsm.Finish()
}
