package main

import (
	"fmt"
	"os"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/radar/cmd"
)

func radar(args []string) error {
	rootCmd := &cli.Command{
		Use:   "radar",
		Short: "Archive the radar rainfall of multiple sources",
	}
	cmd.AddCommand(rootCmd)

	return rootCmd.Execute(args)
}

func main() {
	if err := radar(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
