package sidecar

import (
	"context"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/git"
)

type CloneCommand struct {
	WorkDir        string
	AppId          int64
	InstallationId int64
	PrivateKeyFile string
	Repo           string
	Commit         string
}

func NewCloneCommand() *CloneCommand {
	return &CloneCommand{}
}

func (c *CloneCommand) Name() string {
	return "clone"
}

func (c *CloneCommand) SetFlags(fs *cli.FlagSet) {
	fs.String("work-dir", "Working directory").Shorthand("w").Var(&c.WorkDir)
	fs.Int64("github-app-id", "GitHub App Id").Var(&c.AppId)
	fs.Int64("github-installation-id", "GitHub Installation Id").Var(&c.InstallationId)
	fs.String("private-key-file", "GitHub app private key file").Var(&c.PrivateKeyFile)
	fs.String("url", "Repository url (e.g. git@github.com:octocat/example.git)").Var(&c.Repo)
	fs.String("commit", "Specify commit").Shorthand("b").Var(&c.Commit)
}

func (c *CloneCommand) Run(ctx context.Context) error {
	return git.Clone(ctx, c.AppId, c.InstallationId, c.PrivateKeyFile, c.WorkDir, c.Repo, c.Commit)
}
