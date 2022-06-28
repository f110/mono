package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"go.f110.dev/mono/go/pkg/logger"
)

func runCommand() error {
	c := newGitDataServiceCommand()
	cmd := &cobra.Command{
		Use: "git-data-service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logger.Init(); err != nil {
				return err
			}
			if err := c.ValidateFlagValue(); err != nil {
				return err
			}
			return c.LoopContext(cmd.Context())
		},
	}
	c.Flags(cmd.Flags())
	logger.Flags(cmd.Flags())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cmd.ExecuteContext(ctx)
}

func main() {
	if err := runCommand(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
