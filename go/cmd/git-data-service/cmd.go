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

	"go.f110.dev/mono/go/cli/flag"
	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/github"
	"go.f110.dev/mono/go/leaderelection"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

type gitDataServiceCommand struct {
	*fsm.FSM
	grpcServer    *grpc.Server
	updater       *git.Updater
	locker        leaderelection.Locker
	webhookServer *http.Server
	packCache     *git.PackfileCache

	Listen                string
	DSN                   string
	RepositoryInitTimeout time.Duration
	GitHubClient          *github.GitHubClientFactory

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
	RepackInterval         time.Duration
	DisableInflatePackFile bool
	DisableObjectCache     bool

	PackfileCacheSize int64
	PackfileCacheTTL  time.Duration

	repositories []*git.RepositoryConfig
}

const (
	stateInit fsm.State = iota
	stateStartService
	stateShuttingDown
)

func newGitDataServiceCommand() *gitDataServiceCommand {
	c := &gitDataServiceCommand{GitHubClient: github.NewGitHubClientFactory("", false)}
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

func (c *gitDataServiceCommand) Flags(fs *flag.FlagSet) {
	fs.String("listen", "Listen addr").Var(&c.Listen).Default(":8056")
	fs.String("dsn", "Data source name of the database which stores the repositories. "+
		"If empty, the repositories are stored in the object storage and the flags of memcached and packfile are used.").Var(&c.DSN)
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
	fs.Duration("repack-interval", "The interval for repacking the repositories which are stored in the database (--dsn). "+
		"If set zero, the repack is disabled.").Var(&c.RepackInterval).Default(24 * time.Hour)
	fs.Bool("disable-inflate-packfile", "Disable inflating packfile").Var(&c.DisableInflatePackFile)
	fs.Bool("disable-object-cache", "Disable the in-memory cache of the objects which are read from the database (--dsn)").Var(&c.DisableObjectCache)

	fs.Int64("packfile-cache-size", "Maximum total bytes of packfiles cached in process memory. "+
		"If set zero, the in-memory packfile cache is disabled.").Var(&c.PackfileCacheSize)
	fs.Duration("packfile-cache-ttl", "TTL for in-memory cached packfiles").Var(&c.PackfileCacheTTL).Default(10 * time.Minute)

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

	var backend git.Backend
	if c.DSN != "" {
		conn, err := git.OpenDatabase(ctx, c.DSN, 5*time.Minute)
		if err != nil {
			return fsm.Error(err)
		}
		backend = &git.DatabaseBackend{Conn: conn, DisableObjectCache: c.DisableObjectCache}
	} else {
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
		if c.PackfileCacheSize > 0 {
			c.packCache = git.NewPackfileCache(c.PackfileCacheTTL, c.PackfileCacheTTL/2, c.PackfileCacheSize)
		}
		backend = &git.ObjectStorageBackend{Client: storageClient, CachePool: cachePool, PackCache: c.packCache, DisableInflatePackFile: c.DisableInflatePackFile}
	}

	for _, v := range c.repositories {
		if err := v.Open(ctx, backend, c.GitHubClient.TokenProvider, c.RepositoryInitTimeout); err != nil {
			return fsm.Error(err)
		}
	}

	service, err := git.NewDataService(backend, c.GitHubClient.TokenProvider, git.NewStaticRepositoryProvider(c.repositories))
	if err != nil {
		return fsm.Error(err)
	}
	service.SetInitTimeout(c.RepositoryInitTimeout)
	s := grpc.NewServer()
	git.RegisterGitDataServer(s, service)
	healthSvc := health.NewServer()
	healthSvc.SetServingStatus("git-data", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(s, healthSvc)
	c.grpcServer = s

	if c.RefreshInterval > 0 {
		c.updater = git.NewUpdater(backend, c.GitHubClient.TokenProvider, git.NewStaticRepositoryProvider(c.repositories), c.RefreshWorkers).
			SetInterval(c.RefreshInterval).
			SetTimeout(c.RefreshTimeout).
			SetInitTimeout(c.RepositoryInitTimeout).
			SetRepackInterval(c.RepackInterval)
		if c.LockFilePath != "" {
			l, err := leaderelection.NewObjectStorage(storageClient, c.LockFilePath)
			if err != nil {
				return fsm.Error(err)
			}
			c.locker = l
		}
	}

	return fsm.Next(stateStartService)
}

func (c *gitDataServiceCommand) startService(ctx context.Context) (fsm.State, error) {
	lis, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	logger.Log.Info("Start listen", slog.String("addr", c.Listen))
	go func() {
		if err := c.grpcServer.Serve(lis); err != nil {
			logger.Log.Error("gRPC server returns error", logger.E(err))
		}
	}()

	if c.updater != nil {
		go func() {
			updaterCtx := ctx
			if c.locker != nil {
				lockCtx, err := c.locker.Lock(ctx)
				if err != nil {
					logger.Log.Error("Failed to get the lock", logger.E(err))
					return
				}
				updaterCtx = lockCtx
			}
			c.updater.Run(updaterCtx)
		}()

		if c.ListenWebhookReceiver != "" {
			c.webhookServer = &http.Server{
				Addr:    c.ListenWebhookReceiver,
				Handler: c.updater,
			}
			logger.Log.Info("Start webhook receiver", slog.String("addr", c.ListenWebhookReceiver))
			go func() {
				if err := c.webhookServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					logger.Log.Info("Stop webhook receiver", logger.E(err))
				}
			}()
		}
	}

	return fsm.Wait()
}

func (c *gitDataServiceCommand) shuttingDown(ctx context.Context) (fsm.State, error) {
	if c.grpcServer != nil {
		logger.Log.Debug("Graceful stopping gRPC server")
		c.grpcServer.GracefulStop()
		logger.Log.Info("Stop gRPC server")
	}
	if c.webhookServer != nil {
		c.webhookServer.Shutdown(ctx)
	}
	if c.packCache != nil {
		c.packCache.Close()
	}
	return fsm.Finish()
}
