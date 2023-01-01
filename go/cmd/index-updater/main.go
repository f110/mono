package main

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/pkg/codesearch"
	"go.f110.dev/mono/go/pkg/logger"
)

func indexUpdater(args []string) error {
	cmd := codesearch.NewUpdaterCommand()
	fs := pflag.NewFlagSet("index-updater", pflag.ContinueOnError)
	cmd.Flags(fs)
	logger.Flags(fs)
	if err := fs.Parse(args); err != nil {
		return xerrors.WithStack(err)
	}
	if err := logger.Init(); err != nil {
		return xerrors.WithStack(err)
	}

	if err := cmd.Run(); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func main() {
	if err := indexUpdater(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
