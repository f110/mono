package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/storage"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	"google.golang.org/api/option"

	"go.f110.dev/mono/lib/logger"
)

func unifiBackup(args []string) error {
	var bucket, pathPrefix, credentialFile, backupDir string
	fs := pflag.NewFlagSet("unifibackup", pflag.ContinueOnError)
	fs.StringVar(&backupDir, "dir", "", "Backup file directory")
	fs.StringVar(&bucket, "bucket", "", "Bucket name")
	fs.StringVar(&pathPrefix, "path-prefix", "", "Prefix of file path")
	fs.StringVar(&credentialFile, "credential", "", "Credential file")
	logger.Flags(fs)
	if err := fs.Parse(args); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if err := logger.Init(); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if err := w.Add(backupDir); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	client, err := storage.NewClient(context.Background(), option.WithCredentialsFile(credentialFile))
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	logger.Log.Info("Waiting fs event")
	for event := range w.Events {
		logger.Log.Info("Got event", zap.String("name", event.Name), zap.String("op", event.Op.String()))

		ctx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Second)
		if event.Op&fsnotify.Create == fsnotify.Create {
			path := filepath.Join(pathPrefix, filepath.Base(event.Name))
			obj := client.Bucket(bucket).Object(path)
			if _, err := obj.Attrs(ctx); err == storage.ErrObjectNotExist {
				w := obj.NewWriter(ctx)
				f, err := os.Open(event.Name)
				if err != nil {
					return xerrors.Errorf(": %w", err)
				}
				if _, err := io.Copy(w, f); err != nil {
					return xerrors.Errorf(": %w", err)
				}
				if err := w.Close(); err != nil {
					return xerrors.Errorf(": %w", err)
				}
				if err := f.Close(); err != nil {
					return xerrors.Errorf(": %w", err)
				}

				logger.Log.Info("Succeeded upload", zap.String("object_name", obj.ObjectName()), zap.String("bucket", obj.BucketName()))
			}
		}
		cancelFunc()
	}

	return nil
}

func main() {
	if err := unifiBackup(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
