package dashboard

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"go.f110.dev/mono/lib/logger"
	"go.f110.dev/mono/tools/build/pkg/database/dao"
	"go.f110.dev/mono/tools/build/pkg/storage"
	"go.f110.dev/mono/tools/build/pkg/web"
)

func dashboard(opt Options) error {
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

	minioOpt := storage.NewMinIOOptions(opt.MinIOName, opt.MinIONamespace, opt.MinIOPort, opt.MinIOBucket, opt.MinIOAccessKey, opt.MinIOSecretAccessKey)
	d := web.NewDashboard(opt.Addr, dao.NewOptions(conn), opt.ApiHost, kubeClient, cfg, minioOpt, opt.Dev)
	logger.Log.Info("Listen", zap.String("addr", opt.Addr))
	if err := d.Start(); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

type Options struct {
	Addr                 string
	DSN                  string
	ApiHost              string
	Dev                  bool
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
	fs.StringVar(&opt.ApiHost, "api", "", "API Host which user's browser can access.")
	fs.BoolVar(&opt.Dev, "dev", false, "development mode")
	fs.StringVar(&opt.MinIOName, "minio-name", "", "The name of MinIO")
	fs.StringVar(&opt.MinIONamespace, "minio-namespace", "", "The namespace of MinIO")
	fs.IntVar(&opt.MinIOPort, "minio-port", 8080, "Port number of MinIO")
	fs.StringVar(&opt.MinIOBucket, "minio-bucket", "logs", "The bucket name that will be used a log storage")
	fs.StringVar(&opt.MinIOAccessKey, "minio-access-key", "", "The access key")
	fs.StringVar(&opt.MinIOSecretAccessKey, "minio-secret-access-key", "", "The secret access key")

	rootCmd.AddCommand(cmd)
}
