package main

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"go.f110.dev/mono/go/pkg/git"
	"golang.org/x/xerrors"
)

const (
	ActionClone = "clone"
)

func buildSidecar(args []string) error {
	action := ""
	repo := ""
	appId := int64(0)
	installationId := int64(0)
	privateKeyFile := ""
	commit := ""
	workingDir := ""
	fs := pflag.NewFlagSet("build-sidecar", pflag.ContinueOnError)
	fs.StringVarP(&action, "action", "a", action, "Action")
	fs.StringVarP(&workingDir, "work-dir", "w", workingDir, "Working directory")
	fs.Int64Var(&appId, "github-app-id", appId, "GitHub App Id")
	fs.Int64Var(&installationId, "github-installation-id", installationId, "GitHub Installation Id")
	fs.StringVar(&privateKeyFile, "private-key-file", privateKeyFile, "GitHub app private key file")
	fs.StringVar(&repo, "url", repo, "Repository url (e.g. git@github.com:octocat/example.git)")
	fs.StringVarP(&commit, "commit", "b", "", "Specify commit")
	if err := fs.Parse(args); err != nil {
		return xerrors.Errorf(": %v", err)
	}

	switch action {
	case ActionClone:
		return git.Clone(appId, installationId, privateKeyFile, workingDir, repo, commit)
	default:
		return xerrors.Errorf("unknown action: %v", action)
	}
}

func main() {
	if err := buildSidecar(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
