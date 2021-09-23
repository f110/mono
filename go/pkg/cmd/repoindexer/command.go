package repoindexer

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"go.f110.dev/mono/go/pkg/logger"
)

type RunOption struct {
	ConfigFile     string
	WorkDir        string
	Token          string
	Ctags          string
	RunScheduler   bool
	InitRun        bool
	WithoutFetch   bool
	DisableCleanup bool
	Parallelism    int
	AppId          int64
	InstallationId int64
	PrivateKeyFile string

	MinIOName            string
	MinIONamespace       string
	MinIOPort            int
	MinIOBucket          string
	MinIOAccessKey       string
	MinIOSecretAccessKey string
}

func NewRunOption() *RunOption {
	d, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	return &RunOption{
		WorkDir:     d,
		MinIOPort:   9000,
		Parallelism: 1,
	}
}

func RepoIndexer(opt *RunOption, dev bool) error {
	config, err := ReadConfigFile(opt.ConfigFile)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	indexer, err := NewIndexer(
		config,
		opt.WorkDir,
		opt.Token,
		opt.Ctags,
		opt.AppId,
		opt.InstallationId,
		opt.PrivateKeyFile,
		opt.InitRun,
		opt.Parallelism,
	)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if opt.RunScheduler {
		if err := scheduler(config, indexer); err != nil {
			return xerrors.Errorf(": %w", err)
		}
		return nil
	}

	if !opt.WithoutFetch {
		if err := indexer.Sync(); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}
	if err := indexer.BuildIndex(); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if opt.MinIOName != "" && opt.MinIONamespace != "" && opt.MinIOBucket != "" {
		var k8sConf *rest.Config
		if dev {
			h, err := os.UserHomeDir()
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			kubeconfigPath := filepath.Join(h, ".kube/config")
			cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			k8sConf = cfg
		} else {
			cfg, err := rest.InClusterConfig()
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			k8sConf = cfg
		}
		k8sClient, err := kubernetes.NewForConfig(k8sConf)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		uploader := NewObjectStorageUploader(
			k8sClient,
			k8sConf,
			opt.MinIOName,
			opt.MinIONamespace,
			opt.MinIOPort,
			opt.MinIOBucket,
			opt.MinIOAccessKey,
			opt.MinIOSecretAccessKey,
			dev,
		)
		for _, v := range indexer.Indexes {
			err := uploader.Upload(context.Background(), v.Name, v.Files)
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
		}
	}
	if !opt.DisableCleanup {
		if err := indexer.Cleanup(); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func scheduler(config *Config, indexer *Indexer) error {
	c := cron.New()
	_, err := c.AddFunc(config.RefreshSchedule, func() {
		if err := indexer.Sync(); err != nil {
			logger.Log.Info("Failed sync", zap.Error(err))
		}
		if err := indexer.BuildIndex(); err != nil {
			logger.Log.Info("Failed build index", zap.Error(err))
		}
		if err := indexer.Cleanup(); err != nil {
			logger.Log.Info("Failed cleanup", zap.Error(err))
		}
		indexer.Reset()
	})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	logger.Log.Info("Start scheduler")
	c.Start()

	ctx, stopFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopFunc()

	<-ctx.Done()
	logger.Log.Debug("Got signal")

	ctx = c.Stop()

	logger.Log.Info("Waiting for stop scheduler")
	<-ctx.Done()

	return nil
}
