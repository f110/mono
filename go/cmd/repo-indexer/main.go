package main

import (
	"context"
	"fmt"
	"os"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/codesearch"
)

func repoIndexer(args []string) error {
	indexer := codesearch.NewIndexerCommand()

	cmd := &cli.Command{
		Use: "repo-indexer",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
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

	return cmd.Execute(args)
}

func main() {
	if err := repoIndexer(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
