package main

import (
	"context"
	"fmt"
	"os"

	"go.f110.dev/mono/go/cli"
)

func onep() error {
	rootCmd := &cli.Command{
		Use:   "1p",
		Short: "The CLI for 1Password",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return Main()
		},
	}

	AddCommand(rootCmd)

	return rootCmd.Execute(os.Args)
}

func main() {
	if err := onep(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
