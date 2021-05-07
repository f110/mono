package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.f110.dev/mono/go/pkg/cmd/monodev"
)

func monoDev() error {
	rootCmd := &cobra.Command{
		Use:   "monodev",
		Short: "Utilities for development",
	}

	monodev.Cluster(rootCmd)
	monodev.Graph(rootCmd)

	return rootCmd.Execute()
}

func main() {
	if err := monoDev(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
