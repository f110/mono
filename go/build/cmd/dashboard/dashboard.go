package dashboard

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/web"
	"go.f110.dev/mono/go/cli"
	_ "go.f110.dev/mono/go/database/querylog"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/signals"
	"go.f110.dev/mono/go/storage"
)

func dashboard(ctx context.Context, opt Options) error {
	parsedDSN, err := mysql.ParseDSN(opt.DSN)
	if err != nil {
		return xerrors.WithStack(err)
	}
	parsedDSN.ParseTime = true
	loc, err := time.LoadLocation("Local")
	if err != nil {
		return xerrors.WithStack(err)
	}
	parsedDSN.Loc = loc
	opt.DSN = parsedDSN.FormatDSN()

	cCtx, cancelFunc := context.WithCancel(ctx)
	signals.SetupSignalHandler(cancelFunc)

	logger.Log.Debug("Open sql connection", zap.String("dsn", opt.DSN))
	conn, err := sql.Open("querylog", opt.DSN)
	if err != nil {
		return xerrors.WithStack(err)
	}

	var minioOpt storage.MinIOOptions
	if opt.MinIOEndpoint != "" {
		storage.NewMinIOOptionsViaEndpoint(opt.MinIOEndpoint, "", opt.MinIOAccessKey, opt.MinIOSecretAccessKey)
	} else {
		cfg, err := clientcmd.BuildConfigFromFlags("", "")
		if err != nil {
			return xerrors.WithStack(err)
		}

		kubeClient, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			return xerrors.WithStack(err)
		}
		minioOpt = storage.NewMinIOOptionsViaService(kubeClient, cfg, opt.MinIOName, opt.MinIONamespace, opt.MinIOPort, opt.MinIOAccessKey, opt.MinIOSecretAccessKey, opt.Dev)
	}

	d := web.NewDashboard(opt.Addr, dao.NewOptions(conn), opt.ApiHost, opt.InternalApi, opt.MinIOBucket, minioOpt)

	go func() {
		<-cCtx.Done()
		d.Shutdown(context.Background())
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Log.Info("Listen", zap.String("addr", opt.Addr))
		if err := d.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Info("Error", zap.Error(err))
			return
		}
	}()

	wg.Wait()

	return nil
}

type Options struct {
	Addr                 string
	DSN                  string
	ApiHost              string
	InternalApi          string
	Dev                  bool
	MinIOEndpoint        string
	MinIOName            string
	MinIONamespace       string
	MinIOPort            int
	MinIOBucket          string
	MinIOAccessKey       string
	MinIOSecretAccessKey string
}

func AddCommand(rootCmd *cli.Command) {
	opt := Options{}
	cmd := &cli.Command{
		Use: "dashboard",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return dashboard(ctx, opt)
		},
	}

	fs := cmd.Flags()
	fs.String("addr", "Listen address").Var(&opt.Addr)
	fs.String("dsn", "Data source name").Var(&opt.DSN)
	fs.String("api", "API Host which user's browser can access").Var(&opt.ApiHost)
	fs.String("internal-api", "The URL for internal api").Var(&opt.InternalApi)
	fs.Bool("dev", "development mode").Var(&opt.Dev)
	fs.String("minio-endpoint", "The endpoint of MinIO. If this value is empty, then we find the endpoint from kube-apiserver using incluster config.").Var(&opt.MinIOEndpoint)
	fs.String("minio-name", "The name of MinIO").Var(&opt.MinIOName)
	fs.String("minio-namespace", "The namespace of MinIO").Var(&opt.MinIONamespace)
	fs.Int("minio-port", "Port number of MinIO").Var(&opt.MinIOPort).Default(8080)
	fs.String("minio-bucket", "The bucket name that will be used a log storage").Var(&opt.MinIOBucket).Default("logs")
	fs.String("minio-access-key", "The access key").Var(&opt.MinIOAccessKey)
	fs.String("minio-secret-access-key", "The secret access key").Var(&opt.MinIOSecretAccessKey)

	rootCmd.AddCommand(cmd)
}
