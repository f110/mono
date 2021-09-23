package repoindexer

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

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

	NATSURL string
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
		if err := scheduler(config, indexer, opt, dev); err != nil {
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
	if err := uploadIndex(
		indexer,
		opt.MinIOName,
		opt.MinIONamespace,
		opt.MinIOPort,
		opt.MinIOBucket,
		opt.MinIOAccessKey,
		opt.MinIOSecretAccessKey,
		opt.NATSURL,
		dev,
	); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if !opt.DisableCleanup {
		if err := indexer.Cleanup(); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func scheduler(config *Config, indexer *Indexer, opt *RunOption, dev bool) error {
	c := cron.New()
	_, err := c.AddFunc(config.RefreshSchedule, func() {
		defer indexer.Reset()
		if err := indexer.Sync(); err != nil {
			logger.Log.Info("Failed sync", zap.Error(err))
			return
		}
		if err := indexer.BuildIndex(); err != nil {
			logger.Log.Info("Failed build index", zap.Error(err))
			return
		}
		if err := uploadIndex(
			indexer,
			opt.MinIOName,
			opt.MinIONamespace,
			opt.MinIOPort,
			opt.MinIOBucket,
			opt.MinIOAccessKey,
			opt.MinIOSecretAccessKey,
			opt.NATSURL,
			dev,
		); err != nil {
			logger.Log.Info("Failed upload an index", zap.Error(err))
			return
		}
		if err := indexer.Cleanup(); err != nil {
			logger.Log.Info("Failed cleanup", zap.Error(err))
			return
		}
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

func uploadIndex(
	indexer *Indexer,
	minioName, minioNamespace string,
	minioPort int,
	minioBucket, minioAccessKey, minioSecretAccessKey string,
	natsURL string,
	dev bool,
) error {
	if minioName == "" || minioNamespace == "" || minioBucket == "" {
		return nil
	}

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
		minioName,
		minioNamespace,
		minioPort,
		minioBucket,
		minioAccessKey,
		minioSecretAccessKey,
		dev,
	)
	uploadedPath := make([]string, 0)
	for _, v := range indexer.Indexes {
		uploadDir, err := uploader.Upload(context.Background(), v.Name, v.Files)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		uploadedPath = append(uploadedPath, uploadDir)
	}

	if natsURL != "" {
		n, err := NewNotify(natsURL)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		if err := n.Notify(ctx, uploadedPath); err != nil {
			cancel()
			return xerrors.Errorf(": %w", err)
		}
		cancel()
	}

	return nil
}
