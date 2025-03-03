package codesearch

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

type UpdaterCommand struct {
	IndexDir  string
	Subscribe bool
	HTTPAddr  string

	Bucket                   string
	MinIOEndpoint            string
	MinIORegion              string
	MinIOName                string
	MinIONamespace           string
	MinIOPort                int
	MinIOAccessKey           string
	MinIOSecretAccessKey     string
	MinIOSecretAccessKeyFile string
	S3Endpoint               string
	S3Region                 string
	S3AccessKey              string
	S3SecretAccessKey        string
	S3CACertFile             string

	NATSURL        string
	NATSStreamName string
	NATSSubject    string

	Dev bool

	manifestManager *ManifestManager
	indexManager    *ObjectStorageIndexManager
	latestKey       uint64

	ready bool

	latestKeyDesc *prometheus.Desc
}

var _ prometheus.Collector = &UpdaterCommand{}

func NewUpdaterCommand() *UpdaterCommand {
	return &UpdaterCommand{
		MinIOPort:      9000,
		NATSStreamName: "repoindexer",
		NATSSubject:    "notify",
		latestKeyDesc: prometheus.NewDesc(
			prometheus.BuildFQName("repo_indexer", "", "latest_key"),
			"",
			nil,
			nil,
		),
	}
}

func (u *UpdaterCommand) Flags(fs *pflag.FlagSet) {
	fs.StringVar(&u.IndexDir, "index-dir", u.IndexDir, "Index directory")
	fs.BoolVar(&u.Subscribe, "subscribe", u.Subscribe, "Enable subscribe the stream")
	fs.StringVar(&u.MinIOEndpoint, "minio-endpoint", u.MinIOEndpoint, "The endpoint of MinIO")
	fs.StringVar(&u.MinIORegion, "minio-region", u.MinIORegion, "The region name")
	fs.StringVar(&u.MinIOName, "minio-name", u.MinIOName, "The name of MinIO")
	fs.StringVar(&u.MinIONamespace, "minio-namespace", u.MinIONamespace, "The namespace of MinIO")
	fs.IntVar(&u.MinIOPort, "minio-port", u.MinIOPort, "Port number of MinIO")
	fs.StringVar(&u.MinIOAccessKey, "minio-access-key", u.MinIOAccessKey, "The access key")
	fs.StringVar(&u.MinIOSecretAccessKey, "minio-secret-access-key", u.MinIOSecretAccessKey, "The secret access key")
	fs.StringVar(&u.MinIOSecretAccessKeyFile, "minio-secret-access-key-file", u.MinIOSecretAccessKeyFile, "The file path that contains secret access key for MinIO API")
	fs.StringVar(&u.S3Endpoint, "s3-endpoint", u.S3Endpoint, "The endpoint of s3. If you use the object storage has compatible s3 api not AWS S3, You can use this param")
	fs.StringVar(&u.S3Region, "s3-region", u.S3Region, "The region name")
	fs.StringVar(&u.S3AccessKey, "s3-access-key", u.S3AccessKey, "The access key for S3 API")
	fs.StringVar(&u.S3SecretAccessKey, "s3-secret-access-key", u.S3SecretAccessKey, "The secret access key for S3 API")
	fs.StringVar(&u.S3CACertFile, "s3-ca-file", u.S3CACertFile, "File path that contains the certificate of CA")
	fs.StringVar(&u.Bucket, "bucket", u.Bucket, "The bucket name")
	fs.StringVar(&u.NATSURL, "nats-url", u.NATSURL, "The URL for nats-server")
	fs.StringVar(&u.NATSStreamName, "nats-stream-name", u.NATSStreamName, "The name of stream for JetStream")
	fs.StringVar(&u.NATSSubject, "nats-subject", u.NATSSubject, "The subject of stream")
	fs.BoolVar(&u.Dev, "dev", u.Dev, "Development mode")
	fs.StringVar(&u.HTTPAddr, "http-addr", u.HTTPAddr, "HTTP listen addr")

}

func (u *UpdaterCommand) Run() error {
	s, err := u.newStorageClient()
	if err != nil {
		return xerrors.WithStack(err)
	}
	u.manifestManager = NewManifestManager(s)
	u.indexManager = NewObjectStorageIndexManager(s, u.Bucket)

	ch := make(chan Manifest)
	go u.downloadThread(ch)

	if u.HTTPAddr != "" {
		if err := u.webEndpoint(u.HTTPAddr, ch); err != nil {
			return xerrors.WithStack(err)
		}
	}
	if u.Subscribe {
		if err := u.subscribe(context.Background(), ch); err != nil {
			return xerrors.WithStack(err)
		}
	} else {
		if err := u.downloadLatest(); err != nil {
			return xerrors.WithStack(err)
		}
	}

	return nil
}

func (u *UpdaterCommand) webEndpoint(addr string, ch chan Manifest) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/update", func(w http.ResponseWriter, req *http.Request) {
		go func() {
			manifest, err := u.manifestManager.GetLatest(context.Background())
			if err != nil {
				logger.Log.Warn("Failed to get a latest manifest", zap.Error(err))
				return
			}
			logger.Log.Info("Found manifest", zap.Uint64("key", manifest.ExecutionKey))

			ch <- manifest
		}()
	})
	mux.HandleFunc("/readiness", func(w http.ResponseWriter, req *http.Request) {
		ready := u.ready

		if !ready {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
	})
	prometheus.MustRegister(u)
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	logger.Log.Info("Listen http endpoint", zap.String("addr", addr))
	go srv.ListenAndServe()

	return nil
}

func (u *UpdaterCommand) downloadThread(ch chan Manifest) {
	for {
		select {
		case m := <-ch:
			if err := u.downloadIndex(m); err != nil {
				u.ready = false
				logger.Log.Debug("Failed download an index", zap.Error(err), zap.Uint64("key", m.ExecutionKey))
				continue
			}

			u.ready = true
		}
	}
}

func (u *UpdaterCommand) downloadLatest() error {
	logger.Log.Debug("Download latest the manifest")
	manifest, err := u.manifestManager.GetLatest(context.Background())
	if err != nil {
		return xerrors.WithStack(err)
	}
	logger.Log.Info("Found manifest", zap.Uint64("key", manifest.ExecutionKey))

	if err := u.indexManager.Download(context.Background(), u.IndexDir, manifest); err != nil {
		return xerrors.WithStack(err)
	}

	f, err := os.Create(filepath.Join(u.IndexDir, "manifest.json"))
	if err != nil {
		return xerrors.WithStack(err)
	}
	if err := json.NewEncoder(f).Encode(manifest); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (u *UpdaterCommand) subscribe(ctx context.Context, ch chan Manifest) error {
	n, err := NewNotify(u.NATSURL, u.NATSStreamName, u.NATSSubject)
	if err != nil {
		return xerrors.WithStack(err)
	}
	sub, err := n.Subscribe(u.manifestManager)
	if err != nil {
		return xerrors.WithStack(err)
	}

	u.ready = true
	defer func() {
		u.ready = false
	}()
	logger.Log.Info("Subscribe a stream")
Loop:
	for {
		select {
		case m := <-sub.ch:
			logger.Log.Info("Got notify", zap.Uint64("key", m.ExecutionKey))
			ch <- m
		case <-ctx.Done():
			break Loop
		}
	}
	logger.Log.Info("Stop subscribing a stream")
	return nil
}

func (u *UpdaterCommand) downloadIndex(m Manifest) error {
	if m.ExecutionKey < u.latestKey {
		logger.Log.Debug("Notified manifest is old", zap.Uint64("latest", u.latestKey), zap.Uint64("got", m.ExecutionKey))
		return nil
	}
	if err := u.indexManager.Download(context.Background(), u.IndexDir, m); err != nil {
		return xerrors.WithStack(err)
	}
	u.latestKey = m.ExecutionKey

	f, err := os.Create(filepath.Join(u.IndexDir, "manifest.json"))
	if err != nil {
		return xerrors.WithStack(err)
	}
	if err := json.NewEncoder(f).Encode(m); err != nil {
		return xerrors.WithStack(err)
	}

	u.ready = true
	return nil
}

func (u *UpdaterCommand) newStorageClient() (StorageClient, error) {
	if u.canUseMinIO() {
		secretAccessKey := u.MinIOSecretAccessKey
		if u.MinIOSecretAccessKeyFile != "" {
			b, err := os.ReadFile(u.MinIOSecretAccessKeyFile)
			if err != nil {
				return nil, xerrors.WithStack(err)
			}
			secretAccessKey = strings.TrimSpace(string(b))
		}
		var opt storage.MinIOOptions
		if u.MinIOName != "" && u.MinIONamespace != "" {
			k8sClient, k8sConf, err := newK8sClient(u.Dev)
			if err != nil {
				return nil, xerrors.WithStack(err)
			}
			opt = storage.NewMinIOOptionsViaService(k8sClient, k8sConf, u.MinIOName, u.MinIONamespace, u.MinIOPort, u.MinIOAccessKey, secretAccessKey, u.Dev)
		} else if u.MinIOEndpoint != "" {
			opt = storage.NewMinIOOptionsViaEndpoint(u.MinIOEndpoint, u.MinIORegion, u.MinIOAccessKey, u.MinIOSecretAccessKey)
		}
		opt.Retries = 3
		return storage.NewMinIOStorage(u.Bucket, opt), nil
	}
	if u.canUseS3() {
		var opt storage.S3Options
		if u.S3Endpoint != "" {
			opt = storage.NewS3OptionToExternal(u.S3Endpoint, u.S3Region, u.S3AccessKey, u.S3SecretAccessKey)
		} else {
			opt = storage.NewS3OptionToAWS(u.S3Region, u.S3AccessKey, u.S3SecretAccessKey)
		}
		opt.PathStyle = true
		opt.CACertFile = u.S3CACertFile
		opt.Retries = 3
		return storage.NewS3(u.Bucket, opt), nil
	}

	return nil, nil
}

func (u *UpdaterCommand) canUseMinIO() bool {
	return (u.MinIOName != "" && u.MinIONamespace != "") || u.MinIOEndpoint != ""
}

func (u *UpdaterCommand) canUseS3() bool {
	return u.S3Endpoint != "" && u.S3AccessKey != "" && u.S3SecretAccessKey != "" && u.S3Region != ""
}

func (u *UpdaterCommand) Describe(desc chan<- *prometheus.Desc) {
	desc <- u.latestKeyDesc
}

func (u *UpdaterCommand) Collect(ch chan<- prometheus.Metric) {
	f, err := os.Open(filepath.Join(u.IndexDir, "manifest.json"))
	if err != nil {
		logger.Log.Warn("Could not open the manifest file", zap.Error(err))
		return
	}
	var manifest Manifest
	if err := json.NewDecoder(f).Decode(&manifest); err != nil {
		logger.Log.Warn("Failed to parse the manifest json", zap.Error(err))
		return
	}
	ch <- prometheus.MustNewConstMetric(u.latestKeyDesc, prometheus.GaugeValue, float64(manifest.ExecutionKey))
}
