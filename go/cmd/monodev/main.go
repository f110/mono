package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/pkg/cmd/monodev"
	"go.f110.dev/mono/go/pkg/logger"
)

func monoDev() error {
	rootCmd := &cobra.Command{
		Use:   "monodev",
		Short: "Utilities for development",
	}

	monodev.CommandManager.Add(rootCmd)

	logger.Flags(rootCmd.Flags())
	if err := logger.Init(); err != nil {
		return xerrors.WithStack(err)
	}
	
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	return rootCmd.ExecuteContext(ctx)
}

func main() {
	if err := monoDev(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
