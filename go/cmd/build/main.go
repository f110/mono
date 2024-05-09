package main

import (
	"fmt"
	"os"

	"go.f110.dev/mono/go/build/cmd/builder"
	"go.f110.dev/mono/go/build/cmd/dashboard"
	"go.f110.dev/mono/go/cli"
)

func main() {
	rootCmd := &cli.Command{
		Use: "build",
	}
	dashboard.AddCommand(rootCmd)
	builder.AddCommand(rootCmd)

	if err := rootCmd.Execute(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
