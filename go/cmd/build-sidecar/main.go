package main

import (
	"context"
	"fmt"
	"os"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/git"
)

type cloneCommand struct {
	WorkDir        string
	AppId          int64
	InstallationId int64
	PrivateKeyFile string
	Repo           string
	Commit         string
}

func newCloneCommand() *cloneCommand {
	return &cloneCommand{}
}

func (c *cloneCommand) SetFlags(fs *cli.FlagSet) {
	fs.String("work-dir", "Working directory").Shorthand("w").Var(&c.WorkDir)
	fs.Int64("github-app-id", "GitHub App Id").Var(&c.AppId)
	fs.Int64("github-installation-id", "GitHub Installation Id").Var(&c.InstallationId)
	fs.String("private-key-file", "GitHub app private key file").Var(&c.PrivateKeyFile)
	fs.String("url", "Repository url (e.g. git@github.com:octocat/example.git)").Var(&c.Repo)
	fs.String("commit", "Specify commit").Shorthand("b").Var(&c.Commit)
}

func (c *cloneCommand) Run(ctx context.Context) error {
	return git.Clone(ctx, c.AppId, c.InstallationId, c.PrivateKeyFile, c.WorkDir, c.Repo, c.Commit)
}

func buildSidecar(args []string) error {
	root := &cli.Command{
		Use: "build-sidecar",
	}

	clone := newCloneCommand()
	cloneCmd := &cli.Command{
		Use: "clone",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return clone.Run(ctx)
		},
	}
	clone.SetFlags(cloneCmd.Flags())
	root.AddCommand(cloneCmd)

	return root.Execute(args)
}

func main() {
	if err := buildSidecar(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
