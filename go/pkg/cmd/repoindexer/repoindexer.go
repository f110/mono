package repoindexer

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/storage"
)

type IndexerCommand struct {
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

	Dev bool
}

func NewIndexerCommand() *IndexerCommand {
	d, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	return &IndexerCommand{
		WorkDir:     d,
		MinIOPort:   9000,
		Parallelism: 1,
	}
}

func (r *IndexerCommand) Flags(fs *pflag.FlagSet) {
	fs.StringVarP(&r.ConfigFile, "config", "c", r.ConfigFile, "Config file")
	fs.StringVar(&r.WorkDir, "work-dir", r.WorkDir, "Working root directory")
	fs.StringVar(&r.Token, "token", r.Token, "GitHub API token")
	fs.StringVar(&r.Ctags, "ctags", r.Ctags, "ctags path")
	fs.BoolVar(&r.RunScheduler, "run-scheduler", r.RunScheduler, "")
	fs.BoolVar(&r.InitRun, "init-run", r.InitRun, "")
	fs.BoolVar(&r.WithoutFetch, "without-fetch", r.WithoutFetch, "Disable fetch")
	fs.BoolVar(&r.DisableCleanup, "disable-cleanup", r.DisableCleanup, "Disable cleanup")
	fs.IntVar(&r.Parallelism, "parallelism", r.Parallelism, "The number of workers")
	fs.Int64Var(&r.AppId, "app-id", r.AppId, "GitHub Application ID")
	fs.Int64Var(&r.InstallationId, "installation-id", r.InstallationId, "GitHub Application installation ID")
	fs.StringVar(&r.PrivateKeyFile, "private-key-file", r.PrivateKeyFile, "Private key file for GitHub App")
	fs.StringVar(&r.MinIOName, "minio-name", r.MinIOName, "The name of MinIO")
	fs.StringVar(&r.MinIONamespace, "minio-namespace", r.MinIONamespace, "The namespace of MinIO")
	fs.IntVar(&r.MinIOPort, "minio-port", r.MinIOPort, "Port number of MinIO")
	fs.StringVar(&r.MinIOBucket, "minio-bucket", r.MinIOBucket, "The bucket name that will be used")
	fs.StringVar(&r.MinIOAccessKey, "minio-access-key", r.MinIOAccessKey, "The access key")
	fs.StringVar(&r.MinIOSecretAccessKey, "minio-secret-access-key", r.MinIOSecretAccessKey, "The secret access key")
	fs.StringVar(&r.NATSURL, "nats-url", r.NATSURL, "The URL for nats-server")
	fs.BoolVar(&r.DisableObjectStorageCleanup, "disable-object-storage-cleanup", r.DisableObjectStorageCleanup, "Disable cleanup of the object storage")
	fs.BoolVar(&r.Dev, "dev", r.Dev, "Development mode")
}

func (r *IndexerCommand) Run() error {
	config, err := ReadConfigFile(r.ConfigFile)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	indexer, err := NewIndexer(
		config,
		r.WorkDir,
		r.Token,
		r.Ctags,
		r.AppId,
		r.InstallationId,
		r.PrivateKeyFile,
		r.InitRun,
		r.Parallelism,
	)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if r.RunScheduler {
		if err := r.scheduler(config, indexer); err != nil {
			return xerrors.Errorf(": %w", err)
		}
		return nil
	}

	if err := r.run(indexer); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (r *IndexerCommand) run(indexer *Indexer) error {
	enableObjectStorageUpload := r.MinIOName != "" && r.MinIONamespace != "" && r.MinIOBucket != ""

	if !r.WithoutFetch {
		if err := indexer.Sync(); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}
	if err := indexer.BuildIndex(); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if enableObjectStorageUpload {
		manifest, err := r.uploadIndex(
			indexer,
			r.MinIOName,
			r.MinIONamespace,
			r.MinIOPort,
			r.MinIOBucket,
			r.MinIOAccessKey,
			r.MinIOSecretAccessKey,
			r.DisableObjectStorageCleanup,
			r.Dev,
		)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		if r.NATSURL != "" {
			n, err := NewNotify(r.NATSURL)
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
	if !r.DisableCleanup {
		if err := indexer.Cleanup(); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func (r *IndexerCommand) scheduler(config *Config, indexer *Indexer) error {
	c := cron.New()
	_, err := c.AddFunc(config.RefreshSchedule, func() {
		defer indexer.Reset()

		if err := r.run(indexer); err != nil {
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

func (r *IndexerCommand) uploadIndex(
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
