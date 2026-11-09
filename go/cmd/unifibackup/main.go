package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"cloud.google.com/go/storage"
	"github.com/fsnotify/fsnotify"
	"go.f110.dev/xerrors"
	"google.golang.org/api/option"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/logger"
)

type backupMeta struct {
	Version  string    `json:"version"`
	Time     int64     `json:"time"`
	DateTime time.Time `json:"datetime"`
	Format   string    `json:"format"`
	Days     int       `json:"days"`
	Size     int32     `json:"size"`

	Filename string `json:"-"`
}

func parseBackupMeta(file string) (map[string]*backupMeta, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	meta := make(map[string]*backupMeta)
	if err := json.NewDecoder(f).Decode(&meta); err != nil {
		return nil, xerrors.WithStack(err)
	}
	return meta, nil
}

func selectLatestBackup(m map[string]*backupMeta) string {
	meta := make([]*backupMeta, 0, len(m))
	for filename, v := range m {
		v.Filename = filename
		meta = append(meta, v)
	}
	sort.Slice(meta, func(i, j int) bool {
		return meta[i].DateTime.After(meta[j].DateTime)
	})

	return meta[0].Filename
}

func unifiBackup(backupDir, bucket, pathPrefix, credentialFile string) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return xerrors.WithStack(err)
	}
	if err := w.Add(backupDir); err != nil {
		return xerrors.WithStack(err)
	}

	client, err := storage.NewClient(context.Background(), option.WithCredentialsFile(credentialFile))
	if err != nil {
		return xerrors.WithStack(err)
	}

	logger.Log.Info("Waiting fs event")
	for event := range w.Events {
		logger.Log.Info("Got event", slog.String("name", event.Name), slog.String("op", event.Op.String()))

		if event.Op&fsnotify.Write == fsnotify.Write && filepath.Base(event.Name) == "autobackup_meta.json" {
			m, err := parseBackupMeta(event.Name)
			if err != nil {
				logger.Log.Info("Failed parse metadata file", logger.E(err))
				continue
			}
			latestBackup := selectLatestBackup(m)

			ctx, cancelFunc := ctxutil.WithTimeout(context.Background(), 10*time.Second)
			path := filepath.Join(pathPrefix, latestBackup)
			obj := client.Bucket(bucket).Object(path)
			if _, err := obj.Attrs(ctx); err == storage.ErrObjectNotExist {
				w := obj.NewWriter(ctx)
				f, err := os.Open(filepath.Join(filepath.Dir(event.Name), latestBackup))
				if err != nil {
					return xerrors.WithStack(err)
				}
				if _, err := io.Copy(w, f); err != nil {
					return xerrors.WithStack(err)
				}
				if err := w.Close(); err != nil {
					return xerrors.WithStack(err)
				}
				if err := f.Close(); err != nil {
					return xerrors.WithStack(err)
				}

				logger.Log.Info("Succeeded upload", slog.String("object_name", obj.ObjectName()), slog.String("bucket", obj.BucketName()))
			}
			cancelFunc()
		}
	}

	return nil
}

func main() {
	var bucket, pathPrefix, credentialFile, backupDir string
	cmd := &cli.Command{
		Use: "unifibackup",
		Run: func(_ context.Context, _ *cli.Command, _ []string) error {
			return unifiBackup(backupDir, bucket, pathPrefix, credentialFile)
		},
	}
	fs := cmd.Flags()
	fs.String("dir", "Backup file directory").Var(&backupDir)
	fs.String("bucket", "Bucket name").Var(&bucket)
	fs.String("path-prefix", "Prefix of file path").Var(&pathPrefix)
	fs.String("credential", "Credential file").Var(&credentialFile)
	if err := cmd.Execute(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
