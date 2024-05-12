package main

import (
	"fmt"
	"os"

	"go.f110.dev/mono/go/cli"
)

func monoDev() error {
	rootCmd := &cli.Command{
		Use:   "monodev",
		Short: "Utilities for development",
	}

	CommandManager.Add(rootCmd)

	return rootCmd.Execute(os.Args)
}

func main() {
	if err := monoDev(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
