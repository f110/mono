package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/tools/build/pkg/cmd/buildctl"
)

func buildCtl(args []string) error {
	rootCmd := &cobra.Command{
		Use: "buildctl",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return logger.Init()
		},
	}
	buildctl.Job(rootCmd)

	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

func main() {
	if err := buildCtl(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
