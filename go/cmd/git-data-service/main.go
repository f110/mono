package main

import (
	"context"
	"fmt"
	"os"

	"go.f110.dev/mono/go/cli"
)

func runCommand() error {
	rootCmd := &cli.Command{
		Use: "git-data-service",
	}

	s := newGitDataServiceCommand()
	serverCmd := &cli.Command{
		Use:   "server",
		Short: "Serve the repositories via gRPC",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			if err := s.ValidateFlagValue(); err != nil {
				return err
			}
			return s.LoopContext(ctx)
		},
	}
	s.Flags(serverCmd.Flags())
	rootCmd.AddCommand(serverCmd)

	u := newUpdaterCommand()
	updaterCmd := &cli.Command{
		Use:   "updater",
		Short: "Update the repositories periodically",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			if err := u.ValidateFlagValue(); err != nil {
				return err
			}
			return u.LoopContext(ctx)
		},
	}
	u.Flags(updaterCmd.Flags())
	rootCmd.AddCommand(updaterCmd)

	return rootCmd.Execute(os.Args)
}

func main() {
	if err := runCommand(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
