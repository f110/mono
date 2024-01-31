package main

import (
	"context"
	"fmt"
	"os"

	"go.f110.dev/mono/go/build/cmd/sidecar"
	"go.f110.dev/mono/go/cli"
)

func buildSidecar(args []string) error {
	root := &cli.Command{
		Use: "build-sidecar",
	}

	clone := sidecar.NewCloneCommand()
	cloneCmd := &cli.Command{
		Use: clone.Name(),
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return clone.Run(ctx)
		},
	}
	clone.SetFlags(cloneCmd.Flags())
	root.AddCommand(cloneCmd)

	return root.Execute(args)
}

func main() {
	if err := buildSidecar(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
