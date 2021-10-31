package repoindexer

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/robfig/cron/v3"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/k8s/volume"
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
	HTTPAddr       string

	Bucket                      string
	MinIOEndpoint               string
	MinIOName                   string
	MinIONamespace              string
	MinIOPort                   int
	MinIOAccessKey              string
	MinIOSecretAccessKey        string
	S3Endpoint                  string
	S3Region                    string
	S3AccessKey                 string
	S3SecretAccessKey           string
	DisableObjectStorageCleanup bool

	NATSURL        string
	NATSStreamName string
	NATSSubject    string

	Dev bool

	config  *Config
	indexer *Indexer
	cron    *cron.Cron
	entryId cron.EntryID
	cancel  context.CancelFunc

	mu sync.Mutex
}

func NewIndexerCommand() *IndexerCommand {
	d, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	return &IndexerCommand{
		WorkDir:        d,
		MinIOPort:      9000,
		Parallelism:    1,
		NATSStreamName: "repoindexer",
		NATSSubject:    "notify",
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
	fs.BoolVar(&r.DisableCleanup, "disable-cleanup", r.DisableCleanup, "Disable cleanup in the working directory not the object storage")
	fs.IntVar(&r.Parallelism, "parallelism", r.Parallelism, "The number of workers")
	fs.Int64Var(&r.AppId, "app-id", r.AppId, "GitHub Application ID")
	fs.Int64Var(&r.InstallationId, "installation-id", r.InstallationId, "GitHub Application installation ID")
	fs.StringVar(&r.PrivateKeyFile, "private-key-file", r.PrivateKeyFile, "Private key file for GitHub App")
	fs.StringVar(&r.MinIOEndpoint, "minio-endpoint", r.MinIOEndpoint, "The endpoint of MinIO")
	fs.StringVar(&r.MinIOName, "minio-name", r.MinIOName, "The name of MinIO")
	fs.StringVar(&r.MinIONamespace, "minio-namespace", r.MinIONamespace, "The namespace of MinIO")
	fs.IntVar(&r.MinIOPort, "minio-port", r.MinIOPort, "Port number of MinIO")
	fs.StringVar(&r.Bucket, "minio-bucket", r.Bucket, "Deprecated. Use --bucket instead. The bucket name that will be used")
	fs.StringVar(&r.MinIOAccessKey, "minio-access-key", r.MinIOAccessKey, "The access key for MinIO API")
	fs.StringVar(&r.MinIOSecretAccessKey, "minio-secret-access-key", r.MinIOSecretAccessKey, "The secret access key for MinIO API")
	fs.StringVar(&r.S3Endpoint, "s3-endpoint", r.S3Endpoint, "The endpoint of s3. If you use the object storage has compatible s3 api not AWS S3, You can use this param")
	fs.StringVar(&r.S3Region, "s3-region", r.S3Region, "The region name")
	fs.StringVar(&r.S3AccessKey, "s3-access-key", r.S3AccessKey, "The access key for S3 API")
	fs.StringVar(&r.S3SecretAccessKey, "s3-secret-access-key", r.S3SecretAccessKey, "The secret access key for S3 API")
	fs.StringVar(&r.Bucket, "bucket", r.Bucket, "The bucket name")
	fs.StringVar(&r.NATSURL, "nats-url", r.NATSURL, "The URL for nats-server")
	fs.StringVar(&r.NATSStreamName, "nats-stream-name", r.NATSStreamName, "The name of stream for JetStream")
	fs.StringVar(&r.NATSSubject, "nats-subject", r.NATSSubject, "The subject of stream")
	fs.BoolVar(&r.DisableObjectStorageCleanup, "disable-object-storage-cleanup", r.DisableObjectStorageCleanup, "Disable cleanup in the object storage after uploaded the index")
	fs.BoolVar(&r.Dev, "dev", r.Dev, "Development mode")
	fs.StringVar(&r.HTTPAddr, "http-addr", r.HTTPAddr, "HTTP listen addr")
}

func (r *IndexerCommand) ValidateFlags() error {
	if r.ConfigFile == "" {
		return errors.New("--config is required")
	}

	if r.AppId > 0 || r.InstallationId > 0 || r.PrivateKeyFile != "" {
		// Run as GitHub App
		if r.Token != "" {
			return errors.New("GitHub token and App both configuration specified")
		}
	}

	if r.MinIOEndpoint != "" {
		if r.MinIOName != "" || r.MinIONamespace != "" {
			return errors.New("--minio-endpoint and --minio-name both specified")
		}
	}

	return nil
}

func (r *IndexerCommand) Run() error {
	if err := r.ValidateFlags(); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	config, err := ReadConfigFile(r.ConfigFile)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	r.config = config

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
	r.indexer = indexer
	if r.HTTPAddr != "" {
		if err := r.webEndpoint(r.HTTPAddr); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	if r.RunScheduler {
		if err := r.scheduler(config.RefreshSchedule); err != nil {
			return xerrors.Errorf(": %w", err)
		}
		return nil
	}
	if err := r.run(); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if err := r.gc(); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (r *IndexerCommand) run() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		r.indexer.Reset()
		cancel()
	}()
	r.cancel = cancel

	if !r.WithoutFetch {
		if err := r.indexer.Sync(ctx); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}
	if err := r.indexer.BuildIndex(ctx); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if r.enableUpload() {
		manifest, err := r.uploadIndex(ctx, r.indexer, r.Bucket, r.DisableObjectStorageCleanup)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		if r.NATSURL != "" {
			n, err := NewNotify(r.NATSURL, r.NATSStreamName, r.NATSSubject)
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			if err := n.Notify(ctx, manifest); err != nil {
				return xerrors.Errorf(": %w", err)
			}
		}
	} else {
		logger.Log.Debug("Disable upload", zap.Bool("can_use_minio", r.canUseMinIO()), zap.Bool("can_use_s3", r.canUseS3()))
	}
	if !r.DisableCleanup {
		if err := r.indexer.Cleanup(ctx); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func (r *IndexerCommand) scheduler(schedule string) error {
	if volume.CanWatchVolume(r.ConfigFile) {
		mountPath, err := volume.FindMountPath(r.ConfigFile)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		_, err = volume.NewWatcher(mountPath, r.reloadConfig)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		logger.Log.Debug("Watch config file changes")
	}

	r.cron = cron.New()
	_, err := r.cron.AddFunc("0 0 0 * *", func() {
		if err := r.gc(); err != nil {
			logger.Log.Info("Failed garbage collection", zap.Error(err))
		}
	})
	e, err := r.cron.AddFunc(schedule, func() {
		if err := r.run(); err != nil {
			logger.Log.Info("Failed indexing", zap.Error(err))
		}
	})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	r.entryId = e
	logger.Log.Info("Start scheduler")
	r.cron.Start()

	ctx, stopFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopFunc()

	<-ctx.Done()
	logger.Log.Debug("Got signal")

	ctx = r.cron.Stop()

	logger.Log.Info("Waiting for stop scheduler")
	<-ctx.Done()

	return nil
}

func (r *IndexerCommand) webEndpoint(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/run", func(w http.ResponseWriter, req *http.Request) {
		go func() {
			if err := r.run(); err != nil {
				logger.Log.Info("Failed indexing", zap.Error(err))
			}
		}()
	})

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	logger.Log.Info("Listen web endpoint", zap.String("addr", addr))
	go srv.ListenAndServe()

	return nil
}

func (r *IndexerCommand) gc() error {
	if !r.enableUpload() {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	logger.Log.Info("Start garbage collection")

	s, err := r.newStorageClient()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	gc := NewIndexGC(s, r.Bucket)
	if err := gc.GC(context.Background()); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	return nil
}

func (r *IndexerCommand) enableUpload() bool {
	return r.Bucket != "" &&
		(r.canUseMinIO() || r.canUseS3())
}

func (r *IndexerCommand) canUseMinIO() bool {
	return (r.MinIOName != "" && r.MinIONamespace != "") || r.MinIOEndpoint != ""
}

func (r *IndexerCommand) canUseS3() bool {
	return r.S3Endpoint != "" && r.S3AccessKey != "" && r.S3SecretAccessKey != "" && r.S3Region != ""
}

func (r *IndexerCommand) newStorageClient() (StorageClient, error) {
	if r.canUseMinIO() {
		var opt storage.MinIOOptions
		if r.MinIOName != "" && r.MinIONamespace != "" {
			k8sClient, k8sConf, err := newK8sClient(r.Dev)
			if err != nil {
				return nil, xerrors.Errorf(": %w", err)
			}
			opt = storage.NewMinIOOptionsViaService(k8sClient, k8sConf, r.MinIOName, r.MinIONamespace, r.MinIOPort, r.MinIOAccessKey, r.MinIOSecretAccessKey, r.Dev)
		} else if r.MinIOEndpoint != "" {
			opt = storage.NewMinIOOptionsViaEndpoint(r.MinIOEndpoint, r.MinIOAccessKey, r.MinIOSecretAccessKey)
		}
		return storage.NewMinIOStorage(r.Bucket, opt), nil
	}
	if r.canUseS3() {
		var opt storage.S3Options
		if r.S3Endpoint != "" {
			opt = storage.NewS3OptionToExternal(r.S3Endpoint, r.S3Region, r.S3AccessKey, r.S3SecretAccessKey)
		} else {
			opt = storage.NewS3OptionToAWS(r.S3Region, r.S3AccessKey, r.S3SecretAccessKey)
		}
		opt.PathStyle = true
		return storage.NewS3(r.Bucket, opt), nil
	}

	return nil, nil
}

func (r *IndexerCommand) reloadConfig() {
	logger.Log.Debug("Detect change config file")
	config, err := ReadConfigFile(r.ConfigFile)
	if err != nil {
		logger.Log.Warn("Failed to read a config file", zap.Error(err))
		return
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
		logger.Log.Warn("Failed initialize indexer", zap.Error(err))
		return
	}
	r.indexer = indexer

	if config.RefreshSchedule != r.config.RefreshSchedule {
		r.cron.Remove(r.entryId)
		e, err := r.cron.AddFunc(config.RefreshSchedule, func() {
			if err := r.run(); err != nil {
				logger.Log.Info("Failed indexing", zap.Error(err))
				return
			}
		})
		if err != nil {
			panic(err)
		}
		r.entryId = e
	}

	r.config = config
	if r.cancel != nil {
		r.cancel()
	}
	logger.Log.Info("Reload configuration was finished")

	logger.Log.Info("Build the index with new configuration")
	if err := r.run(); err != nil {
		logger.Log.Warn("Failed to create index", zap.Error(err))
		return
	}
	logger.Log.Info("Finished reload configuration file")
}

func (r *IndexerCommand) uploadIndex(
	ctx context.Context,
	indexer *Indexer,
	bucket string,
	disableCleanup bool,
) (*Manifest, error) {
	s, err := r.newStorageClient()
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	manager := NewObjectStorageIndexManager(s, bucket)
	uploadedPath := make(map[string]string, 0)
	for _, v := range indexer.Indexes {
		uploadDir, err := manager.Add(ctx, v.Name, v.Files)
		if err != nil {
			if err := manager.CleanUploadedFiles(ctx); err != nil {
				logger.Log.Warn("Failed cleanup uploaded files", zap.Error(err))
			}
			return nil, xerrors.Errorf(": %w", err)
		}
		uploadedPath[v.Name] = uploadDir
	}

	manifest := NewManifest(manager.ExecutionKey(), uploadedPath)
	mm := NewManifestManager(s)
	if err := mm.Update(ctx, manifest); err != nil {
		if err := manager.CleanUploadedFiles(ctx); err != nil {
			logger.Log.Warn("Failed cleanup loaded files", zap.Error(err))
		}
		return nil, xerrors.Errorf(": %w", err)
	}

	if !disableCleanup {
		expired, err := mm.FindExpiredManifests(ctx)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		if err := manager.Delete(ctx, expired); err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		for _, m := range expired {
			if err := mm.Delete(ctx, m); err != nil {
				return nil, xerrors.Errorf(": %w", err)
			}
		}
	}

	return &manifest, nil
}
