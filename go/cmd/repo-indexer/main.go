package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/codesearch"
	"go.f110.dev/mono/go/logger"
)

func repoIndexer(args []string) error {
	indexer := codesearch.NewIndexerCommand()

	cmd := &cobra.Command{
		Use: "repo-indexer",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := logger.Init(); err != nil {
				return xerrors.WithStack(err)
			}
			if err := indexer.Init(); err != nil {
				return xerrors.WithStack(err)
			}

			if err := indexer.Run(); err != nil {
				return xerrors.WithStack(err)
			}

			return nil
		},
	}

	indexer.Flags(cmd.Flags())
	logger.Flags(cmd.Flags())

	cmd.SetArgs(args)
	return cmd.Execute()
}

func main() {
	if err := repoIndexer(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
