package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/robfig/cron/v3"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/cmd/repoindexer"
	"go.f110.dev/mono/go/pkg/logger"
)

func repoIndexer(args []string) error {
	d, err := os.Getwd()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	configFile := ""
	workDir := d
	token := ""
	ctags := ""
	runScheduler := false
	fs := pflag.NewFlagSet("repo-indexer", pflag.ContinueOnError)
	fs.StringVarP(&configFile, "config", "c", configFile, "Config file")
	fs.StringVar(&workDir, "work-dir", workDir, "Working root directory")
	fs.StringVar(&token, "token", token, "GitHub API token")
	fs.StringVar(&ctags, "ctags", ctags, "ctags path")
	fs.BoolVar(&runScheduler, "run-scheduler", false, "")
	logger.Flags(fs)
	if err := fs.Parse(args); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	logger.Init()

	config, err := repoindexer.ReadConfigFile(configFile)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	indexer := repoindexer.NewIndexer(config, workDir, token, ctags)
	if err := indexer.Sync(); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if err := indexer.BuildIndex(); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if runScheduler {
		c := cron.New()
		_, err := c.AddFunc(config.RefreshSchedule, func() {
			if err := indexer.Sync(); err != nil {
				logger.Log.Info("Failed sync", zap.Error(err))
			}
			if err := indexer.BuildIndex(); err != nil {
				logger.Log.Info("Failed build index", zap.Error(err))
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
	}

	return nil
}

func main() {
	if err := repoIndexer(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
