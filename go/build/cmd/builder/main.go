package builder

import (
	"context"
	"database/sql"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/go-github/v85/github"
	"github.com/google/uuid"
	"go.f110.dev/go-memcached/client"
	"go.f110.dev/kubeproto/go/k8sclient"
	"go.f110.dev/protoc-ddl/probe"
	"go.f110.dev/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	k8sleaderelection "k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	secretstoreclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"

	"go.f110.dev/mono/go/build/api"
	"go.f110.dev/mono/go/build/coordinator"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/gc"
	"go.f110.dev/mono/go/build/mcp"
	"go.f110.dev/mono/go/build/releasewatcher"
	"go.f110.dev/mono/go/build/watcher"
	"go.f110.dev/mono/go/build/webhook"
	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/cli/flag"
	"go.f110.dev/mono/go/ctxutil"
	_ "go.f110.dev/mono/go/database/querylog"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/git"
	githubutil "go.f110.dev/mono/go/github"
	ghwebhook "go.f110.dev/mono/go/github/webhook"
	"go.f110.dev/mono/go/grpcutil"
	"go.f110.dev/mono/go/http/authproxy"
	"go.f110.dev/mono/go/leaderelection"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/netutil"
	"go.f110.dev/mono/go/storage"
	"go.f110.dev/mono/go/vault"
)

type Options struct {
	Id                       string // Identity name. This name used to leader election.
	DSN                      string // DataSourceName.
	Namespace                string
	EnableLeaderElection     bool
	LeaseLockName            string
	LeaseLockNamespace       string
	GitHubClient             *githubutil.GitHubClientFactory
	GithubAppSecretName      string
	MinIOEndpoint            string
	MinIOName                string
	MinIONamespace           string
	MinIOPort                int
	MinIOBucket              string
	MinIOAccessKey           string
	MinIOSecretAccessKey     string
	MinIOSecretAccessKeyFile string
	ServiceAccountTokenFile  string
	VaultAddr                string
	VaultTokenFile           string
	VaultK8sAuthPath         string
	VaultK8sAuthRole         string

	Addr                           string
	DashboardUrl                   string // URL of dashboard that can access people via browser
	BuilderApiUrl                  string // URL of the api of builder.
	RemoteCache                    string // If not empty, This value will passed to Bazel through --remote_cache argument.
	RemoteAssetApi                 bool   // Use Remote Asset API. An api is experimental and depends on remote cache with gRPC.
	BazelImage                     string
	UseBazelisk                    bool
	DefaultBazelVersion            string
	BazelMirrorURL                 string
	BazelMirrorEndpoint            string
	BazelMirrorName                string
	BazelMirrorNamespace           string
	BazelMirrorPort                int
	BazelMirrorBucket              string
	BazelMirrorPrefix              string
	BazelMirrorAccessKey           string
	BazelMirrorSecretAccessKey     string
	BazelMirrorSecretAccessKeyFile string
	CentralRegistryMirrorURL       string
	ArtifactUpload                 bool   // Upload collected build artifacts to the OCI registry named in their repository.
	ArtifactRegistryInsecure       bool   // Allow accessing the artifact registries over plain HTTP.
	ArtifactBuilderAddr            string // gRPC address of the builder API the collect sidecar uploads to.
	NATSURL                        string // URL of the NATS server to publish the events of the artifacts. When empty, no event is published.
	NATSStreamName                 string
	NATSSubject                    string
	SidecarImage                   string
	CLIImage                       string
	PublicKeyURL                   string // URL of the public key of heimdallr. When empty, MCP is not served.
	PullAlways                     bool
	TaskCPULimit                   string
	TaskMemoryLimit                string
	WithGC                         bool
	ExcludeNodes                   []string
	ExternalReleasePollInterval    time.Duration
	EventReconcileInterval         time.Duration
	EventMaxProcessingDuration     time.Duration
	TaskScheduleInterval           time.Duration

	GitDataServiceURL                 string
	GitDataUpdaterURL                 string
	CloneFromGitDataService           bool
	GitDataListen                     string
	GitDataStorageEndpoint            string
	GitDataStorageRegion              string
	GitDataStorageAccessKey           string
	GitDataStorageSecretAccessKey     string
	GitDataStorageSecretAccessKeyFile string
	GitDataStorageCAFile              string
	GitDataBucket                     string
	GitDataMemcachedEndpoint          string
	GitDataDisableInflatePackFile     bool
	GitDataRepositoryInitTimeout      time.Duration
	GitDataPackfileCacheSize          int64
	GitDataPackfileCacheTTL           time.Duration
	GitDataPackfileCacheSweepInterval time.Duration

	Dev   bool
	Debug bool
	// Kubeconfig is the path to a kubeconfig file. When empty, the in-cluster
	// config is used. Set it to point at the local kwok cluster in development.
	Kubeconfig string
	// EmulateJobControl enables the dev-only CompleteTask RPC against a
	// kwok-emulated kube-apiserver. EmulateJobFixtureDir points at the fixture
	// logs served as pod logs; it defaults to go/build/dev/jobresult under the
	// bazel workspace.
	EmulateJobControl    bool
	EmulateJobFixtureDir string
}

const (
	stateInit fsm.State = iota
	stateCheckMigrate
	stateSetup
	stateStartGitDataService
	stateStartApiServer
	stateLeaderElection
	stateStartWorker
	stateShutdown
)

const (
	stateGitDataUpdaterInit fsm.State = iota
	stateGitDataUpdaterRun
	stateGitDataUpdaterShutdown
)

type process struct {
	FSM    *fsm.FSM
	opt    Options
	ctx    context.Context
	cancel context.CancelFunc

	ghClient              *github.Client
	coreClient            *k8sclient.Set
	k8sClient             kubernetes.Interface
	secretStoreClient     *secretstoreclient.Clientset
	restCfg               *rest.Config
	coreInformerFactory   *k8sclient.InformerFactory
	dao                   dao.Options
	storageOpt            storage.S3Options
	bazelMirrorStorageOpt storage.S3Options
	vaultClient           *vault.Client

	bazelBuilder       *coordinator.BazelBuilder
	apiServer          *api.Api
	eventProcessor     *ghwebhook.EventProcessor
	gitDataGRPCServer  *grpc.Server
	gitDataPackCache   *git.PackfileCache
	gitDataConn        *grpc.ClientConn
	gitDataClient      git.GitDataClient
	gitDataUpdaterConn *grpc.ClientConn
}

func newProcess(opt Options) *process {
	ctx, cancel := ctxutil.WithCancel(context.Background())
	p := &process{ctx: ctx, cancel: cancel, opt: opt}
	p.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:                p.init,
			stateCheckMigrate:        p.checkMigrate,
			stateSetup:               p.setup,
			stateStartGitDataService: p.startGitDataService,
			stateStartApiServer:      p.startApiServer,
			stateLeaderElection:      p.leaderElection,
			stateStartWorker:         p.startWorker,
			stateShutdown:            p.shutdown,
		},
		stateInit,
		stateShutdown,
	)
	p.FSM.CloseContext = func() (context.Context, context.CancelFunc) {
		return ctxutil.WithTimeout(context.Background(), 10*time.Second)
	}

	return p
}

func (p *process) init(ctx context.Context) (fsm.State, error) {
	if err := p.opt.GitHubClient.Init(); err != nil {
		return fsm.Error(err)
	}
	p.ghClient = p.opt.GitHubClient.REST

	if p.opt.Dev {
		logger.Log.Info("Start without kube-apiserver. All of integrations with kube-apiserver will be disabled.")
	} else {
		cfg, err := clientcmd.BuildConfigFromFlags("", p.opt.Kubeconfig)
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		p.restCfg = cfg

		kubeClient, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		p.k8sClient = kubeClient
		coreClient, err := k8sclient.NewSet(cfg)
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		p.coreClient = coreClient
		p.coreInformerFactory = k8sclient.NewInformerFactory(coreClient, k8sclient.NewInformerCache(), p.opt.Namespace, 30*time.Second)

		ssClient, err := secretstoreclient.NewForConfig(cfg)
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		p.secretStoreClient = ssClient
	}

	conn, err := openDatabase(p.opt.DSN)
	if err != nil {
		return fsm.Error(err)
	}
	p.dao = dao.NewOptions(conn)

	if p.opt.VaultAddr != "" {
		if p.opt.VaultTokenFile != "" {
			token, err := os.ReadFile(p.opt.VaultTokenFile)
			if err != nil {
				return fsm.Error(err)
			}
			vc, err := vault.NewClient(p.opt.VaultAddr, string(token))
			if err != nil {
				return fsm.Error(err)
			}
			p.vaultClient = vc
		} else if _, err := os.Stat(p.opt.ServiceAccountTokenFile); err == nil {
			vc, err := vault.NewClientAsK8SServiceAccount(ctx, p.opt.VaultAddr, p.opt.VaultK8sAuthPath, p.opt.VaultK8sAuthRole, p.opt.ServiceAccountTokenFile)
			if err != nil {
				logger.Log.Debug("Can not log in", logger.Verbose(err))
				return fsm.Error(err)
			}
			p.vaultClient = vc
		}
	}

	if p.opt.MinIOSecretAccessKeyFile != "" {
		b, err := os.ReadFile(p.opt.MinIOSecretAccessKeyFile)
		if err != nil {
			return fsm.Error(err)
		}
		p.opt.MinIOSecretAccessKey = strings.TrimSpace(string(b))
	}
	if p.opt.BazelMirrorSecretAccessKeyFile != "" {
		b, err := os.ReadFile(p.opt.BazelMirrorSecretAccessKeyFile)
		if err != nil {
			return fsm.Error(err)
		}
		p.opt.BazelMirrorSecretAccessKey = strings.TrimSpace(string(b))
	}

	return fsm.Next(stateCheckMigrate)
}

func (p *process) checkMigrate(ctx context.Context) (fsm.State, error) {
	logger.Log.Debug("Check migration")
	pr := probe.NewProbe(p.dao.RawConnection)
	ticker := time.NewTicker(1 * time.Second)
	timeout := time.After(5 * time.Minute)
Wait:
	for {
		select {
		case <-ticker.C:
			if pr.Ready(ctx, database.SchemaHash) {
				break Wait
			}
		case <-timeout:
			return fsm.Error(xerrors.Define("waiting db migration was timed out").WithStack())
		}
	}
	return fsm.Next(stateSetup)
}

func (p *process) setup(_ context.Context) (fsm.State, error) {
	var storageOpt storage.S3Options
	if p.opt.MinIOEndpoint != "" {
		storageOpt = storage.NewS3OptionToExternal(p.opt.MinIOEndpoint, "", p.opt.MinIOAccessKey, p.opt.MinIOSecretAccessKey)
	} else {
		storageOpt = storage.NewS3OptionViaService(
			p.coreClient,
			p.opt.MinIOName,
			p.opt.MinIONamespace,
			p.opt.MinIOPort,
			p.opt.MinIOAccessKey,
			p.opt.MinIOSecretAccessKey,
			p.opt.Dev,
		)
	}
	storageOpt.PathStyle = true
	p.storageOpt = storageOpt

	var bazelMirrorStorageOpt storage.S3Options
	if p.opt.BazelMirrorEndpoint != "" {
		bazelMirrorStorageOpt = storage.NewS3OptionToExternal(p.opt.BazelMirrorEndpoint, "", p.opt.BazelMirrorAccessKey, p.opt.BazelMirrorSecretAccessKey)
	} else {
		bazelMirrorStorageOpt = storage.NewS3OptionViaService(
			p.coreClient,
			p.opt.BazelMirrorName,
			p.opt.BazelMirrorNamespace,
			p.opt.BazelMirrorPort,
			p.opt.BazelMirrorAccessKey,
			p.opt.BazelMirrorSecretAccessKey,
			p.opt.Dev,
		)
	}
	bazelMirrorStorageOpt.PathStyle = true
	p.bazelMirrorStorageOpt = bazelMirrorStorageOpt

	var kubernetesOpt coordinator.KubernetesOptions
	if p.coreInformerFactory != nil && p.coreClient != nil {
		batchInformerFactory := k8sclient.NewBatchV1Informer(p.coreInformerFactory.Cache(), p.coreClient.BatchV1, p.opt.Namespace, 30*time.Second)
		coreInformerFactory := k8sclient.NewCoreV1Informer(p.coreInformerFactory.Cache(), p.coreClient.CoreV1, p.opt.Namespace, 30*time.Second)
		kubernetesOpt = coordinator.NewKubernetesOptions(
			batchInformerFactory,
			coreInformerFactory,
			p.coreClient,
			p.secretStoreClient,
			p.restCfg,
			p.opt.TaskCPULimit,
			p.opt.TaskMemoryLimit,
		)
	}
	bazelOpt := coordinator.NewBazelOptions(
		p.opt.RemoteCache,
		p.opt.RemoteAssetApi,
		p.opt.SidecarImage,
		p.opt.BazelImage,
		p.opt.UseBazelisk,
		p.opt.DefaultBazelVersion,
		p.opt.BazelMirrorURL,
		p.opt.CentralRegistryMirrorURL,
		p.opt.PullAlways,
		p.opt.GitHubClient.AppID,
		p.opt.GitHubClient.InstallationID,
		p.opt.GithubAppSecretName,
		p.opt.GitDataServiceURL,
		p.opt.CloneFromGitDataService,
		p.opt.ArtifactBuilderAddr,
	)
	c, err := coordinator.NewBazelBuilder(
		p.opt.DashboardUrl,
		kubernetesOpt,
		p.dao,
		p.opt.Namespace,
		p.ghClient,
		p.opt.MinIOBucket,
		storageOpt,
		bazelOpt,
		p.vaultClient,
		p.opt.ExcludeNodes,
		p.opt.Dev,
	)
	if err != nil {
		logger.Log.Error("Failed create BazelBuilder", logger.E(err))
		return fsm.Error(xerrors.WithStack(err))
	}
	if p.opt.EmulateJobControl {
		fixtureDir := p.opt.EmulateJobFixtureDir
		if fixtureDir == "" {
			fixtureDir = filepath.Join(os.Getenv("BUILD_WORKSPACE_DIRECTORY"), "go/build/dev/jobresult")
		}
		c.EnableEmulateJobControl(fixtureDir)
		logger.Log.Info("Emulate job control is enabled", slog.String("fixture_dir", fixtureDir))
	}
	p.bazelBuilder = c

	if p.opt.GitDataListen == "" {
		return fsm.Next(stateStartApiServer)
	}
	return fsm.Next(stateStartGitDataService)
}

func (p *process) startGitDataService(ctx context.Context) (fsm.State, error) {
	// A zero cache size disables the in-memory packfile cache.
	if p.opt.GitDataPackfileCacheSize > 0 {
		p.gitDataPackCache = git.NewPackfileCache(p.opt.GitDataPackfileCacheTTL, p.opt.GitDataPackfileCacheSweepInterval, p.opt.GitDataPackfileCacheSize)
	}
	backend, _, err := newGitDataBackend(ctx, p.opt, p.gitDataPackCache)
	if err != nil {
		return fsm.Error(err)
	}

	repositoryProvider := newGitDataRepositories(func(ctx context.Context) ([]*database.SourceRepository, error) {
		rows, err := p.dao.Repository.ListAll(ctx)
		return rows, xerrors.WithStack(err)
	})
	service, err := git.NewDataService(backend, p.opt.GitHubClient.TokenProvider, repositoryProvider)
	if err != nil {
		return fsm.Error(err)
	}
	service.SetInitTimeout(p.opt.GitDataRepositoryInitTimeout)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcutil.WithServerLogging()))
	git.RegisterGitDataServer(grpcServer, service)
	healthSvc := health.NewServer()
	healthSvc.SetServingStatus("git-data", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, healthSvc)
	p.gitDataGRPCServer = grpcServer

	lis, err := net.Listen("tcp", p.opt.GitDataListen)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	logger.Log.Info("Start git-data-service", slog.String("addr", p.opt.GitDataListen))
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Log.Error("git-data-service returns error", logger.E(err))
		}
	}()

	return fsm.Next(stateStartApiServer)
}

func (p *process) startApiServer(ctx context.Context) (fsm.State, error) {
	if p.opt.GitDataServiceURL != "" {
		conn, err := grpc.NewClient(p.opt.GitDataServiceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		p.gitDataConn = conn
		p.gitDataClient = git.NewGitDataClient(conn)
		p.bazelBuilder.GitDataClient(p.gitDataClient)
		logger.Log.Info("Use git-data-service for reading repository data", slog.String("addr", p.opt.GitDataServiceURL))
	}

	// Avoid handing the push reconciler a typed-nil *git.UpdaterClient: that would
	// satisfy the GitSyncer interface as a non-nil value and panic when called.
	var gitSyncer webhook.GitSyncer
	if p.opt.GitDataUpdaterURL != "" {
		conn, err := grpc.NewClient(p.opt.GitDataUpdaterURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		p.gitDataUpdaterConn = conn
		gitSyncer = git.NewUpdaterClient(conn)
		logger.Log.Info("Use git-data updater for syncing the pushed repository", slog.String("addr", p.opt.GitDataUpdaterURL))
	}
	p.eventProcessor = webhook.NewEventProcessor(p.dao, p.ghClient, p.bazelBuilder, gitSyncer, p.gitDataClient, p.opt.EventReconcileInterval, p.opt.EventMaxProcessingDuration)

	var artifactRegistry api.ArtifactRegistry
	if p.opt.ArtifactUpload {
		artifactRegistry = api.NewCraneArtifactRegistry(p.opt.ArtifactRegistryInsecure)
	}
	var artifactEventPublisher api.ArtifactEventPublisher
	if p.opt.ArtifactUpload && p.opt.NATSURL != "" {
		pub, err := api.NewNATSArtifactEventPublisher(p.opt.NATSURL, p.opt.NATSStreamName, p.opt.NATSSubject)
		if err != nil {
			return fsm.Error(err)
		}
		artifactEventPublisher = pub
	}
	var newMCPHandler func(api.APIServer) http.Handler
	if p.opt.PublicKeyURL != "" {
		publicKey, err := authproxy.FetchPublicKey(ctx, p.opt.PublicKeyURL)
		if err != nil {
			return fsm.Error(err)
		}
		newMCPHandler = func(s api.APIServer) http.Handler {
			return mcp.New(s, func(next http.Handler) http.Handler { return authproxy.Authenticate(publicKey, next) })
		}
	} else {
		logger.Log.Info("Skip serving MCP because --public-key-url is not specified")
	}
	apiServer, err := api.NewApi(
		p.opt.Addr,
		p.bazelBuilder,
		p.dao,
		p.ghClient,
		p.gitDataClient,
		storage.NewS3(p.opt.BazelMirrorBucket, p.bazelMirrorStorageOpt),
		storage.NewS3(p.opt.MinIOBucket, p.storageOpt),
		p.opt.BazelMirrorPrefix,
		p.eventProcessor,
		buildServerConfig(p.opt),
		artifactRegistry,
		artifactEventPublisher,
		newMCPHandler,
	)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	p.apiServer = apiServer

	go func() {
		logger.Log.Info("Start API Server", slog.String("addr", p.apiServer.Addr))
		p.apiServer.ListenAndServe()
	}()

	return stateLeaderElection, nil
}

// leaderElection will get the lock.
// Next state: stateStartWorker
func (p *process) leaderElection(_ context.Context) (fsm.State, error) {
	if p.k8sClient == nil || p.opt.LeaseLockName == "" || p.opt.LeaseLockNamespace == "" {
		logger.Log.Info("Skip leader election")
		return fsm.Next(stateStartWorker)
	}

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      p.opt.LeaseLockName,
			Namespace: p.opt.LeaseLockNamespace,
		},
		Client: p.k8sClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: p.opt.Id,
		},
	}

	elected := make(chan struct{})
	e, err := k8sleaderelection.NewLeaderElector(k8sleaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   30 * time.Second,
		RenewDeadline:   15 * time.Second,
		RetryPeriod:     5 * time.Second,
		Callbacks: k8sleaderelection.LeaderCallbacks{
			OnStartedLeading: func(_ context.Context) {
				close(elected)
			},
			OnStoppedLeading: func() {
				p.FSM.Shutdown()
			},
			OnNewLeader: func(_ string) {},
		},
	})
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	go e.Run(p.ctx)

	select {
	case <-elected:
	case <-p.ctx.Done():
		return fsm.Error(p.ctx.Err())
	}

	return fsm.Next(stateStartWorker)
}

func (p *process) startWorker(_ context.Context) (fsm.State, error) {
	if p.coreInformerFactory != nil {
		batchInformerFactory := k8sclient.NewBatchV1Informer(p.coreInformerFactory.Cache(), p.coreClient.BatchV1, p.opt.Namespace, 30*time.Second)
		jobWatcher := watcher.NewJobWatcher(batchInformerFactory)

		p.coreInformerFactory.Run(p.ctx)

		go func() {
			logger.Log.Info("Start JobWatcher")
			if err := jobWatcher.Run(p.ctx, 1); err != nil {
				logger.Log.Error("Error occurred at JobWatcher", logger.E(err))
				return
			}
		}()
	}

	if p.opt.WithGC {
		g := gc.NewGC(1*time.Hour, p.dao, p.opt.MinIOBucket, p.storageOpt)
		go func() {
			logger.Log.Info("Start GC")
			g.Start()
		}()
	}

	interval := p.opt.ExternalReleasePollInterval
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	manager := releasewatcher.NewManager(p.bazelBuilder, p.dao, p.ghClient, nil, interval)
	go manager.Start(p.ctx)

	taskScheduleInterval := p.opt.TaskScheduleInterval
	if taskScheduleInterval <= 0 {
		taskScheduleInterval = 30 * time.Second
	}
	go p.bazelBuilder.Run(p.ctx, taskScheduleInterval)

	go p.eventProcessor.Run(p.ctx)

	return fsm.Wait()
}

func (p *process) shutdown(ctx context.Context) (fsm.State, error) {
	logger.Log.Info("Shutting down")
	if p.apiServer != nil {
		p.apiServer.Shutdown(ctx)
		logger.Log.Info("Shutdown API Server")
	}
	if p.gitDataGRPCServer != nil {
		p.gitDataGRPCServer.GracefulStop()
		logger.Log.Info("Shutdown Git Data GRPC Server")
	}
	if p.gitDataConn != nil {
		p.gitDataConn.Close()
	}
	if p.gitDataUpdaterConn != nil {
		p.gitDataUpdaterConn.Close()
	}
	if p.gitDataPackCache != nil {
		p.gitDataPackCache.Close()
	}

	return fsm.Finish()
}

// gitDataRepositories is the git.RepositoryProvider which lists the repositories from the database on every call.
// The repositories which are already known are reused to keep them opened.
type gitDataRepositories struct {
	list func(ctx context.Context) ([]*database.SourceRepository, error)

	mu    sync.Mutex
	repos map[string]*git.RepositoryConfig
}

var _ git.RepositoryProvider = &gitDataRepositories{}

func newGitDataRepositories(list func(ctx context.Context) ([]*database.SourceRepository, error)) *gitDataRepositories {
	return &gitDataRepositories{list: list, repos: make(map[string]*git.RepositoryConfig)}
}

func (r *gitDataRepositories) Repositories(ctx context.Context) ([]*git.RepositoryConfig, error) {
	rows, err := r.list(ctx)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	repos := make(map[string]*git.RepositoryConfig, len(rows))
	out := make([]*git.RepositoryConfig, 0, len(rows))
	for _, v := range rows {
		if v.CloneUrl == "" || v.Name == "" {
			continue
		}
		if _, ok := repos[v.Name]; ok {
			logger.Log.Warn("Skip duplicate repository name for git-data-service", slog.String("name", v.Name))
			continue
		}
		repo, ok := r.repos[v.Name]
		if !ok || repo.URL != v.CloneUrl {
			repo = &git.RepositoryConfig{Name: v.Name, URL: v.CloneUrl, Prefix: v.Name}
		}
		repos[v.Name] = repo
		out = append(out, repo)
	}
	r.repos = repos
	return out, nil
}

// gitDataUpdater updates the repositories in the database periodically and serves Sync of the updater via gRPC.
// The process exits when it loses the lock so that the lock is acquired again by the restarted process.
type gitDataUpdater struct {
	FSM *fsm.FSM
	opt Options

	grpcServer *grpc.Server
	updater    *git.Updater
	locker     leaderelection.Locker

	Listen          string
	LockFilePath    string
	RefreshInterval time.Duration
	RefreshTimeout  time.Duration
	RefreshWorkers  int
}

func newGitDataUpdater() *gitDataUpdater {
	u := &gitDataUpdater{opt: Options{GitHubClient: githubutil.NewGitHubClientFactory("", true)}}
	u.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateGitDataUpdaterInit:     u.init,
			stateGitDataUpdaterRun:      u.run,
			stateGitDataUpdaterShutdown: u.shutdown,
		},
		stateGitDataUpdaterInit,
		stateGitDataUpdaterShutdown,
	)
	u.FSM.CloseContext = func() (context.Context, context.CancelFunc) {
		return ctxutil.WithTimeout(context.Background(), 10*time.Second)
	}

	return u
}

func (u *gitDataUpdater) Flags(fs *flag.FlagSet) {
	fs.String("dsn", "Data source name").Var(&u.opt.DSN)
	u.opt.GitHubClient.Flags(fs)
	gitDataBackendFlags(fs, &u.opt)
	fs.String("listen", "Listen addr of the gRPC server which serves Sync").Var(&u.Listen).Default(":8057")
	fs.String("lock-file-path", "The path of the lock file in the bucket (--git-data-bucket). If not set the value, don't get the lock").Var(&u.LockFilePath)
	fs.Duration("refresh-interval", "Interval for refreshing repositories").Var(&u.RefreshInterval)
	fs.Duration("refresh-timeout", "Timeout for refreshing a repository").Var(&u.RefreshTimeout).Default(1 * time.Minute)
	fs.Int("refresh-workers", "Number of workers for refreshing repositories").Var(&u.RefreshWorkers).Default(1)
}

func (u *gitDataUpdater) init(ctx context.Context) (fsm.State, error) {
	if u.RefreshInterval <= 0 {
		return fsm.Error(xerrors.Define("--refresh-interval is mandatory").WithStack())
	}
	if err := u.opt.GitHubClient.Init(); err != nil {
		return fsm.Error(err)
	}
	conn, err := openDatabase(u.opt.DSN)
	if err != nil {
		return fsm.Error(err)
	}
	d := dao.NewOptions(conn)
	backend, storageClient, err := newGitDataBackend(ctx, u.opt, nil)
	if err != nil {
		return fsm.Error(err)
	}

	repositoryProvider := newGitDataRepositories(func(ctx context.Context) ([]*database.SourceRepository, error) {
		rows, err := d.Repository.ListAll(ctx)
		return rows, xerrors.WithStack(err)
	})
	u.updater = git.NewUpdater(backend, u.opt.GitHubClient.TokenProvider, repositoryProvider, u.RefreshWorkers).
		SetInterval(u.RefreshInterval).
		SetTimeout(u.RefreshTimeout).
		SetInitTimeout(u.opt.GitDataRepositoryInitTimeout)
	if u.LockFilePath != "" {
		l, err := leaderelection.NewObjectStorage(storageClient, u.LockFilePath)
		if err != nil {
			return fsm.Error(err)
		}
		u.locker = l
	}

	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcutil.WithServerLogging()))
	git.RegisterGitDataUpdaterServer(grpcServer, git.NewUpdaterService(u.updater))
	healthSvc := health.NewServer()
	healthSvc.SetServingStatus("git-data-updater", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, healthSvc)
	u.grpcServer = grpcServer

	lis, err := net.Listen("tcp", u.Listen)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	logger.Log.Info("Start git-data updater", slog.String("addr", u.Listen))
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Log.Error("git-data updater returns error", logger.E(err))
		}
	}()

	return fsm.Next(stateGitDataUpdaterRun)
}

func (u *gitDataUpdater) run(ctx context.Context) (fsm.State, error) {
	updaterCtx := ctx
	if u.locker != nil {
		lockCtx, err := u.locker.Lock(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return fsm.Wait()
			}
			return fsm.Error(err)
		}
		updaterCtx = lockCtx
	}
	u.updater.Run(updaterCtx)
	if ctx.Err() != nil {
		return fsm.Wait()
	}
	return fsm.Error(xerrors.Define("lost the lock").WithStack())
}

func (u *gitDataUpdater) shutdown(_ context.Context) (fsm.State, error) {
	if u.grpcServer != nil {
		u.grpcServer.GracefulStop()
		logger.Log.Info("Shutdown git-data updater")
	}
	return fsm.Finish()
}

// buildServerConfig converts the runtime Options into a curated, human-meaningful
// view shown on the info page. Secrets (credentials, tokens, DSN) are intentionally
// excluded. Durations are formatted to strings; a zero duration becomes an empty
// string so the frontend can render it as disabled/unset.
func buildServerConfig(opt Options) *api.ServerConfig {
	dev := opt.Dev
	leaderElection := opt.EnableLeaderElection
	namespace := opt.Namespace
	useBazelisk := opt.UseBazelisk
	defaultBazelVersion := opt.DefaultBazelVersion
	remoteCache := opt.RemoteCache
	taskCPULimit := opt.TaskCPULimit
	taskMemoryLimit := opt.TaskMemoryLimit
	gcEnabled := opt.WithGC
	gitDataServiceListen := opt.GitDataListen
	gitDataServiceURL := opt.GitDataServiceURL
	gitDataUpdaterURL := opt.GitDataUpdaterURL
	externalReleasePollInterval := formatConfigDuration(opt.ExternalReleasePollInterval)
	eventReconcileInterval := formatConfigDuration(opt.EventReconcileInterval)
	githubAppID := opt.GitHubClient.AppID
	githubInstallationID := opt.GitHubClient.InstallationID
	githubAppSecretName := opt.GithubAppSecretName
	vaultAddr := opt.VaultAddr
	dashboardURL := opt.DashboardUrl
	bazelImage := opt.BazelImage
	sidecarImage := opt.SidecarImage
	bazelMirrorURL := opt.BazelMirrorURL
	centralRegistryMirrorURL := opt.CentralRegistryMirrorURL
	remoteAssetAPI := opt.RemoteAssetApi
	pullAlways := opt.PullAlways
	emulateJobControl := opt.EmulateJobControl
	return api.ServerConfig_builder{
		Dev:                         &dev,
		EmulateJobControl:           &emulateJobControl,
		LeaderElection:              &leaderElection,
		Namespace:                   &namespace,
		UseBazelisk:                 &useBazelisk,
		DefaultBazelVersion:         &defaultBazelVersion,
		RemoteCache:                 &remoteCache,
		TaskCpuLimit:                &taskCPULimit,
		TaskMemoryLimit:             &taskMemoryLimit,
		GcEnabled:                   &gcEnabled,
		GitDataServiceListen:        &gitDataServiceListen,
		GitDataServiceUrl:           &gitDataServiceURL,
		GitDataUpdaterUrl:           &gitDataUpdaterURL,
		ExternalReleasePollInterval: &externalReleasePollInterval,
		EventReconcileInterval:      &eventReconcileInterval,
		GithubAppId:                 &githubAppID,
		GithubInstallationId:        &githubInstallationID,
		GithubAppSecretName:         &githubAppSecretName,
		VaultAddr:                   &vaultAddr,
		DashboardUrl:                &dashboardURL,
		BazelImage:                  &bazelImage,
		SidecarImage:                &sidecarImage,
		BazelMirrorUrl:              &bazelMirrorURL,
		CentralRegistryMirrorUrl:    &centralRegistryMirrorURL,
		RemoteAssetApi:              &remoteAssetAPI,
		PullAlways:                  &pullAlways,
	}.Build()
}

func formatConfigDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	return d.String()
}

func builder(ctx context.Context, opt Options) error {
	p := newProcess(opt)

	if err := p.FSM.LoopContext(ctx); err != nil {
		return err
	}

	return nil
}

func AddCommand(rootCmd *cli.Command) {
	opt := Options{GitHubClient: githubutil.NewGitHubClientFactory("", true)}

	cmd := &cli.Command{
		Use: "builder",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			if err := loadGitHubAppFromEnv(opt.GitHubClient); err != nil {
				return err
			}
			return builder(ctx, opt)
		},
	}

	fs := cmd.Flags()
	fs.String("dsn", "Data source name").Var(&opt.DSN)
	fs.String("id", "the holder identity name").Var(&opt.Id).Default(uuid.New().String())
	fs.Bool("enable-leader-election", "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.").Var(&opt.EnableLeaderElection)
	fs.String("lease-lock-name", "the lease lock resource name").Var(&opt.LeaseLockName)
	fs.String("lease-lock-namespace", "the lease lock resource namespace").Var(&opt.LeaseLockNamespace)
	fs.String("namespace", "The namespace which will be created the job").Var(&opt.Namespace)
	fs.String("github-app-secret-name", "The name of Secret which contains github app id, installation id and private key.").Var(&opt.GithubAppSecretName)
	opt.GitHubClient.Flags(fs)
	fs.String("addr", "Listen addr which will be served API").Var(&opt.Addr).Default("127.0.0.1:8081")
	fs.String("dashboard", "URL of dashboard").Var(&opt.DashboardUrl).Default("http://localhost")
	fs.String("builder-api", "URL of the api of builder").Var(&opt.BuilderApiUrl).Default("http://localhost")
	fs.Bool("dev", "development mode").Var(&opt.Dev)
	fs.String("kubeconfig", "Path to a kubeconfig file. If empty, the in-cluster config is used. Set it to the local kwok cluster in development.").Var(&opt.Kubeconfig)
	fs.Bool("emulate-job-control", "Enable the dev-only CompleteTask RPC against a kwok-emulated kube-apiserver (local development)").Var(&opt.EmulateJobControl)
	fs.String("emulate-job-fixture-dir", "Directory of fixture logs served as pod logs by kwok. Defaults to go/build/dev/jobresult under the bazel workspace.").Var(&opt.EmulateJobFixtureDir)
	fs.String("minio-endpoint", "The endpoint of MinIO. If this value is empty, then we find the endpoint from kube-apiserver using incluster config.").Var(&opt.MinIOEndpoint)
	fs.String("minio-name", "The name of MinIO").Var(&opt.MinIOName)
	fs.String("minio-namespace", "The namespace of MinIO").Var(&opt.MinIONamespace)
	fs.Int("minio-port", "Port number of MinIO").Var(&opt.MinIOPort).Default(8080)
	fs.String("minio-bucket", "The bucket name that will be used a log storage").Var(&opt.MinIOBucket).Default("logs")
	fs.String("minio-access-key", "The access key").Var(&opt.MinIOAccessKey)
	fs.String("minio-secret-access-key", "The secret access key").Var(&opt.MinIOSecretAccessKey)
	fs.String("minio-secret-access-key-file", "The file path that contains secret access key").Var(&opt.MinIOSecretAccessKeyFile)
	fs.String("service-account-token-file", "A file path that contains JWT token").Var(&opt.ServiceAccountTokenFile).Default("/var/run/secrets/kubernetes.io/serviceaccount/token")
	fs.String("vault-addr", "The vault URL").Var(&opt.VaultAddr)
	fs.String("vault-token-file", "The token for Vault").Var(&opt.VaultTokenFile)
	fs.String("vault-k8s-auth-path", "The mount path of kubernetes auth method").Var(&opt.VaultK8sAuthPath).Default("auth/kubernetes")
	fs.String("vault-k8s-auth-role", "Role name for k8s auth method").Var(&opt.VaultK8sAuthRole)
	fs.String("remote-cache", "The url of remote cache of bazel.").Var(&opt.RemoteCache)
	fs.Bool("remote-asset", "Enable Remote Asset API. This is experimental feature.").Var(&opt.RemoteAssetApi)
	fs.String("bazel-image", "Bazel container image").Var(&opt.BazelImage).Default("ghcr.io/f110/bazel-container")
	fs.Bool("use-bazelisk", "Use bazelisk").Var(&opt.UseBazelisk)
	fs.String("default-bazel-version", "Default bazel version").Var(&opt.DefaultBazelVersion).Default("3.2.0")
	fs.String("bazel-mirror-url", "The URL of bazel").Var(&opt.BazelMirrorURL)
	fs.String("bazel-mirror-endpoint", "The endpoint of MinIO for bazel mirror. If this value is empty, then we find the endpoint from kube-apiserver using incluster config.").Var(&opt.BazelMirrorEndpoint)
	fs.String("bazel-mirror-name", "The name of MinIO for bazel mirror").Var(&opt.BazelMirrorName)
	fs.String("bazel-mirror-namespace", "The namespace of MinIO for bazel mirror").Var(&opt.BazelMirrorNamespace)
	fs.Int("bazel-mirror-port", "Port number of MinIO for bazel mirror").Var(&opt.BazelMirrorPort).Default(8080)
	fs.String("bazel-mirror-bucket", "The bucket name that contains bazel's binaries").Var(&opt.BazelMirrorBucket)
	fs.String("bazel-mirror-prefix", "The prefix of bazel's artifacts").Var(&opt.BazelMirrorPrefix)
	fs.String("bazel-mirror-access-key", "The access key for bazel mirror").Var(&opt.BazelMirrorAccessKey)
	fs.String("bazel-mirror-secret-access-key", "The secret access key for bazel mirror").Var(&opt.BazelMirrorSecretAccessKey)
	fs.String("bazel-mirror-secret-access-key-file", "The file path that contains secret access key").Var(&opt.BazelMirrorSecretAccessKeyFile)
	fs.String("central-registry-mirror-url", "The URL of Bazel Central Registry mirror").Var(&opt.CentralRegistryMirrorURL)
	fs.Bool("artifact-upload", "Upload collected build artifacts to the OCI registry named in the repository of cd_artifact").Var(&opt.ArtifactUpload)
	fs.Bool("artifact-registry-insecure", "Allow accessing the artifact registries over plain HTTP").Var(&opt.ArtifactRegistryInsecure)
	fs.String("artifact-builder-addr", "gRPC address of the builder API the collect sidecar uploads to. When empty, artifact collection is disabled").Var(&opt.ArtifactBuilderAddr)
	fs.String("nats-url", "The URL for nats-server to publish the events of the uploaded artifacts. If empty, no event is published").Var(&opt.NATSURL)
	fs.String("nats-stream-name", "The name of stream for JetStream").Var(&opt.NATSStreamName).Default("build")
	fs.String("nats-subject", "The subject of stream").Var(&opt.NATSSubject).Default("artifact.published")
	fs.String("sidecar-image", "Sidecar container image").Var(&opt.SidecarImage).Default("registry.f110.dev/build/sidecar")
	fs.String("ctl-image", "CLI container image").Var(&opt.CLIImage).Default("registry.f110.dev/build/buildctl")
	fs.String("public-key-url", "URL of the public key of heimdallr to verify the token of the requests to /mcp. It is /internal/publickey of the internal API. If not specified, MCP is not served").Var(&opt.PublicKeyURL)
	fs.Bool("pull-always", "Pull always").Var(&opt.PullAlways)
	fs.String("task-cpu-limit", "Task cpu limit. If the job set the limit, It will used the job defined value.").Var(&opt.TaskCPULimit).Default("1000m")
	fs.String("task-memory-limit", "Task memory limit. If the job set the limit, It will used the job defined value.").Var(&opt.TaskMemoryLimit).Default("4096Mi")
	fs.Bool("with-gc", "Enable GC for the job").Var(&opt.WithGC)
	fs.StringArray("exclude-nodes", "THe list of node to not assigned job").Var(&opt.ExcludeNodes)
	fs.Duration("external-release-poll-interval", "Interval between polls of third-party repositories for external_release triggers").Var(&opt.ExternalReleasePollInterval).Default(1 * time.Hour)
	fs.Duration("event-reconcile-interval", "Interval between scans of the github_event table for PENDING/FAILED rows").Var(&opt.EventReconcileInterval).Default(30 * time.Second)
	fs.Duration("task-schedule-interval", "Interval between scans of the pending tasks to start them").Var(&opt.TaskScheduleInterval).Default(30 * time.Second)
	fs.Duration("event-max-processing-duration", "Time after `created_at` at which an unfinished github_event row is moved to EXPIRED").Var(&opt.EventMaxProcessingDuration).Default(30 * time.Minute)
	fs.String("git-data-service-url", "URL of the git-data-service gRPC endpoint used by reconcilers to read repository data. If empty, reconcilers read from GitHub instead.").Var(&opt.GitDataServiceURL)
	fs.String("git-data-updater-url", "URL of the gRPC endpoint of the git-data updater (build git-data-updater) which syncs the pushed repository before building it. If empty, the pushed repository is not synced.").Var(&opt.GitDataUpdaterURL)
	fs.Bool("clone-from-git-data-service", "Fetch the source tree from git-data-service (--git-data-service-url) in the pre-process container instead of cloning from GitHub. If not set, the source is always cloned from GitHub.").Var(&opt.CloneFromGitDataService)
	fs.String("git-data-listen", "Listen addr of the embedded git-data-service. If empty, the service is disabled.").Var(&opt.GitDataListen)
	gitDataBackendFlags(fs, &opt)
	fs.Int64("git-data-packfile-cache-size", "Maximum total bytes of packfiles cached in memory by git-data-service. "+
		"If set zero, the in-memory packfile cache is disabled.").Var(&opt.GitDataPackfileCacheSize).Default(256 * 1024 * 1024)
	fs.Duration("git-data-packfile-cache-ttl", "Lifetime of an in-memory cached packfile in git-data-service").Var(&opt.GitDataPackfileCacheTTL).Default(1 * time.Hour)
	fs.Duration("git-data-packfile-cache-sweep-interval", "Interval for reclaiming expired in-memory packfiles in git-data-service").Var(&opt.GitDataPackfileCacheSweepInterval).Default(6 * time.Hour)
	fs.Bool("debug", "Enable debugging mode").Var(&opt.Debug)

	rootCmd.AddCommand(cmd)
}

func AddGitDataUpdaterCommand(rootCmd *cli.Command) {
	u := newGitDataUpdater()
	cmd := &cli.Command{
		Use:   "git-data-updater",
		Short: "Update the repositories of git-data-service which are registered to the database",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			if err := loadGitHubAppFromEnv(u.opt.GitHubClient); err != nil {
				return err
			}
			return u.FSM.LoopContext(ctx)
		},
	}
	u.Flags(cmd.Flags())

	rootCmd.AddCommand(cmd)
}

// openDatabase opens the connection of the database with the parameters which dao requires.
func openDatabase(dsn string) (*sql.DB, error) {
	parsedDSN, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	parsedDSN.ParseTime = true
	loc, err := time.LoadLocation("Local")
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	parsedDSN.Loc = loc
	dsn = parsedDSN.FormatDSN()

	logger.Log.Debug("Open sql connection", slog.String("dsn", dsn))
	conn, err := sql.Open("querylog", dsn)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return conn, nil
}

// newGitDataBackend creates the backend of git-data-service from the --git-data-* options. It also returns the client of the object storage.
func newGitDataBackend(ctx context.Context, opt Options, packCache *git.PackfileCache) (*git.ObjectStorageBackend, *storage.S3, error) {
	if opt.GitDataStorageEndpoint == "" || opt.GitDataBucket == "" {
		return nil, nil, xerrors.Define("--git-data-storage-endpoint and --git-data-bucket are required").WithStack()
	}

	secretAccessKey := opt.GitDataStorageSecretAccessKey
	if opt.GitDataStorageSecretAccessKeyFile != "" {
		b, err := os.ReadFile(opt.GitDataStorageSecretAccessKeyFile)
		if err != nil {
			return nil, nil, xerrors.WithStack(err)
		}
		secretAccessKey = strings.TrimSpace(string(b))
	}
	storageOpt := storage.NewS3OptionToExternal(opt.GitDataStorageEndpoint, opt.GitDataStorageRegion, opt.GitDataStorageAccessKey, secretAccessKey)
	storageOpt.PathStyle = true
	storageOpt.CACertFile = opt.GitDataStorageCAFile
	storageClient := storage.NewS3(opt.GitDataBucket, storageOpt)

	var cachePool *client.SinglePool
	if opt.GitDataMemcachedEndpoint != "" {
		if err := netutil.WaitListen(opt.GitDataMemcachedEndpoint, 5*time.Minute); err != nil {
			return nil, nil, xerrors.WithStack(err)
		}
		cacheServer, err := client.NewServerWithMetaProtocol(ctx, "cache-1", "tcp", opt.GitDataMemcachedEndpoint)
		if err != nil {
			return nil, nil, xerrors.WithStack(err)
		}
		cachePool, err = client.NewSinglePool(cacheServer)
		if err != nil {
			return nil, nil, xerrors.WithStack(err)
		}
	}

	return &git.ObjectStorageBackend{
		Client:                 storageClient,
		CachePool:              cachePool,
		PackCache:              packCache,
		DisableInflatePackFile: opt.GitDataDisableInflatePackFile,
	}, storageClient, nil
}

func gitDataBackendFlags(fs *flag.FlagSet, opt *Options) {
	fs.String("git-data-storage-endpoint", "The endpoint of the object storage for git-data-service").Var(&opt.GitDataStorageEndpoint)
	fs.String("git-data-storage-region", "The region name of the object storage for git-data-service").Var(&opt.GitDataStorageRegion)
	fs.String("git-data-storage-access-key", "The access key for the git-data-service object storage").Var(&opt.GitDataStorageAccessKey)
	fs.String("git-data-storage-secret-access-key", "The secret access key for the git-data-service object storage").Var(&opt.GitDataStorageSecretAccessKey)
	fs.String("git-data-storage-secret-access-key-file", "The file path that contains the secret access key for the git-data-service object storage").Var(&opt.GitDataStorageSecretAccessKeyFile)
	fs.String("git-data-storage-ca-file", "CA certificate file path for the git-data-service object storage").Var(&opt.GitDataStorageCAFile)
	fs.String("git-data-bucket", "The bucket name used by git-data-service").Var(&opt.GitDataBucket)
	fs.String("git-data-memcached-endpoint", "The endpoint of memcached used by git-data-service").Var(&opt.GitDataMemcachedEndpoint)
	fs.Bool("git-data-disable-inflate-packfile", "Disable inflating packfile in git-data-service").Var(&opt.GitDataDisableInflatePackFile)
	fs.Duration("git-data-repository-init-timeout", "Timeout for initializing a repository in git-data-service").Var(&opt.GitDataRepositoryInitTimeout).Default(5 * time.Minute)
}

// loadGitHubAppFromEnv overrides the credential of GitHub App by the environment variables.
func loadGitHubAppFromEnv(f *githubutil.GitHubClientFactory) error {
	if v := os.Getenv("GITHUB_APP_ID"); v != "" {
		appId, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return xerrors.WithStack(err)
		}
		f.AppID = appId
	}
	if v := os.Getenv("GITHUB_INSTALLATION_ID"); v != "" {
		installationId, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return xerrors.WithStack(err)
		}
		f.InstallationID = installationId
	}
	if v := os.Getenv("GITHUB_PRIVATEKEY_FILE"); v != "" {
		f.PrivateKeyFile = v
	}
	return nil
}
