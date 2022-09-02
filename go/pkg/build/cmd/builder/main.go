package builder

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/go-sql-driver/mysql"
	"github.com/google/go-github/v32/github"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"go.f110.dev/mono/go/pkg/build/api"
	"go.f110.dev/mono/go/pkg/build/coordinator"
	"go.f110.dev/mono/go/pkg/build/database/dao"
	"go.f110.dev/mono/go/pkg/build/gc"
	"go.f110.dev/mono/go/pkg/build/watcher"
	"go.f110.dev/mono/go/pkg/fsm"
	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/storage"
)

type Options struct {
	Id                   string // Identity name. This name used to leader election.
	DSN                  string // DataSourceName.
	Namespace            string
	EnableLeaderElection bool
	LeaseLockName        string
	LeaseLockNamespace   string
	GithubAppId          int64
	GithubInstallationId int64
	GithubPrivateKeyFile string
	GithubAppSecretName  string
	MinIOName            string
	MinIONamespace       string
	MinIOPort            int
	MinIOBucket          string
	MinIOAccessKey       string
	MinIOSecretAccessKey string

	Addr                string
	DashboardUrl        string // URL of dashboard that can access people via browser
	BuilderApiUrl       string // URL of the api of builder.
	RemoteCache         string // If not empty, This value will passed to Bazel through --remote_cache argument.
	RemoteAssetApi      bool   // Use Remote Asset API. An api is experimental and depends on remote cache with gRPC.
	BazelImage          string
	DefaultBazelVersion string
	SidecarImage        string
	CLIImage            string
	TaskCPULimit        string
	TaskMemoryLimit     string
	WithGC              bool

	Dev   bool
	Debug bool
}

const (
	stateInit fsm.State = iota
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
	restCfg                   *rest.Config
	coreSharedInformerFactory kubeinformers.SharedInformerFactory
	dao                       dao.Options
	minioOpt                  storage.MinIOOptions

	bazelBuilder *coordinator.BazelBuilder
	apiServer    *api.Api
}

func newProcess(opt Options) *process {
	ctx, cancel := context.WithCancel(context.Background())
	p := &process{ctx: ctx, cancel: cancel, opt: opt}
	p.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:           p.init,
			stateSetup:          p.setup,
			stateStartApiServer: p.startApiServer,
			stateLeaderElection: p.leaderElection,
			stateStartWorker:    p.startWorker,
			stateShutdown:       p.shutdown,
		},
		stateInit,
		stateShutdown,
	)
	p.FSM.SignalHandling(os.Interrupt, syscall.SIGTERM)

	return p
}

func (p *process) init() (fsm.State, error) {
	kubeConfigPath := ""
	if p.opt.Dev {
		h, err := os.UserHomeDir()
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		kubeConfigPath = filepath.Join(h, ".kube", "config")
	}

	t, err := ghinstallation.NewKeyFromFile(
		http.DefaultTransport,
		p.opt.GithubAppId,
		p.opt.GithubInstallationId,
		p.opt.GithubPrivateKeyFile,
	)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	p.ghClient = github.NewClient(&http.Client{Transport: t})

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
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
	conn, err := sql.Open("mysql", p.opt.DSN)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	p.dao = dao.NewOptions(conn)

	return stateSetup, nil
}

func (p *process) setup() (fsm.State, error) {
	minioOpt := storage.NewMinIOOptionsViaService(
		p.kubeClient,
		p.restCfg,
		p.opt.MinIOName,
		p.opt.MinIONamespace,
		p.opt.MinIOPort,
		p.opt.MinIOAccessKey,
		p.opt.MinIOSecretAccessKey,
		p.opt.Dev,
	)
	p.minioOpt = minioOpt
	kubernetesOpt := coordinator.NewKubernetesOptions(
		p.coreSharedInformerFactory.Batch().V1().Jobs(),
		p.coreSharedInformerFactory.Core().V1().Pods(),
		p.kubeClient,
		p.restCfg,
		p.opt.TaskCPULimit,
		p.opt.TaskMemoryLimit,
	)
	bazelOpt := coordinator.NewBazelOptions(
		p.opt.RemoteCache,
		p.opt.RemoteAssetApi,
		p.opt.SidecarImage,
		p.opt.BazelImage,
		p.opt.DefaultBazelVersion,
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
		p.opt.Dev,
	)
	if err != nil {
		logger.Log.Error("Failed create BazelBuilder", zap.Error(err))
		return fsm.Error(xerrors.WithStack(err))
	}
	p.bazelBuilder = c

	return stateStartApiServer, nil
}

func (p *process) startApiServer() (fsm.State, error) {
	apiServer, err := api.NewApi(p.opt.Addr, p.bazelBuilder, p.dao, p.ghClient)
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

func (p *process) leaderElection() (fsm.State, error) {
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
		return fsm.UnknownState, nil
	}

	return stateStartWorker, nil
}

func (p *process) startWorker() (fsm.State, error) {
	jobWatcher := watcher.NewJobWatcher(p.coreSharedInformerFactory.Batch().V1().Jobs())

	p.coreSharedInformerFactory.Start(p.ctx.Done())

	go func() {
		logger.Log.Info("Start JobWatcher")
		if err := jobWatcher.Run(p.ctx, 1); err != nil {
			logger.Log.Error("Error occurred at JobWatcher", zap.Error(err))
			return
		}
	}()

	if p.opt.WithGC {
		g := gc.NewGC(1*time.Hour, p.dao, p.opt.MinIOBucket, p.minioOpt)
		go func() {
			logger.Log.Info("Start GC")
			g.Start()
		}()
	}

	return fsm.WaitState, nil
}

func (p *process) shutdown() (fsm.State, error) {
	if p.apiServer != nil {
		p.apiServer.Shutdown(context.Background())
		logger.Log.Info("Shutdown API Server")
	}
	return fsm.CloseState, nil
}

func builder(opt Options) error {
	p := newProcess(opt)

	if err := p.FSM.Loop(); err != nil {
		return err
	}

	return nil
}

func AddCommand(rootCmd *cobra.Command) {
	opt := Options{}

	cmd := &cobra.Command{
		Use: "builder",
		RunE: func(_ *cobra.Command, _ []string) error {
			return builder(opt)
		},
	}

	fs := cmd.Flags()
	fs.StringVar(&opt.DSN, "dsn", "", "Data source name")
	fs.StringVar(&opt.Id, "id", uuid.New().String(), "the holder identity name")
	fs.BoolVar(
		&opt.EnableLeaderElection,
		"enable-leader-election",
		opt.EnableLeaderElection,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.",
	)
	fs.StringVar(&opt.LeaseLockName, "lease-lock-name", "", "the lease lock resource name")
	fs.StringVar(&opt.LeaseLockNamespace, "lease-lock-namespace", "", "the lease lock resource namespace")
	fs.StringVar(&opt.Namespace, "namespace", "", "The namespace which will be created  the job")
	fs.StringVar(
		&opt.GithubAppSecretName,
		"github-app-secret-name",
		"",
		"The name of Secret which contains github app id, installation id and private key.",
	)
	fs.Int64Var(&opt.GithubAppId, "github-app-id", 0, "GitHub App id")
	fs.Int64Var(&opt.GithubInstallationId, "github-installation-id", 0, "GitHub Installation id")
	fs.StringVar(&opt.GithubPrivateKeyFile, "github-private-key-file", "", "PrivateKey file path of GitHub App")
	fs.StringVar(&opt.Addr, "addr", "127.0.0.1:8081", "Listen addr which will be served API")
	fs.StringVar(&opt.DashboardUrl, "dashboard", "http://localhost", "URL of dashboard")
	fs.StringVar(&opt.BuilderApiUrl, "builder-api", "http://localhost", "URL of the api of builder")
	fs.BoolVar(&opt.Dev, "dev", opt.Dev, "development mode")
	fs.StringVar(&opt.MinIOName, "minio-name", "", "The name of MinIO")
	fs.StringVar(&opt.MinIONamespace, "minio-namespace", "", "The namespace of MinIO")
	fs.IntVar(&opt.MinIOPort, "minio-port", 8080, "Port number of MinIO")
	fs.StringVar(&opt.MinIOBucket, "minio-bucket", "logs", "The bucket name that will be used a log storage")
	fs.StringVar(&opt.MinIOAccessKey, "minio-access-key", "", "The access key")
	fs.StringVar(&opt.MinIOSecretAccessKey, "minio-secret-access-key", "", "The secret access key")
	fs.StringVar(&opt.RemoteCache, "remote-cache", "", "The url of remote cache of bazel.")
	fs.BoolVar(&opt.RemoteAssetApi, "remote-asset", false, "Enable Remote Asset API. This is experimental feature.")
	fs.StringVar(&opt.BazelImage, "bazel-image", "l.gcr.io/google/bazel", "Bazel container image")
	fs.StringVar(&opt.DefaultBazelVersion, "default-bazel-version", "3.2.0", "Default bazel version")
	fs.StringVar(&opt.SidecarImage, "sidecar-image", "registry.f110.dev/build/sidecar", "Sidecar container image")
	fs.StringVar(&opt.CLIImage, "ctl-image", "registry.f110.dev/build/buildctl", "CLI container image")
	fs.StringVar(
		&opt.TaskCPULimit,
		"task-cpu-limit",
		"1000m",
		"Task cpu limit. If the job set the limit, It will used the job defined value.",
	)
	fs.StringVar(
		&opt.TaskMemoryLimit,
		"task-memory-limit",
		"4096Mi",
		"Task memory limit. If the job set the limit, It will used the job defined value.",
	)
	fs.BoolVar(&opt.WithGC, "with-gc", false, "Enable GC for the job")
	fs.BoolVar(&opt.Debug, "debug", false, "Enable debugging mode")

	rootCmd.AddCommand(cmd)
}
