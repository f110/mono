package main

import (
	"context"
	"fmt"
	"os"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/codesearch"
)

func indexUpdater() error {
	u := codesearch.NewUpdaterCommand()
	cmd := &cli.Command{
		Use: "index-updater",
		Run: func(_ context.Context, _ *cli.Command, _ []string) error {
			if err := u.Run(); err != nil {
				return xerrors.WithStack(err)
			}
			return nil
		},
	}
	u.Flags(cmd.Flags())
	return cmd.Execute(os.Args)
}

func main() {
	if err := indexUpdater(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
