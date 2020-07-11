package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"go.f110.dev/mono/lib/logger"
	"go.f110.dev/mono/tools/build/pkg/database/dao"
	"go.f110.dev/mono/tools/build/pkg/storage"
	"go.f110.dev/mono/tools/build/pkg/web"
)

func dashboard(args []string) error {
	addr := "127.0.0.1:8080"
	dsn := ""
	apiHost := ""
	dev := false
	minIOName := ""
	minIONamespace := ""
	minIOPort := 0
	minIOBucket := ""
	minIOAccessKey := ""
	minIOSecretAccessKey := ""
	fs := pflag.NewFlagSet("dashboard", pflag.ContinueOnError)
	fs.StringVar(&addr, "addr", "", "Listen address")
	fs.StringVar(&dsn, "dsn", "", "Data source name")
	fs.StringVar(&apiHost, "api", "", "API Host which user's browser can access.")
	fs.BoolVar(&dev, "dev", dev, "development mode")
	fs.StringVar(&minIOName, "minio-name", "", "The name of MinIO")
	fs.StringVar(&minIONamespace, "minio-namespace", "", "The namespace of MinIO")
	fs.IntVar(&minIOPort, "minio-port", 8080, "Port number of MinIO")
	fs.StringVar(&minIOBucket, "minio-bucket", "logs", "The bucket name that will be used a log storage")
	fs.StringVar(&minIOAccessKey, "minio-access-key", "", "The access key")
	fs.StringVar(&minIOSecretAccessKey, "minio-secret-access-key", "", "The secret access key")
	logger.Flags(fs)
	if err := fs.Parse(args); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if err := logger.Init(); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	parsedDSN, err := mysql.ParseDSN(dsn)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	parsedDSN.ParseTime = true
	loc, err := time.LoadLocation("Local")
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	parsedDSN.Loc = loc
	dsn = parsedDSN.FormatDSN()

	logger.Log.Debug("Open sql connection", zap.String("dsn", dsn))
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	kubeConfigPath := ""
	if dev {
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

	minioOpt := storage.NewMinIOOptions(minIOName, minIONamespace, minIOPort, minIOBucket, minIOAccessKey, minIOSecretAccessKey)
	d := web.NewDashboard(addr, dao.NewOptions(conn), apiHost, kubeClient, cfg, minioOpt, dev)
	logger.Log.Info("Listen", zap.String("addr", addr))
	if err := d.Start(); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func main() {
	if err := dashboard(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
