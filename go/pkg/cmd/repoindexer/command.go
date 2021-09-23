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
	"go.f110.dev/mono/go/pkg/storage"
)

type IndexerRunOption struct {
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

	MinIOName                   string
	MinIONamespace              string
	MinIOPort                   int
	MinIOBucket                 string
	MinIOAccessKey              string
	MinIOSecretAccessKey        string
	DisableObjectStorageCleanup bool

	NATSURL string
}

func NewIndexerRunOption() *IndexerRunOption {
	d, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	return &IndexerRunOption{
		WorkDir:     d,
		MinIOPort:   9000,
		Parallelism: 1,
	}
}

func RepoIndexer(opt *IndexerRunOption, dev bool) error {
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

	if err := run(indexer, opt, dev); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func run(indexer *Indexer, opt *IndexerRunOption, dev bool) error {
	enableObjectStorageUpload := opt.MinIOName != "" && opt.MinIONamespace != "" && opt.MinIOBucket != ""

	if !opt.WithoutFetch {
		if err := indexer.Sync(); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}
	if err := indexer.BuildIndex(); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if enableObjectStorageUpload {
		manifest, err := uploadIndex(
			indexer,
			opt.MinIOName,
			opt.MinIONamespace,
			opt.MinIOPort,
			opt.MinIOBucket,
			opt.MinIOAccessKey,
			opt.MinIOSecretAccessKey,
			opt.DisableObjectStorageCleanup,
			dev,
		)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		if opt.NATSURL != "" {
			n, err := NewNotify(opt.NATSURL)
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			if err := n.Notify(ctx, manifest); err != nil {
				cancel()
				return xerrors.Errorf(": %w", err)
			}
			cancel()
		}
	}
	if !opt.DisableCleanup {
		if err := indexer.Cleanup(); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func scheduler(config *Config, indexer *Indexer, opt *IndexerRunOption, dev bool) error {
	c := cron.New()
	_, err := c.AddFunc(config.RefreshSchedule, func() {
		defer indexer.Reset()

		if err := run(indexer, opt, dev); err != nil {
			logger.Log.Info("Failed indexing", zap.Error(err))
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
	disableCleanup bool,
	dev bool,
) (*Manifest, error) {
	if minioName == "" || minioNamespace == "" || minioBucket == "" {
		return nil, nil
	}

	k8sClient, k8sConf, err := newK8sClient(dev)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	opt := storage.NewMinIOOptions(minioName, minioNamespace, minioPort, minioBucket, minioAccessKey, minioSecretAccessKey)
	s := storage.NewMinIOStorage(k8sClient, k8sConf, opt, dev)
	manager := NewObjectStorageIndexManager(s, minioBucket)
	uploadedPath := make(map[string]string, 0)
	for _, v := range indexer.Indexes {
		uploadDir, err := manager.Add(context.Background(), v.Name, v.Files)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		uploadedPath[v.Name] = uploadDir
	}

	manifest := NewManifest(manager.ExecutionKey(), uploadedPath)
	mm := NewManifestManager(s)
	if err := mm.Update(context.Background(), manifest); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	if !disableCleanup {
		expired, err := mm.FindExpiredManifests(context.Background())
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		if err := manager.Delete(context.Background(), expired); err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		for _, m := range expired {
			if err := mm.Delete(context.Background(), m); err != nil {
				return nil, xerrors.Errorf(": %w", err)
			}
		}
	}

	return &manifest, nil
}

func newK8sClient(dev bool) (kubernetes.Interface, *rest.Config, error) {
	var k8sConf *rest.Config
	if dev {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, nil, xerrors.Errorf(": %w", err)
		}
		kubeconfigPath := filepath.Join(h, ".kube/config")
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, nil, xerrors.Errorf(": %w", err)
		}
		k8sConf = cfg
	} else {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, nil, xerrors.Errorf(": %w", err)
		}
		k8sConf = cfg
	}
	k8sClient, err := kubernetes.NewForConfig(k8sConf)
	if err != nil {
		return nil, nil, xerrors.Errorf(": %w", err)
	}

	return k8sClient, k8sConf, nil
}

type UpdaterRunOption struct {
	IndexDir string

	MinIOName            string
	MinIONamespace       string
	MinIOPort            int
	MinIOBucket          string
	MinIOAccessKey       string
	MinIOSecretAccessKey string

	NATSURL string
}

func NewUpdaterRunOptions() *UpdaterRunOption {
	return &UpdaterRunOption{
		MinIOPort: 9000,
	}
}

func IndexUpdater(opt *UpdaterRunOption, dev bool) error {
	k8sClient, k8sConf, err := newK8sClient(dev)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	minioOpt := storage.NewMinIOOptions(opt.MinIOName, opt.MinIONamespace, opt.MinIOPort, opt.MinIOBucket, opt.MinIOAccessKey, opt.MinIOSecretAccessKey)
	s := storage.NewMinIOStorage(k8sClient, k8sConf, minioOpt, dev)

	mm := NewManifestManager(s)
	manifest, err := mm.GetLatest(context.Background())
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	logger.Log.Info("Found manifest", zap.Int64("createdAt", manifest.CreatedAt.Unix()))

	manager := NewObjectStorageIndexManager(s, opt.MinIOBucket)
	if err := manager.Download(context.Background(), opt.IndexDir, manifest); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}
