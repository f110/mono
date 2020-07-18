package builder

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"go.f110.dev/mono/lib/logger"
	"go.f110.dev/mono/lib/signals"
	"go.f110.dev/mono/tools/build/pkg/api"
	"go.f110.dev/mono/tools/build/pkg/coordinator"
	"go.f110.dev/mono/tools/build/pkg/database/dao"
	"go.f110.dev/mono/tools/build/pkg/discovery"
	"go.f110.dev/mono/tools/build/pkg/storage"
	"go.f110.dev/mono/tools/build/pkg/watcher"
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
	MinIOName            string
	MinIONamespace       string
	MinIOPort            int
	MinIOBucket          string
	MinIOAccessKey       string
	MinIOSecretAccessKey string

	Addr                string
	DashboardUrl        string // URL of dashboard that can access people via browser
	RemoteCache         string // If not empty, This value will passed to Bazel through --remote_cache argument.
	RemoteAssetApi      bool   // Use Remote Asset API. An api is experimental and depends on remote cache with gRPC.
	BazelImage          string
	DefaultBazelVersion string
	SidecarImage        string
	TaskCPULimit        string
	TaskMemoryLimit     string

	Dev bool
}

func builder(opt Options) error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	signals.SetupSignalHandler(cancelFunc)

	kubeConfigPath := ""
	if opt.Dev {
		h, err := os.UserHomeDir()
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		kubeConfigPath = filepath.Join(h, ".kube", "config")
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	coreSharedInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 30*time.Second, kubeinformers.WithNamespace(opt.Namespace))

	parsedDSN, err := mysql.ParseDSN(opt.DSN)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	parsedDSN.ParseTime = true
	loc, err := time.LoadLocation("Local")
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	parsedDSN.Loc = loc
	opt.DSN = parsedDSN.FormatDSN()

	logger.Log.Debug("Open sql connection", zap.String("dsn", opt.DSN))
	conn, err := sql.Open("mysql", opt.DSN)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	daoOpt := dao.NewOptions(conn)

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      opt.LeaseLockName,
			Namespace: opt.LeaseLockNamespace,
		},
		Client: kubeClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: opt.Id,
		},
	}
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   30 * time.Second,
		RenewDeadline:   15 * time.Second,
		RetryPeriod:     5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				jobWatcher := watcher.NewJobWatcher(coreSharedInformerFactory.Batch().V1().Jobs())

				minioOpt := storage.NewMinIOOptions(opt.MinIOName, opt.MinIONamespace, opt.MinIOPort, opt.MinIOBucket, opt.MinIOAccessKey, opt.MinIOSecretAccessKey)
				githubOpt := coordinator.NewGithubAppOptions(opt.GithubAppId, opt.GithubInstallationId, opt.GithubPrivateKeyFile)
				kubernetesOpt := coordinator.NewKubernetesOptions(coreSharedInformerFactory.Batch().V1().Jobs(), kubeClient, cfg, opt.TaskCPULimit, opt.TaskMemoryLimit)
				bazelOpt := coordinator.NewBazelOptions(opt.RemoteCache, opt.RemoteAssetApi, opt.SidecarImage, opt.BazelImage, opt.DefaultBazelVersion)
				c, err := coordinator.NewBazelBuilder(opt.DashboardUrl, kubernetesOpt, daoOpt, opt.Namespace, githubOpt, minioOpt, bazelOpt, opt.Dev)
				if err != nil {
					logger.Log.Error("Failed create BazelBuilder", zap.Error(err))
					return
				}

				d := discovery.NewDiscover(coreSharedInformerFactory.Batch().V1().Jobs(), kubeClient, opt.Namespace, daoOpt, c, opt.SidecarImage)

				apiServer, err := api.NewApi(opt.Addr, c, d, daoOpt, opt.GithubAppId, opt.GithubInstallationId, opt.GithubPrivateKeyFile)
				if err != nil {
					return
				}

				coreSharedInformerFactory.Start(ctx.Done())

				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					defer wg.Done()

					logger.Log.Info("Start API Server", zap.String("addr", apiServer.Addr))
					apiServer.ListenAndServe()
				}()

				wg.Add(1)
				go func() {
					defer wg.Done()

					if err := jobWatcher.Run(ctx, 1); err != nil {
						logger.Log.Error("Error occurred at JobWatcher", zap.Error(err))
						return
					}
				}()

				wg.Add(1)
				go func() {
					defer wg.Done()

					<-ctx.Done()
					apiServer.Shutdown(context.Background())
				}()

				wg.Wait()
				logger.Log.Debug("Shutdown")
			},
			OnStoppedLeading: func() {
				logger.Log.Debug("leader lost", zap.String("id", opt.Id))
			},
			OnNewLeader: func(identity string) {
				if identity == opt.Id {
					return
				}
				logger.Log.Info("new leader elected", zap.String("id", identity))
			},
		},
	})

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
	fs.BoolVar(&opt.EnableLeaderElection, "enable-leader-election", opt.EnableLeaderElection,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	fs.StringVar(&opt.LeaseLockName, "lease-lock-name", "", "the lease lock resource name")
	fs.StringVar(&opt.LeaseLockNamespace, "lease-lock-namespace", "", "the lease lock resource namespace")
	fs.StringVar(&opt.Namespace, "namespace", "", "The namespace which will be created  the job")
	fs.Int64Var(&opt.GithubAppId, "github-app-id", 0, "GitHub App id")
	fs.Int64Var(&opt.GithubInstallationId, "github-installation-id", 0, "GitHub Installation id")
	fs.StringVar(&opt.GithubPrivateKeyFile, "github-private-key-file", "", "PrivateKey file path of GitHub App")
	fs.StringVar(&opt.Addr, "addr", "127.0.0.1:8081", "Listen addr which will be served API")
	fs.StringVar(&opt.DashboardUrl, "dashboard", "http://localhost", "URL of dashboard")
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
	fs.StringVar(&opt.TaskCPULimit, "task-cpu-limit", "1000m", "Task cpu limit. If the job set the limit, It will used the job defined value.")
	fs.StringVar(&opt.TaskMemoryLimit, "task-memory-limit", "4096Mi", "Task memory limit. If the job set the limit, It will used the job defined value.")

	rootCmd.AddCommand(cmd)
}
