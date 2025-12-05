package builder

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/go-github/v73/github"
	"github.com/google/uuid"
	"go.f110.dev/protoc-ddl/probe"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
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
	"go.f110.dev/mono/go/build/watcher"
	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/ctxutil"
	_ "go.f110.dev/mono/go/database/querylog"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/githubutil"
	"go.f110.dev/mono/go/logger"
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
	GithubAppId              int64
	GithubInstallationId     int64
	GithubPrivateKeyFile     string
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
	SidecarImage                   string
	CLIImage                       string
	PullAlways                     bool
	TaskCPULimit                   string
	TaskMemoryLimit                string
	WithGC                         bool
	ExcludeNodes                   []string

	Dev   bool
	Debug bool
}

const (
	stateInit fsm.State = iota
	stateCheckMigrate
	stateSetup
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

	ghClient                  *github.Client
	kubeClient                *kubernetes.Clientset
	secretStoreClient         *secretstoreclient.Clientset
	restCfg                   *rest.Config
	coreSharedInformerFactory kubeinformers.SharedInformerFactory
	dao                       dao.Options
	minioOpt                  storage.MinIOOptions
	bazelMirrorMinIOOpt       storage.MinIOOptions
	vaultClient               *vault.Client

	bazelBuilder *coordinator.BazelBuilder
	apiServer    *api.Api
}

func newProcess(opt Options) *process {
	ctx, cancel := ctxutil.WithCancel(context.Background())
	p := &process{ctx: ctx, cancel: cancel, opt: opt}
	p.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:           p.init,
			stateCheckMigrate:   p.checkMigrate,
			stateSetup:          p.setup,
			stateStartApiServer: p.startApiServer,
			stateLeaderElection: p.leaderElection,
			stateStartWorker:    p.startWorker,
			stateShutdown:       p.shutdown,
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
	app, err := githubutil.NewApp(p.opt.GithubAppId, p.opt.GithubInstallationId, p.opt.GithubPrivateKeyFile)
	if err != nil {
		return fsm.Error(err)
	}
	p.ghClient = github.NewClient(&http.Client{Transport: githubutil.NewTransportWithApp(http.DefaultTransport, app)})

	if p.opt.Dev {
		logger.Log.Info("Start without kube-apiserver. All of integrations with kube-apiserver will be disabled.")
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
		p.kubeClient = kubeClient
		p.coreSharedInformerFactory = kubeinformers.NewSharedInformerFactoryWithOptions(
			kubeClient,
			30*time.Second,
			kubeinformers.WithNamespace(p.opt.Namespace),
		)

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

	logger.Log.Debug("Open sql connection", zap.String("dsn", p.opt.DSN))
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
	var minioOpt storage.MinIOOptions
	if p.opt.MinIOEndpoint != "" {
		minioOpt = storage.NewMinIOOptionsViaEndpoint(p.opt.MinIOEndpoint, "", p.opt.MinIOAccessKey, p.opt.MinIOSecretAccessKey)
	} else {
		minioOpt = storage.NewMinIOOptionsViaService(
			p.kubeClient,
			p.restCfg,
			p.opt.MinIOName,
			p.opt.MinIONamespace,
			p.opt.MinIOPort,
			p.opt.MinIOAccessKey,
			p.opt.MinIOSecretAccessKey,
			p.opt.Dev,
		)
	}
	p.minioOpt = minioOpt

	var bazelMirrorMinIOOpt storage.MinIOOptions
	if p.opt.BazelMirrorEndpoint != "" {
		bazelMirrorMinIOOpt = storage.NewMinIOOptionsViaEndpoint(p.opt.BazelMirrorEndpoint, "", p.opt.BazelMirrorAccessKey, p.opt.BazelMirrorSecretAccessKey)
	} else {
		bazelMirrorMinIOOpt = storage.NewMinIOOptionsViaService(
			p.kubeClient,
			p.restCfg,
			p.opt.BazelMirrorName,
			p.opt.BazelMirrorNamespace,
			p.opt.BazelMirrorPort,
			p.opt.BazelMirrorAccessKey,
			p.opt.BazelMirrorSecretAccessKey,
			p.opt.Dev,
		)
	}
	p.bazelMirrorMinIOOpt = bazelMirrorMinIOOpt

	var kubernetesOpt coordinator.KubernetesOptions
	if p.coreSharedInformerFactory != nil && p.kubeClient != nil {
		kubernetesOpt = coordinator.NewKubernetesOptions(
			p.coreSharedInformerFactory.Batch().V1().Jobs(),
			p.coreSharedInformerFactory.Core().V1().Pods(),
			p.kubeClient,
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
		p.opt.PullAlways,
		p.opt.GithubAppId,
		p.opt.GithubInstallationId,
		p.opt.GithubAppSecretName,
	)
	c, err := coordinator.NewBazelBuilder(
		p.opt.DashboardUrl,
		kubernetesOpt,
		p.dao,
		p.opt.Namespace,
		p.ghClient,
		p.opt.MinIOBucket,
		minioOpt,
		bazelOpt,
		p.vaultClient,
		p.opt.ExcludeNodes,
		p.opt.Dev,
	)
	if err != nil {
		logger.Log.Error("Failed create BazelBuilder", zap.Error(err))
		return fsm.Error(xerrors.WithStack(err))
	}
	p.bazelBuilder = c

	return fsm.Next(stateStartApiServer)
}

func (p *process) startApiServer(_ context.Context) (fsm.State, error) {
	apiServer, err := api.NewApi(p.opt.Addr, p.bazelBuilder, p.dao, p.ghClient, storage.NewMinIOStorage(p.opt.BazelMirrorBucket, p.bazelMirrorMinIOOpt), p.opt.BazelMirrorPrefix)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	p.apiServer = apiServer

	go func() {
		logger.Log.Info("Start API Server", zap.String("addr", p.apiServer.Addr))
		p.apiServer.ListenAndServe()
	}()

	return stateLeaderElection, nil
}

// leaderElection will get the lock.
// Next state: stateStartWorker
func (p *process) leaderElection(_ context.Context) (fsm.State, error) {
	if p.kubeClient == nil || p.opt.LeaseLockName == "" || p.opt.LeaseLockNamespace == "" {
		logger.Log.Info("Skip leader election")
		return fsm.Next(stateStartWorker)
	}

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      p.opt.LeaseLockName,
			Namespace: p.opt.LeaseLockNamespace,
		},
		Client: p.kubeClient.CoordinationV1(),
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
	if p.coreSharedInformerFactory != nil {
		jobWatcher := watcher.NewJobWatcher(p.coreSharedInformerFactory.Batch().V1().Jobs())

		p.coreSharedInformerFactory.Start(p.ctx.Done())

		go func() {
			logger.Log.Info("Start JobWatcher")
			if err := jobWatcher.Run(p.ctx, 1); err != nil {
				logger.Log.Error("Error occurred at JobWatcher", zap.Error(err))
				return
			}
		}()
	}

	if p.opt.WithGC {
		g := gc.NewGC(1*time.Hour, p.dao, p.opt.MinIOBucket, p.minioOpt)
		go func() {
			logger.Log.Info("Start GC")
			g.Start()
		}()
	}

	return fsm.Wait()
}

func (p *process) shutdown(ctx context.Context) (fsm.State, error) {
	logger.Log.Info("Shutting down")
	if p.apiServer != nil {
		p.apiServer.Shutdown(ctx)
		logger.Log.Info("Shutdown API Server")
	}

	return fsm.Finish()
}

func builder(ctx context.Context, opt Options) error {
	p := newProcess(opt)

	if err := p.FSM.LoopContext(ctx); err != nil {
		return err
	}

	return nil
}

func AddCommand(rootCmd *cli.Command) {
	opt := Options{}

	cmd := &cli.Command{
		Use: "builder",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			if v := os.Getenv("GITHUB_APP_ID"); v != "" {
				appId, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return err
				}
				opt.GithubAppId = appId
			}
			if v := os.Getenv("GITHUB_INSTALLATION_ID"); v != "" {
				installationId, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return err
				}
				opt.GithubInstallationId = installationId
			}
			if v := os.Getenv("GITHUB_PRIVATEKEY_FILE"); v != "" {
				opt.GithubPrivateKeyFile = v
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
	fs.Int64("github-app-id", "GitHub App id").Var(&opt.GithubAppId)
	fs.Int64("github-installation-id", "GitHub Installation id").Var(&opt.GithubInstallationId)
	fs.String("github-private-key-file", "PrivateKey file path of GitHub App").Var(&opt.GithubPrivateKeyFile)
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
	fs.String("sidecar-image", "Sidecar container image").Var(&opt.SidecarImage).Default("registry.f110.dev/build/sidecar")
	fs.String("ctl-image", "CLI container image").Var(&opt.CLIImage).Default("registry.f110.dev/build/buildctl")
	fs.Bool("pull-always", "Pull always").Var(&opt.PullAlways)
	fs.String("task-cpu-limit", "Task cpu limit. If the job set the limit, It will used the job defined value.").Var(&opt.TaskCPULimit).Default("1000m")
	fs.String("task-memory-limit", "Task memory limit. If the job set the limit, It will used the job defined value.").Var(&opt.TaskMemoryLimit).Default("4096Mi")
	fs.Bool("with-gc", "Enable GC for the job").Var(&opt.WithGC)
	fs.StringArray("exclude-nodes", "THe list of node to not assigned job").Var(&opt.ExcludeNodes)
	fs.Bool("debug", "Enable debugging mode").Var(&opt.Debug)

	rootCmd.AddCommand(cmd)
}
