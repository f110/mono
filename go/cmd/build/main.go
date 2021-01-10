package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.f110.dev/mono/go/pkg/build/cmd/builder"
	"go.f110.dev/mono/go/pkg/build/cmd/dashboard"
	"go.f110.dev/mono/go/pkg/logger"
)

func main() {
	rootCmd := &cobra.Command{
		Use: "build",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return logger.Init()
		},
	}
	dashboard.AddCommand(rootCmd)
	builder.AddCommand(rootCmd)

	logger.Flags(rootCmd.PersistentFlags())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
