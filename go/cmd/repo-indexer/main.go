package main

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/cmd/repoindexer"
	"go.f110.dev/mono/go/pkg/logger"
)

func repoIndexer(args []string) error {
	dev := false
	opt := repoindexer.NewRunOption()
	fs := pflag.NewFlagSet("repo-indexer", pflag.ContinueOnError)
	fs.StringVarP(&opt.ConfigFile, "config", "c", opt.ConfigFile, "Config file")
	fs.StringVar(&opt.WorkDir, "work-dir", opt.WorkDir, "Working root directory")
	fs.StringVar(&opt.Token, "token", opt.Token, "GitHub API token")
	fs.StringVar(&opt.Ctags, "ctags", opt.Ctags, "ctags path")
	fs.BoolVar(&opt.RunScheduler, "run-scheduler", opt.RunScheduler, "")
	fs.BoolVar(&opt.InitRun, "init-run", opt.InitRun, "")
	fs.BoolVar(&opt.WithoutFetch, "without-fetch", opt.WithoutFetch, "Disable fetch")
	fs.BoolVar(&opt.DisableCleanup, "disable-cleanup", opt.DisableCleanup, "Disable cleanup")
	fs.IntVar(&opt.Parallelism, "parallelism", opt.Parallelism, "The number of workers")
	fs.Int64Var(&opt.AppId, "app-id", opt.AppId, "GitHub Application ID")
	fs.Int64Var(&opt.InstallationId, "installation-id", opt.InstallationId, "GitHub Application installation ID")
	fs.StringVar(&opt.PrivateKeyFile, "private-key-file", opt.PrivateKeyFile, "Private key file for GitHub App")
	fs.StringVar(&opt.MinIOName, "minio-name", opt.MinIOName, "The name of MinIO")
	fs.StringVar(&opt.MinIONamespace, "minio-namespace", opt.MinIONamespace, "The namespace of MinIO")
	fs.IntVar(&opt.MinIOPort, "minio-port", opt.MinIOPort, "Port number of MinIO")
	fs.StringVar(&opt.MinIOBucket, "minio-bucket", opt.MinIOBucket, "The bucket name that will be used")
	fs.StringVar(&opt.MinIOAccessKey, "minio-access-key", opt.MinIOAccessKey, "The access key")
	fs.StringVar(&opt.MinIOSecretAccessKey, "minio-secret-access-key", opt.MinIOSecretAccessKey, "The secret access key")
	fs.BoolVar(&dev, "dev", dev, "Development mode")
	logger.Flags(fs)
	if err := fs.Parse(args); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if err := logger.Init(); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	err := repoindexer.RepoIndexer(opt, dev)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func main() {
	if err := repoIndexer(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
