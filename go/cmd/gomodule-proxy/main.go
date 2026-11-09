package main

import (
	"context"
	"fmt"
	"os"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/logger"
)

func goModuleProxy() error {
	proxy := newGoModuleProxyCommand()

	cmd := &cli.Command{
		Use: "gomodule-proxy",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			if err := logger.Init(); err != nil {
				return err
			}
			return proxy.LoopContext(ctx)
		},
	}
	proxy.Flags(cmd.Flags())

	return cmd.Execute(os.Args)
}

func main() {
	if err := goModuleProxy(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
