package builder

import (
	"context"
	"database/sql"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
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
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	secretstoreclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"

	"go.f110.dev/mono/go/build/api"
	"go.f110.dev/mono/go/build/coordinator"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/gc"
	"go.f110.dev/mono/go/build/releasewatcher"
	"go.f110.dev/mono/go/build/watcher"
	"go.f110.dev/mono/go/build/webhook"
	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/collections/set"
	"go.f110.dev/mono/go/ctxutil"
	_ "go.f110.dev/mono/go/database/querylog"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/githubutil"
	"go.f110.dev/mono/go/logger/slogger"
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
	SidecarImage                   string
	CLIImage                       string
	PullAlways                     bool
	TaskCPULimit                   string
	TaskMemoryLimit                string
	WithGC                         bool
	ExcludeNodes                   []string
	ExternalReleasePollInterval    time.Duration
	EventReconcileInterval         time.Duration
	EventMaxProcessingDuration     time.Duration

	GitDataServiceURL                 string
	GitDataListen                     string
	GitDataStorageEndpoint            string
	GitDataStorageRegion              string
	GitDataStorageAccessKey           string
	GitDataStorageSecretAccessKey     string
	GitDataStorageSecretAccessKeyFile string
	GitDataStorageCAFile              string
	GitDataBucket                     string
	GitDataMemcachedEndpoint          string
	GitDataExternalRepositories       []string
	GitDataLockFilePath               string
	GitDataRefreshInterval            time.Duration
	GitDataRefreshTimeout             time.Duration
	GitDataRefreshWorkers             int
	GitDataDisableInflatePackFile     bool
	GitDataRepositoryInitTimeout      time.Duration

	Dev   bool
	Debug bool
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

	bazelBuilder      *coordinator.BazelBuilder
	apiServer         *api.Api
	notifier          *webhook.Notifier
	reconcilers       webhook.Reconcilers
	gitDataGRPCServer *grpc.Server
	gitDataUpdater    *git.Updater
	gitDataConn       *grpc.ClientConn
	gitDataClient     git.GitDataClient
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
		slogger.Log.Info("Start without kube-apiserver. All of integrations with kube-apiserver will be disabled.")
	} else {
		cfg, err := clientcmd.BuildConfigFromFlags("", "")
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

	parsedDSN, err := mysql.ParseDSN(p.opt.DSN)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	parsedDSN.ParseTime = true
	loc, err := time.LoadLocation("Local")
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	parsedDSN.Loc = loc
	p.opt.DSN = parsedDSN.FormatDSN()

	slogger.Log.Debug("Open sql connection", slog.String("dsn", p.opt.DSN))
	conn, err := sql.Open("querylog", p.opt.DSN)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
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
				slogger.Log.Debug("Can not log in", slogger.Verbose(err))
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
	slogger.Log.Debug("Check migration")
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
		storageOpt.PathStyle = true
	}
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
		bazelMirrorStorageOpt.PathStyle = true
	}
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
		slogger.Log.Error("Failed create BazelBuilder", slogger.E(err))
		return fsm.Error(xerrors.WithStack(err))
	}
	p.bazelBuilder = c

	if p.opt.GitDataListen == "" {
		return fsm.Next(stateStartApiServer)
	}
	return fsm.Next(stateStartGitDataService)
}

func (p *process) startGitDataService(ctx context.Context) (fsm.State, error) {
	if p.opt.GitDataStorageEndpoint == "" || p.opt.GitDataBucket == "" {
		return fsm.Error(xerrors.Define("--git-data-storage-endpoint and --git-data-bucket are required when --git-data-listen is set").WithStack())
	}

	secretAccessKey := p.opt.GitDataStorageSecretAccessKey
	if p.opt.GitDataStorageSecretAccessKeyFile != "" {
		b, err := os.ReadFile(p.opt.GitDataStorageSecretAccessKeyFile)
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		secretAccessKey = strings.TrimSpace(string(b))
	}
	storageOpt := storage.NewS3OptionToExternal(p.opt.GitDataStorageEndpoint, p.opt.GitDataStorageRegion, p.opt.GitDataStorageAccessKey, secretAccessKey)
	storageOpt.PathStyle = true
	storageOpt.CACertFile = p.opt.GitDataStorageCAFile
	storageClient := storage.NewS3(p.opt.GitDataBucket, storageOpt)

	var cachePool *client.SinglePool
	if p.opt.GitDataMemcachedEndpoint != "" {
		if err := netutil.WaitListen(p.opt.GitDataMemcachedEndpoint, 5*time.Minute); err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		cacheServer, err := client.NewServerWithMetaProtocol(ctx, "cache-1", "tcp", p.opt.GitDataMemcachedEndpoint)
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		cachePool, err = client.NewSinglePool(cacheServer)
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
	}

	repositories, err := p.collectGitDataRepositories(ctx)
	if err != nil {
		return fsm.Error(err)
	}

	tokenProvider := p.opt.GitHubClient.TokenProvider

	repos := make(map[string]*git.RepositoryConfig)
	for _, v := range repositories {
		if err := v.Open(ctx, storageClient, cachePool, tokenProvider, p.opt.GitDataRepositoryInitTimeout, p.opt.GitDataDisableInflatePackFile); err != nil {
			return fsm.Error(err)
		}
		repos[v.Name] = v
	}

	service, err := git.NewDataService(repos)
	if err != nil {
		return fsm.Error(err)
	}
	grpcServer := grpc.NewServer()
	git.RegisterGitDataServer(grpcServer, service)
	healthSvc := health.NewServer()
	healthSvc.SetServingStatus("git-data", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, healthSvc)
	p.gitDataGRPCServer = grpcServer

	lis, err := net.Listen("tcp", p.opt.GitDataListen)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	slogger.Log.Info("Start git-data-service", slog.String("addr", p.opt.GitDataListen))
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			slogger.Log.Error("git-data-service returns error", slogger.E(err))
		}
	}()

	updater, err := git.NewUpdater(storageClient, tokenProvider, repositories, p.opt.GitDataLockFilePath, p.opt.GitDataRefreshWorkers)
	if err != nil {
		return fsm.Error(err)
	}
	updater.SetCachePool(cachePool).
		SetInitTimeout(p.opt.GitDataRepositoryInitTimeout).
		SetDisableInflatePackFile(p.opt.GitDataDisableInflatePackFile).
		SetDataService(service)
	if p.opt.GitDataRefreshInterval > 0 {
		updater.SetInterval(p.opt.GitDataRefreshInterval).
			SetTimeout(p.opt.GitDataRefreshTimeout)
	}
	p.gitDataUpdater = updater

	return fsm.Next(stateStartApiServer)
}

func (p *process) collectGitDataRepositories(ctx context.Context) ([]*git.RepositoryConfig, error) {
	repos, err := p.dao.Repository.ListAll(ctx)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	seen := set.New[string]()
	var out []*git.RepositoryConfig
	for _, r := range repos {
		if r.Status != database.SourceRepositoryStatusReady {
			continue
		}
		if r.CloneUrl == "" || r.Name == "" {
			continue
		}
		if seen.Has(r.Name) {
			slogger.Log.Warn("Skip duplicate repository name for git-data-service", slog.String("name", r.Name))
			continue
		}
		seen.Add(r.Name)
		out = append(out, &git.RepositoryConfig{Name: r.Name, URL: r.CloneUrl, Prefix: r.Name})
	}

	for _, v := range p.opt.GitDataExternalRepositories {
		if strings.Index(v, "|") == -1 {
			return nil, xerrors.Definef("--git-data-external-repository=%s is invalid", v).WithStack()
		}
		parts := strings.Split(v, "|")
		if len(parts) != 3 {
			return nil, xerrors.Definef("--git-data-external-repository=%s must be 'name|url|prefix'", v).WithStack()
		}
		name := parts[0]
		if seen.Has(name) {
			slogger.Log.Warn("Skip duplicate repository name for git-data-service", slog.String("name", name))
			continue
		}
		seen.Add(name)
		out = append(out, &git.RepositoryConfig{Name: name, URL: parts[1], Prefix: parts[2]})
	}

	return out, nil
}

func (p *process) startApiServer(_ context.Context) (fsm.State, error) {
	if p.opt.GitDataServiceURL != "" {
		conn, err := grpc.NewClient(p.opt.GitDataServiceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		p.gitDataConn = conn
		p.gitDataClient = git.NewGitDataClient(conn)
		slogger.Log.Info("Use git-data-service for reading repository data", slog.String("addr", p.opt.GitDataServiceURL))
	}

	p.notifier = webhook.NewNotifier()
	p.reconcilers = webhook.Reconcilers{}
	p.reconcilers.Register(webhook.NewPushReconciler(p.dao, p.ghClient, p.bazelBuilder, p.gitDataUpdater, p.gitDataClient))
	p.reconcilers.Register(webhook.NewPullRequestReconciler(p.dao, p.ghClient, p.bazelBuilder, p.gitDataClient))
	p.reconcilers.Register(webhook.NewReleaseReconciler(p.dao, p.ghClient, p.bazelBuilder, p.gitDataClient))
	p.reconcilers.Register(webhook.NewIssueCommentReconciler(p.dao, p.ghClient, p.bazelBuilder, p.gitDataClient))

	addRepo := make(chan *git.RepositoryConfig, 1)
	go func() {
		for {
			repo := <-addRepo
			p.gitDataUpdater.AddRepo(context.Background(), repo)
		}
	}()
	apiServer, err := api.NewApi(p.opt.Addr, p.bazelBuilder, p.dao, p.ghClient, p.gitDataClient, storage.NewS3(p.opt.BazelMirrorBucket, p.bazelMirrorStorageOpt), p.opt.BazelMirrorPrefix, p.notifier, addRepo, buildServerConfig(p.opt))
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	p.apiServer = apiServer

	go func() {
		slogger.Log.Info("Start API Server", slog.String("addr", p.apiServer.Addr))
		p.apiServer.ListenAndServe()
	}()

	return stateLeaderElection, nil
}

// leaderElection will get the lock.
// Next state: stateStartWorker
func (p *process) leaderElection(_ context.Context) (fsm.State, error) {
	if p.k8sClient == nil || p.opt.LeaseLockName == "" || p.opt.LeaseLockNamespace == "" {
		slogger.Log.Info("Skip leader election")
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
	e, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   30 * time.Second,
		RenewDeadline:   15 * time.Second,
		RetryPeriod:     5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
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
			slogger.Log.Info("Start JobWatcher")
			if err := jobWatcher.Run(p.ctx, 1); err != nil {
				slogger.Log.Error("Error occurred at JobWatcher", slogger.E(err))
				return
			}
		}()
	}

	if p.opt.WithGC {
		g := gc.NewGC(1*time.Hour, p.dao, p.opt.MinIOBucket, p.storageOpt)
		go func() {
			slogger.Log.Info("Start GC")
			g.Start()
		}()
	}

	interval := p.opt.ExternalReleasePollInterval
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	manager := releasewatcher.NewManager(p.bazelBuilder, p.dao, p.ghClient, nil, interval)
	go manager.Start(p.ctx)

	scheduler := webhook.NewScheduler(p.dao, p.reconcilers, p.notifier, p.opt.EventReconcileInterval, p.opt.EventMaxProcessingDuration)
	go scheduler.Run(p.ctx)

	if p.gitDataUpdater != nil && p.opt.GitDataRefreshInterval > 0 {
		go p.gitDataUpdater.Run(p.ctx)
	}

	return fsm.Wait()
}

func (p *process) shutdown(ctx context.Context) (fsm.State, error) {
	slogger.Log.Info("Shutting down")
	if p.apiServer != nil {
		p.apiServer.Shutdown(ctx)
		slogger.Log.Info("Shutdown API Server")
	}
	if p.gitDataGRPCServer != nil {
		p.gitDataGRPCServer.GracefulStop()
		slogger.Log.Info("Shutdown Git Data GRPC Server")
	}
	if p.gitDataConn != nil {
		p.gitDataConn.Close()
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
	gitDataRefreshInterval := formatConfigDuration(opt.GitDataRefreshInterval)
	gitDataRefreshWorkers := int32(opt.GitDataRefreshWorkers)
	externalReleasePollInterval := formatConfigDuration(opt.ExternalReleasePollInterval)
	eventReconcileInterval := formatConfigDuration(opt.EventReconcileInterval)
	githubAppID := opt.GitHubClient.AppID
	vaultAddr := opt.VaultAddr
	dashboardURL := opt.DashboardUrl
	return api.ServerConfig_builder{
		Dev:                         &dev,
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
		GitDataRefreshInterval:      &gitDataRefreshInterval,
		GitDataRefreshWorkers:       &gitDataRefreshWorkers,
		ExternalReleasePollInterval: &externalReleasePollInterval,
		EventReconcileInterval:      &eventReconcileInterval,
		GithubAppId:                 &githubAppID,
		VaultAddr:                   &vaultAddr,
		DashboardUrl:                &dashboardURL,
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
			if v := os.Getenv("GITHUB_APP_ID"); v != "" {
				appId, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return err
				}
				opt.GitHubClient.AppID = appId
			}
			if v := os.Getenv("GITHUB_INSTALLATION_ID"); v != "" {
				installationId, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return err
				}
				opt.GitHubClient.InstallationID = installationId
			}
			if v := os.Getenv("GITHUB_PRIVATEKEY_FILE"); v != "" {
				opt.GitHubClient.PrivateKeyFile = v
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
	fs.String("sidecar-image", "Sidecar container image").Var(&opt.SidecarImage).Default("registry.f110.dev/build/sidecar")
	fs.String("ctl-image", "CLI container image").Var(&opt.CLIImage).Default("registry.f110.dev/build/buildctl")
	fs.Bool("pull-always", "Pull always").Var(&opt.PullAlways)
	fs.String("task-cpu-limit", "Task cpu limit. If the job set the limit, It will used the job defined value.").Var(&opt.TaskCPULimit).Default("1000m")
	fs.String("task-memory-limit", "Task memory limit. If the job set the limit, It will used the job defined value.").Var(&opt.TaskMemoryLimit).Default("4096Mi")
	fs.Bool("with-gc", "Enable GC for the job").Var(&opt.WithGC)
	fs.StringArray("exclude-nodes", "THe list of node to not assigned job").Var(&opt.ExcludeNodes)
	fs.Duration("external-release-poll-interval", "Interval between polls of third-party repositories for external_release triggers").Var(&opt.ExternalReleasePollInterval).Default(1 * time.Hour)
	fs.Duration("event-reconcile-interval", "Interval between scans of the github_event table for PENDING/FAILED rows").Var(&opt.EventReconcileInterval).Default(30 * time.Second)
	fs.Duration("event-max-processing-duration", "Time after `created_at` at which an unfinished github_event row is moved to EXPIRED").Var(&opt.EventMaxProcessingDuration).Default(30 * time.Minute)
	fs.String("git-data-service-url", "URL of the git-data-service gRPC endpoint used by reconcilers to read repository data. If empty, reconcilers read from GitHub instead.").Var(&opt.GitDataServiceURL)
	fs.String("git-data-listen", "Listen addr of the embedded git-data-service. If empty, the service is disabled.").Var(&opt.GitDataListen)
	fs.String("git-data-storage-endpoint", "The endpoint of the object storage for git-data-service").Var(&opt.GitDataStorageEndpoint)
	fs.String("git-data-storage-region", "The region name of the object storage for git-data-service").Var(&opt.GitDataStorageRegion)
	fs.String("git-data-storage-access-key", "The access key for the git-data-service object storage").Var(&opt.GitDataStorageAccessKey)
	fs.String("git-data-storage-secret-access-key", "The secret access key for the git-data-service object storage").Var(&opt.GitDataStorageSecretAccessKey)
	fs.String("git-data-storage-secret-access-key-file", "The file path that contains the secret access key for the git-data-service object storage").Var(&opt.GitDataStorageSecretAccessKeyFile)
	fs.String("git-data-storage-ca-file", "CA certificate file path for the git-data-service object storage").Var(&opt.GitDataStorageCAFile)
	fs.String("git-data-bucket", "The bucket name used by git-data-service").Var(&opt.GitDataBucket)
	fs.String("git-data-memcached-endpoint", "The endpoint of memcached used by git-data-service").Var(&opt.GitDataMemcachedEndpoint)
	fs.StringArray("git-data-external-repository", "External repository to be synced by the embedded git-data-service. Format: name|url|prefix (e.g. go|https://github.com/golang/go.git|golang/go)").Var(&opt.GitDataExternalRepositories)
	fs.String("git-data-lock-file-path", "The path of the git-data-service updater lock file").Var(&opt.GitDataLockFilePath)
	fs.Duration("git-data-refresh-interval", "Interval for refreshing repositories in git-data-service. Zero disables periodic refresh.").Var(&opt.GitDataRefreshInterval)
	fs.Duration("git-data-refresh-timeout", "Timeout for refreshing a repository in git-data-service").Var(&opt.GitDataRefreshTimeout).Default(1 * time.Minute)
	fs.Int("git-data-refresh-workers", "Number of workers for refreshing repositories in git-data-service").Var(&opt.GitDataRefreshWorkers).Default(1)
	fs.Bool("git-data-disable-inflate-packfile", "Disable inflating packfile in git-data-service").Var(&opt.GitDataDisableInflatePackFile)
	fs.Duration("git-data-repository-init-timeout", "Timeout for initializing a repository in git-data-service").Var(&opt.GitDataRepositoryInitTimeout).Default(5 * time.Minute)
	fs.Bool("debug", "Enable debugging mode").Var(&opt.Debug)

	rootCmd.AddCommand(cmd)
}
