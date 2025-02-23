package codesearch

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/robfig/cron/v3"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/githubutil"
	"go.f110.dev/mono/go/k8s/volume"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

type IndexerCommand struct {
	ConfigFile       string
	WorkDir          string
	Ctags            string
	RunScheduler     bool
	InitRun          bool
	WithoutFetch     bool
	DisableCleanup   bool
	Parallelism      int
	HTTPAddr         string
	URLReplaceRegexp []string
	CABundleFile     string

	Bucket                      string
	MinIOEndpoint               string
	MinIORegion                 string
	MinIOName                   string
	MinIONamespace              string
	MinIOPort                   int
	MinIOAccessKey              string
	MinIOSecretAccessKey        string
	MinIOSecretAccessKeyFile    string
	S3Endpoint                  string
	S3Region                    string
	S3AccessKey                 string
	S3SecretAccessKey           string
	S3CACertFile                string
	S3PartSize                  uint64
	DisableObjectStorageCleanup bool

	NATSURL        string
	NATSStreamName string
	NATSSubject    string

	Dev bool

	githubClientFactory *githubutil.GitHubClientFactory
	config              *Config
	indexer             *Indexer
	cron                *cron.Cron
	entryId             cron.EntryID
	cancel              context.CancelFunc
	caBundle            []byte
	natsNotify          *Notify

	mu sync.Mutex

	readyMu sync.Mutex
	ready   bool
}

func NewIndexerCommand() *IndexerCommand {
	d, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	return &IndexerCommand{
		WorkDir:             d,
		MinIOPort:           9000,
		Parallelism:         1,
		NATSStreamName:      "repoindexer",
		NATSSubject:         "notify",
		S3PartSize:          1 * 1024 * 1024 * 1024, // 1GiB
		githubClientFactory: githubutil.NewGitHubClientFactory("repo-indexer", false),
	}
}

func (r *IndexerCommand) Flags(fs *cli.FlagSet) {
	fs.String("config", "Config file").Var(&r.ConfigFile).Shorthand("c")
	fs.String("work-dir", "Working root directory").Var(&r.WorkDir).Default(r.WorkDir)
	fs.String("ctags", "ctags path").Var(&r.Ctags)
	fs.Bool("run-scheduler", "").Var(&r.RunScheduler)
	fs.Bool("init-run", "").Var(&r.InitRun)
	fs.Bool("without-fetch", "Disable fetch").Var(&r.WithoutFetch)
	fs.Bool("disable-cleanup", "Disable cleanup in the working directory not the object storage").Var(&r.DisableCleanup)
	fs.Int("parallelism", "The number of workers").Var(&r.Parallelism).Default(r.Parallelism)
	fs.String("minio-endpoint", "The endpoint of MinIO").Var(&r.MinIOEndpoint)
	fs.String("minio-region", "The region name").Var(&r.MinIORegion)
	fs.String("minio-name", "The name of MinIO").Var(&r.MinIOName)
	fs.String("minio-namespace", "The namespace of MinIO").Var(&r.MinIONamespace)
	fs.Int("minio-port", "Port number of MinIO").Var(&r.MinIOPort)
	fs.String("minio-bucket", "The bucket name that will be used").Var(&r.Bucket).Deprecated("Use --bucket instead")
	fs.String("minio-access-key", "The access key for MinIO API").Var(&r.MinIOAccessKey)
	fs.String("minio-secret-access-key", "The secret access key for MinIO API").Var(&r.MinIOSecretAccessKey)
	fs.String("minio-secret-access-key-file", "The file path that contains secret access key for MinIO API").Var(&r.MinIOSecretAccessKeyFile)
	fs.String("s3-endpoint", "The endpoint of s3. If you use the object storage has compatible s3 api not AWS S3, You can use this param").Var(&r.S3Endpoint)
	fs.String("s3-region", "The region name").Var(&r.S3Region)
	fs.String("s3-access-key", "The access key for S3 API").Var(&r.S3AccessKey)
	fs.String("s3-secret-access-key", "The secret access key for S3 API").Var(&r.S3SecretAccessKey)
	fs.String("s3-ca-file", "File path that contains the certificate of CA").Var(&r.S3CACertFile)
	fs.Uint64("s3-part-size", "Part size").Var(&r.S3PartSize)
	fs.String("bucket", "The bucket name").Var(&r.Bucket)
	fs.String("nats-url", "The URL for nats-server").Var(&r.NATSURL)
	fs.String("nats-stream-name", "The name of stream for JetStream").Var(&r.NATSStreamName).Default(r.NATSStreamName)
	fs.String("nats-subject", "The subject of stream").Var(&r.NATSSubject).Default(r.NATSSubject)
	fs.Bool("disable-object-storage-cleanup", "Disable cleanup in the object storage after uploaded the index").Var(&r.DisableObjectStorageCleanup)
	fs.String("ca-bundle-file", "A file path that contains ca certificates for clone a repository").Var(&r.CABundleFile)
	fs.Bool("dev", "Development mode").Var(&r.Dev)
	fs.String("http-addr", "HTTP listen addr").Var(&r.HTTPAddr)

	r.githubClientFactory.Flags(fs)
}

func (r *IndexerCommand) ValidateFlags() error {
	if r.ConfigFile == "" {
		return xerrors.Define("--config is required").WithStack()
	}

	if r.MinIOEndpoint != "" {
		if r.MinIOName != "" || r.MinIONamespace != "" {
			return xerrors.Define("--minio-endpoint and --minio-name both specified").WithStack()
		}
	}

	return nil
}

func (r *IndexerCommand) Init() error {
	if err := r.ValidateFlags(); err != nil {
		return xerrors.WithStack(err)
	}

	if r.CABundleFile != "" {
		b, err := os.ReadFile(r.CABundleFile)
		if err != nil {
			return xerrors.WithStack(err)
		}
		r.caBundle = b
	}

	if err := r.githubClientFactory.Init(); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (r *IndexerCommand) Run() error {
	config, err := ReadConfigFile(r.ConfigFile)
	if err != nil {
		return xerrors.WithStack(err)
	}
	r.config = config

	indexer := NewIndexer(
		config,
		r.WorkDir,
		r.Ctags,
		r.githubClientFactory.REST,
		r.githubClientFactory.GraphQL,
		r.githubClientFactory.TokenProvider,
		r.InitRun,
		r.Parallelism,
		r.caBundle,
	)
	r.indexer = indexer
	if r.HTTPAddr != "" {
		if err := r.webEndpoint(r.HTTPAddr); err != nil {
			return xerrors.WithStack(err)
		}
	}
	if r.enableUpload() && r.NATSURL != "" {
		logger.Log.Debug("Start notifier")
		n, err := NewNotify(r.NATSURL, r.NATSStreamName, r.NATSSubject)
		if err != nil {
			return xerrors.WithStack(err)
		}
		r.natsNotify = n
	}

	if r.RunScheduler {
		if err := r.scheduler(config.RefreshSchedule); err != nil {
			return xerrors.WithStack(err)
		}
		return nil
	}
	if err := r.run(); err != nil {
		return xerrors.WithStack(err)
	}
	if err := r.gc(); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (r *IndexerCommand) run() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ctx, cancel := ctxutil.WithCancel(context.Background())
	defer func() {
		r.indexer.Reset()
		cancel()
	}()
	r.cancel = cancel

	if !r.WithoutFetch {
		if err := r.indexer.Sync(ctx); err != nil {
			return err
		}
	}
	if err := r.indexer.BuildIndex(ctx); err != nil {
		return err
	}
	if r.enableUpload() {
		manifest, err := r.uploadIndex(ctx, r.indexer, r.Bucket, r.DisableObjectStorageCleanup)
		if err != nil {
			return err
		}

		if r.natsNotify != nil {
			if err := r.natsNotify.Notify(ctx, manifest); err != nil {
				return err
			}
		}
	} else {
		logger.Log.Debug("Disable upload", zap.Bool("can_use_minio", r.canUseMinIO()), zap.Bool("can_use_s3", r.canUseS3()))
	}
	if !r.DisableCleanup {
		if err := r.indexer.Cleanup(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (r *IndexerCommand) scheduler(schedule string) error {
	if volume.CanWatchVolume(r.ConfigFile) {
		mountPath, err := volume.FindMountPath(r.ConfigFile)
		if err != nil {
			return xerrors.WithStack(err)
		}
		_, err = volume.NewWatcher(mountPath, r.reloadConfig)
		if err != nil {
			return xerrors.WithStack(err)
		}
		logger.Log.Debug("Watch config file changes")
	}

	r.cron = cron.New()
	_, err := r.cron.AddFunc("0 0 * * *", func() {
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
		return xerrors.WithStack(err)
	}
	r.entryId = e
	logger.Log.Info("Start scheduler")
	r.cron.Start()

	r.readyMu.Lock()
	r.ready = true
	r.readyMu.Unlock()
	defer func() {
		r.readyMu.Lock()
		r.ready = false
		r.readyMu.Unlock()
	}()
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
	mux.HandleFunc("/gc", func(w http.ResponseWriter, req *http.Request) {
		go func() {
			if err := r.gc(); err != nil {
				logger.Log.Info("Failed to garbage collection", logger.Error(err), logger.StackTrace(err))
			}
		}()
	})
	mux.HandleFunc("/readiness", func(w http.ResponseWriter, req *http.Request) {
		r.readyMu.Lock()
		ready := r.ready
		r.readyMu.Unlock()

		if !ready {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
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
		return err
	}
	gc := NewIndexGC(s, r.Bucket)
	if err := gc.GC(context.Background()); err != nil {
		return err
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
		secretAccessKey := r.MinIOSecretAccessKey
		if r.MinIOSecretAccessKeyFile != "" {
			b, err := os.ReadFile(r.MinIOSecretAccessKeyFile)
			if err != nil {
				return nil, xerrors.WithStack(err)
			}
			secretAccessKey = strings.TrimSpace(string(b))
		}
		var opt storage.MinIOOptions
		if r.MinIOName != "" && r.MinIONamespace != "" {
			k8sClient, k8sConf, err := newK8sClient(r.Dev)
			if err != nil {
				return nil, xerrors.WithStack(err)
			}
			opt = storage.NewMinIOOptionsViaService(k8sClient, k8sConf, r.MinIOName, r.MinIONamespace, r.MinIOPort, r.MinIOAccessKey, secretAccessKey, r.Dev)
		} else if r.MinIOEndpoint != "" {
			opt = storage.NewMinIOOptionsViaEndpoint(r.MinIOEndpoint, r.MinIORegion, r.MinIOAccessKey, secretAccessKey)
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
		opt.CACertFile = r.S3CACertFile
		opt.PartSize = r.S3PartSize
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
	indexer := NewIndexer(
		config,
		r.WorkDir,
		r.Ctags,
		r.githubClientFactory.REST,
		r.githubClientFactory.GraphQL,
		r.githubClientFactory.TokenProvider,
		r.InitRun,
		r.Parallelism,
		r.caBundle,
	)
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
		return nil, xerrors.WithStack(err)
	}
	manager := NewObjectStorageIndexManager(s, bucket)
	uploadedPath := make(map[string]string, 0)
	totalSize := uint64(0)
	for _, v := range indexer.Indexes {
		uploadDir, size, err := manager.Add(ctx, v.Name, v.Files)
		if err != nil {
			if err := manager.CleanUploadedFiles(ctx); err != nil {
				logger.Log.Warn("Failed cleanup uploaded files", zap.Error(err))
			}
			return nil, xerrors.WithStack(err)
		}
		uploadedPath[v.Name] = uploadDir
		totalSize += size
	}

	manifest := NewManifest(manager.ExecutionKey(), uploadedPath, totalSize)
	mm := NewManifestManager(s)
	if err := mm.Update(ctx, manifest); err != nil {
		if err := manager.CleanUploadedFiles(ctx); err != nil {
			logger.Log.Warn("Failed cleanup loaded files", zap.Error(err))
		}
		return nil, xerrors.WithStack(err)
	}

	if !disableCleanup {
		expired, err := mm.FindExpiredManifests(ctx)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		if err := manager.Delete(ctx, expired); err != nil {
			return nil, xerrors.WithStack(err)
		}
		for _, m := range expired {
			if err := mm.Delete(ctx, m); err != nil {
				return nil, xerrors.WithStack(err)
			}
		}
	}

	return &manifest, nil
}
