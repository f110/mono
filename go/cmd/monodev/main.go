package main

import (
	"fmt"
	"os"

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
	return rootCmd.Execute()
}

func main() {
	if err := monoDev(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
