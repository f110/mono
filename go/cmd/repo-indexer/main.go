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
	d, err := os.Getwd()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	configFile := ""
	workDir := d
	token := ""
	fs := pflag.NewFlagSet("repo-indexer", pflag.ContinueOnError)
	fs.StringVarP(&configFile, "config", "c", configFile, "Config file")
	fs.StringVar(&workDir, "work-dir", workDir, "Working root directory")
	fs.StringVar(&token, "token", token, "GitHub API token")
	logger.Flags(fs)
	if err := fs.Parse(args); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	logger.Init()

	config, err := repoindexer.ReadConfigFile(configFile)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	indexer := repoindexer.NewIndexer(config, workDir, token)
	if err := indexer.Sync(); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if err := indexer.BuildIndex(); err != nil {
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
