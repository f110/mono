package main

import (
	"fmt"
	"os"

	"go.f110.dev/mono/go/build/cmd/buildctl"
	"go.f110.dev/mono/go/cli"
)

func buildCtl(args []string) error {
	rootCmd := &cli.Command{
		Use: "buildctl",
	}
	buildctl.Job(rootCmd)

	return rootCmd.Execute(args)
}

func main() {
	if err := buildCtl(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
