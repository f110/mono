package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"go.f110.dev/mono/go/logger"
)

func goModuleProxy() error {
	proxy := newGoModuleProxyCommand()

	cmd := &cobra.Command{
		Use: "gomodule-proxy",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := logger.Init(); err != nil {
				return err
			}
			return proxy.LoopContext(cmd.Context())
		},
	}
	logger.Flags(cmd.Flags())
	proxy.Flags(cmd.Flags())
	for _, v := range proxy.RequiredFlags() {
		if err := cmd.MarkFlagRequired(v); err != nil {
			return err
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cmd.ExecuteContext(ctx)
}

func main() {
	if err := goModuleProxy(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
