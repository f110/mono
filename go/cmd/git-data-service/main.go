package main

import (
	"context"
	"fmt"
	"os"

	"go.f110.dev/mono/go/cli"
)

func runCommand() error {
	c := newGitDataServiceCommand()
	cmd := &cli.Command{
		Use: "git-data-service",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			if err := c.ValidateFlagValue(); err != nil {
				return err
			}
			return c.LoopContext(ctx)
		},
	}
	c.Flags(cmd.Flags())
	c.GitHubClient.Flags(cmd.Flags())

	return cmd.Execute(os.Args)
}

func main() {
	if err := runCommand(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
