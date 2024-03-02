package dashboard

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/web"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/signals"
	"go.f110.dev/mono/go/storage"
)

func dashboard(opt Options) error {
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

	ctx, cancelFunc := context.WithCancel(context.Background())
	signals.SetupSignalHandler(cancelFunc)

	logger.Log.Debug("Open sql connection", zap.String("dsn", opt.DSN))
	conn, err := sql.Open("mysql", opt.DSN)
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
		<-ctx.Done()
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

func AddCommand(rootCmd *cobra.Command) {
	opt := Options{}
	cmd := &cobra.Command{
		Use: "dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return dashboard(opt)
		},
	}

	fs := cmd.Flags()
	fs.StringVar(&opt.Addr, "addr", "", "Listen address")
	fs.StringVar(&opt.DSN, "dsn", "", "Data source name")
	fs.StringVar(&opt.ApiHost, "api", "", "API Host which user's browser can access")
	fs.StringVar(&opt.InternalApi, "internal-api", "", "The URL for internal api")
	fs.BoolVar(&opt.Dev, "dev", false, "development mode")
	fs.StringVar(&opt.MinIOEndpoint, "minio-endpoint", "", "The endpoint of MinIO. If this value is empty, then we find the endpoint from kube-apiserver using incluster config.")
	fs.StringVar(&opt.MinIOName, "minio-name", "", "The name of MinIO")
	fs.StringVar(&opt.MinIONamespace, "minio-namespace", "", "The namespace of MinIO")
	fs.IntVar(&opt.MinIOPort, "minio-port", 8080, "Port number of MinIO")
	fs.StringVar(&opt.MinIOBucket, "minio-bucket", "logs", "The bucket name that will be used a log storage")
	fs.StringVar(&opt.MinIOAccessKey, "minio-access-key", "", "The access key")
	fs.StringVar(&opt.MinIOSecretAccessKey, "minio-secret-access-key", "", "The secret access key")

	rootCmd.AddCommand(cmd)
}
