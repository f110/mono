package main

import (
	"context"
	"fmt"
	"os"

	"go.f110.dev/mono/go/cli"
)

func controllerManager(args []string) error {
	c := New(args)
	cmd := &cli.Command{
		Use: "controller-manager",
		Run: func(ctx context.Context, _ *cli.Command, args []string) error {
			return c.LoopContext(ctx)
		},
	}
	c.Flags(cmd.Flags())

	return cmd.Execute(args)
}

func main() {
	if err := controllerManager(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v", err)
		os.Exit(1)
	}
}
