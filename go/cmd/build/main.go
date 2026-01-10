package main

import (
	"fmt"
	"os"

	"go.f110.dev/mono/go/build/cmd/bff"
	"go.f110.dev/mono/go/build/cmd/builder"
	"go.f110.dev/mono/go/cli"
)

func main() {
	rootCmd := &cli.Command{
		Use: "build",
	}
	builder.AddCommand(rootCmd)
	bff.AddCommand(rootCmd)

	if err := rootCmd.Execute(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
