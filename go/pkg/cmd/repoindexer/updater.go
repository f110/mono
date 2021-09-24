package repoindexer

import (
	"context"

	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/storage"
)

type UpdaterCommand struct {
	IndexDir  string
	Subscribe bool

	MinIOName            string
	MinIONamespace       string
	MinIOPort            int
	MinIOBucket          string
	MinIOAccessKey       string
	MinIOSecretAccessKey string

	NATSURL        string
	NATSStreamName string
	NATSSubject    string

	Dev bool

	manifestManager *ManifestManager
	indexManager    *ObjectStorageIndexManager
	latestKey       uint64
}

func NewUpdaterCommand() *UpdaterCommand {
	return &UpdaterCommand{
		MinIOPort:      9000,
		NATSStreamName: "repoindexer",
		NATSSubject:    "notify",
	}
}

func (u *UpdaterCommand) Flags(fs *pflag.FlagSet) {
	fs.StringVar(&u.IndexDir, "index-dir", u.IndexDir, "Index directory")
	fs.BoolVar(&u.Subscribe, "subscribe", u.Subscribe, "Enable subscribe the stream")
	fs.StringVar(&u.MinIOName, "minio-name", u.MinIOName, "The name of MinIO")
	fs.StringVar(&u.MinIONamespace, "minio-namespace", u.MinIONamespace, "The namespace of MinIO")
	fs.IntVar(&u.MinIOPort, "minio-port", u.MinIOPort, "Port number of MinIO")
	fs.StringVar(&u.MinIOBucket, "minio-bucket", u.MinIOBucket, "The bucket name that will be used")
	fs.StringVar(&u.MinIOAccessKey, "minio-access-key", u.MinIOAccessKey, "The access key")
	fs.StringVar(&u.MinIOSecretAccessKey, "minio-secret-access-key", u.MinIOSecretAccessKey, "The secret access key")
	fs.StringVar(&u.NATSURL, "nats-url", u.NATSURL, "The URL for nats-server")
	fs.StringVar(&u.NATSStreamName, "nats-stream-name", u.NATSStreamName, "The name of stream for JetStream")
	fs.StringVar(&u.NATSSubject, "nats-subject", u.NATSSubject, "The subject of stream")
	fs.BoolVar(&u.Dev, "dev", u.Dev, "Development mode")
}

func (u *UpdaterCommand) Run() error {
	k8sClient, k8sConf, err := newK8sClient(u.Dev)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	minioOpt := storage.NewMinIOOptions(u.MinIOName, u.MinIONamespace, u.MinIOPort, u.MinIOBucket, u.MinIOAccessKey, u.MinIOSecretAccessKey)
	s := storage.NewMinIOStorage(k8sClient, k8sConf, minioOpt, u.Dev)
	u.manifestManager = NewManifestManager(s)
	u.indexManager = NewObjectStorageIndexManager(s, u.MinIOBucket)

	if u.Subscribe {
		if err := u.subscribe(context.Background()); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	} else {
		if err := u.downloadLatest(); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func (u *UpdaterCommand) downloadLatest() error {
	logger.Log.Debug("Download latest the manifest")
	manifest, err := u.manifestManager.GetLatest(context.Background())
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	logger.Log.Info("Found manifest", zap.Uint64("key", manifest.ExecutionKey))

	if err := u.indexManager.Download(context.Background(), u.IndexDir, manifest); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (u *UpdaterCommand) subscribe(ctx context.Context) error {
	if err := u.downloadLatest(); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	n, err := NewNotify(u.NATSURL, u.NATSStreamName, u.NATSSubject)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	sub, err := n.Subscribe(u.manifestManager)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	logger.Log.Info("Subscribe stream")
Loop:
	for {
		select {
		case m := <-sub.ch:
			logger.Log.Info("Got notify", zap.Uint64("key", m.ExecutionKey))
			if m.ExecutionKey < u.latestKey {
				logger.Log.Debug("Notified manifest is old", zap.Uint64("latest", u.latestKey), zap.Uint64("got", m.ExecutionKey))
				continue
			}
			if err := u.indexManager.Download(context.Background(), u.IndexDir, m); err != nil {
				logger.Log.Warn("Failed download an index", zap.Error(err), zap.Uint64("key", m.ExecutionKey))
			}
			u.latestKey = m.ExecutionKey
		case <-ctx.Done():
			break Loop
		}
	}
	return nil
}
