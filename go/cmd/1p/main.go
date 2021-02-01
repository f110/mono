package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.f110.dev/mono/go/pkg/cmd/onepassword"
	"go.f110.dev/mono/go/pkg/logger"
)

func onep() error {
	rootCmd := &cobra.Command{
		Use:   "1p",
		Short: "The CLI for 1Password",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			logger.Init()
		},
		RunE: func(_ *cobra.Command, args []string) error {
			return onepassword.Main()
		},
	}

	onepassword.AddCommand(rootCmd)

	return rootCmd.Execute()
}

func main() {
	if err := onep(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
