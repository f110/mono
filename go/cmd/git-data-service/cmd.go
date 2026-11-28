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

const mysqlLockName = "git-data-updater"

const (
	stateInit fsm.State = iota
	stateStartService
	stateShuttingDown
)

const (
	stateUpdaterInit fsm.State = iota
	stateUpdaterRun
	stateUpdaterShuttingDown
)

type gitDataServiceCommand struct {
	*fsm.FSM
	*repositoryOptions
	grpcServer *grpc.Server

	Listen string
}

func newGitDataServiceCommand() *gitDataServiceCommand {
	c := &gitDataServiceCommand{repositoryOptions: newRepositoryOptions()}
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
	c.repositoryOptions.Flags(fs)
	fs.String("listen", "Listen addr").Var(&c.Listen).Default(":8056")
}

func (c *gitDataServiceCommand) init(ctx context.Context) (fsm.State, error) {
	backend, _, err := c.open(ctx)
	if err != nil {
		return fsm.Error(err)
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

	return fsm.Next(stateStartService)
}

func (c *gitDataServiceCommand) startService(_ context.Context) (fsm.State, error) {
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

	return fsm.Wait()
}

func (c *gitDataServiceCommand) shuttingDown(_ context.Context) (fsm.State, error) {
	if c.grpcServer != nil {
		logger.Log.Debug("Graceful stopping gRPC server")
		c.grpcServer.GracefulStop()
		logger.Log.Info("Stop gRPC server")
	}
	c.close()
	return fsm.Finish()
}

// repositoryOptions is the options to open the repositories which are shared by the server and the updater.
type repositoryOptions struct {
	packCache *git.PackfileCache

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

	Bucket string

	Repositories           []string
	DisableInflatePackFile bool
	DisableObjectCache     bool

	PackfileCacheSize int64
	PackfileCacheTTL  time.Duration

	repositories []*git.RepositoryConfig
}

func newRepositoryOptions() *repositoryOptions {
	return &repositoryOptions{GitHubClient: github.NewGitHubClientFactory("", false)}
}

func (o *repositoryOptions) Flags(fs *flag.FlagSet) {
	fs.String("dsn", "Data source name of the database which stores the repositories. "+
		"If empty, the repositories are stored in the object storage and the flags of memcached and packfile are used.").Var(&o.DSN)
	fs.String("storage-endpoint", "The endpoint of the object storage").Var(&o.StorageEndpoint)
	fs.String("storage-region", "The region name").Var(&o.StorageRegion)
	fs.String("bucket", "The bucket name that will be used").Var(&o.Bucket)
	fs.String("storage-access-key", "The access key for the object storage").Var(&o.StorageAccessKey)
	fs.String("storage-secret-access-key", "The secret access key for the object storage").Var(&o.StorageSecretAccessKey)
	fs.String("storage-secret-access-key-file", "The file path that containing the secret access key for the object storage").Var(&o.StorageSecretAccessKeyFile)
	fs.String("storage-ca-file", "File path that contains CA certificate").Var(&o.StorageCAFile)
	fs.String("memcached-endpoint", "The endpoint of memcached").Var(&o.MemcachedEndpoint)

	fs.StringArray("repository", "The repository name that will be served."+
		"The value consists three elements separated by a vertical bar. The first element is the repository name. "+
		"The second element is a url for the repository. "+
		"The third element is a prefix in an object storage. (e.g. go|https://github.com/golang/go.git|golang/go)").Var(&o.Repositories)
	fs.Bool("disable-inflate-packfile", "Disable inflating packfile").Var(&o.DisableInflatePackFile)
	fs.Bool("disable-object-cache", "Disable the in-memory cache of the objects which are read from the database (--dsn)").Var(&o.DisableObjectCache)

	fs.Int64("packfile-cache-size", "Maximum total bytes of packfiles cached in process memory. "+
		"If set zero, the in-memory packfile cache is disabled.").Var(&o.PackfileCacheSize)
	fs.Duration("packfile-cache-ttl", "TTL for in-memory cached packfiles").Var(&o.PackfileCacheTTL).Default(10 * time.Minute)

	fs.Duration("repository-init-timeout", "The duration for timeout to initializing repository").Var(&o.RepositoryInitTimeout).Default(5 * time.Minute)
	o.GitHubClient.Flags(fs)
}

func (o *repositoryOptions) ValidateFlagValue() error {
	if len(o.Repositories) == 0 {
		return xerrors.Define("--repository is mandatory").WithStack()
	}
	var repositories []*git.RepositoryConfig
	for _, v := range o.Repositories {
		if strings.Index(v, "|") == -1 {
			return xerrors.Definef("--repository=%s is invalid", v).WithStack()
		}
		s := strings.Split(v, "|")
		repositories = append(repositories, &git.RepositoryConfig{Name: s[0], URL: s[1], Prefix: s[2]})
	}
	o.repositories = repositories

	return nil
}

// open opens the backend and the repositories. It also returns the client of the object storage.
func (o *repositoryOptions) open(ctx context.Context) (git.Backend, *storage.S3, error) {
	if err := o.GitHubClient.Init(); err != nil {
		return nil, nil, err
	}

	secretAccessKey := o.StorageSecretAccessKey
	if o.StorageSecretAccessKeyFile != "" {
		b, err := os.ReadFile(o.StorageSecretAccessKeyFile)
		if err != nil {
			return nil, nil, xerrors.WithStack(err)
		}
		secretAccessKey = strings.TrimSpace(string(b))
	}
	opt := storage.NewS3OptionToExternal(o.StorageEndpoint, o.StorageRegion, o.StorageAccessKey, secretAccessKey)
	opt.PathStyle = true
	opt.CACertFile = o.StorageCAFile
	storageClient := storage.NewS3(o.Bucket, opt)

	var backend git.Backend
	if o.DSN != "" {
		conn, err := git.OpenDatabase(ctx, o.DSN, 5*time.Minute)
		if err != nil {
			return nil, nil, err
		}
		backend = &git.DatabaseBackend{Conn: conn, DisableObjectCache: o.DisableObjectCache}
	} else {
		var cachePool *client.SinglePool
		if o.MemcachedEndpoint != "" {
			cacheServer, err := client.NewServerWithMetaProtocol(ctx, "cache-1", "tcp", o.MemcachedEndpoint)
			if err != nil {
				return nil, nil, xerrors.WithStack(err)
			}
			cachePool, err = client.NewSinglePool(cacheServer)
			if err != nil {
				return nil, nil, xerrors.WithStack(err)
			}
		}
		if o.PackfileCacheSize > 0 {
			o.packCache = git.NewPackfileCache(o.PackfileCacheTTL, o.PackfileCacheTTL/2, o.PackfileCacheSize)
		}
		backend = &git.ObjectStorageBackend{Client: storageClient, CachePool: cachePool, PackCache: o.packCache, DisableInflatePackFile: o.DisableInflatePackFile}
	}

	for _, v := range o.repositories {
		if err := v.Open(ctx, backend, o.GitHubClient.TokenProvider, o.RepositoryInitTimeout); err != nil {
			return nil, nil, err
		}
	}

	return backend, storageClient, nil
}

func (o *repositoryOptions) close() {
	if o.packCache != nil {
		o.packCache.Close()
	}
}

// updaterCommand updates the repositories periodically.
// The process exits when it loses the lock so that the lock is acquired again by the restarted process.
type updaterCommand struct {
	*fsm.FSM
	*repositoryOptions
	updater       *git.Updater
	locker        leaderelection.Locker
	webhookServer *http.Server

	ListenWebhookReceiver string
	LockFilePath          string
	RefreshInterval       time.Duration
	RefreshTimeout        time.Duration
	RefreshWorkers        int
	RepackInterval        time.Duration
}

func newUpdaterCommand() *updaterCommand {
	c := &updaterCommand{repositoryOptions: newRepositoryOptions()}
	c.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateUpdaterInit:         c.init,
			stateUpdaterRun:          c.run,
			stateUpdaterShuttingDown: c.shuttingDown,
		},
		stateUpdaterInit,
		stateUpdaterShuttingDown,
	)
	c.FSM.CloseContext = func() (context.Context, context.CancelFunc) {
		return ctxutil.WithTimeout(context.Background(), 5*time.Second)
	}

	return c
}

func (c *updaterCommand) Flags(fs *flag.FlagSet) {
	c.repositoryOptions.Flags(fs)
	fs.String("listen-webhook-receiver", "Listen addr of webhook receiver.").Var(&c.ListenWebhookReceiver)
	fs.String("lock-file-path", "The path of the lock file in the object storage. If not set the value, don't get the lock. "+
		"If --dsn is set, the named lock of the database is used instead").Var(&c.LockFilePath)
	fs.Duration("refresh-interval", "The interval time for updating the repository").Var(&c.RefreshInterval)
	fs.Duration("refresh-timeout", "The duration for timeout to updating repository").Var(&c.RefreshTimeout).Default(1 * time.Minute)
	fs.Int("refresh-workers", "The number of workers for updating repository").Var(&c.RefreshWorkers).Default(1)
	fs.Duration("repack-interval", "The interval for repacking the repositories which are stored in the database (--dsn). "+
		"If set zero, the repack is disabled.").Var(&c.RepackInterval).Default(24 * time.Hour)
}

func (c *updaterCommand) ValidateFlagValue() error {
	if c.RefreshInterval <= 0 {
		return xerrors.Define("--refresh-interval is mandatory").WithStack()
	}
	return c.repositoryOptions.ValidateFlagValue()
}

func (c *updaterCommand) init(ctx context.Context) (fsm.State, error) {
	backend, storageClient, err := c.open(ctx)
	if err != nil {
		return fsm.Error(err)
	}

	c.updater = git.NewUpdater(backend, c.GitHubClient.TokenProvider, git.NewStaticRepositoryProvider(c.repositories), c.RefreshWorkers).
		SetInterval(c.RefreshInterval).
		SetTimeout(c.RefreshTimeout).
		SetInitTimeout(c.RepositoryInitTimeout).
		SetRepackInterval(c.RepackInterval)
	if b, ok := backend.(*git.DatabaseBackend); ok {
		c.locker = leaderelection.NewMySQL(b.Conn, mysqlLockName)
	} else if c.LockFilePath != "" {
		l, err := leaderelection.NewObjectStorage(storageClient, c.LockFilePath)
		if err != nil {
			return fsm.Error(err)
		}
		c.locker = l
	}

	return fsm.Next(stateUpdaterRun)
}

func (c *updaterCommand) run(ctx context.Context) (fsm.State, error) {
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

	updaterCtx := ctx
	if c.locker != nil {
		lockCtx, err := c.locker.Lock(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return fsm.Wait()
			}
			return fsm.Error(err)
		}
		updaterCtx = lockCtx
	}
	c.updater.Run(updaterCtx)
	if ctx.Err() != nil {
		return fsm.Wait()
	}
	return fsm.Error(xerrors.Define("lost the lock").WithStack())
}

func (c *updaterCommand) shuttingDown(ctx context.Context) (fsm.State, error) {
	if c.webhookServer != nil {
		c.webhookServer.Shutdown(ctx)
	}
	c.close()
	return fsm.Finish()
}
