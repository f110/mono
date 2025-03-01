package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/notion"
)

type githubTaskCommand struct {
	notionToken     string
	notionTokenFile string
	githubToken     string
	appId           int64
	installationId  int64
	privateKeyFile  string
	configFile      string
	schedule        string
	oneshot         bool
}

func newGitHubTaskCommand() *githubTaskCommand {
	return &githubTaskCommand{
		schedule: "0 * * * *",
	}
}

func (g *githubTaskCommand) Flags(fs *cli.FlagSet) {
	fs.String("notion-token", "API token for Notion").Var(&g.notionToken)
	fs.String("notion-token-file", "The file path that contains API token for notion").Var(&g.notionTokenFile)
	fs.String("github-token", "Personal access token of GitHub").Var(&g.githubToken)
	fs.Int64("github-app-id", "GitHub App Id").Var(&g.appId)
	fs.Int64("github-installation-id", "GitHub App installation Id").Var(&g.installationId)
	fs.String("github-private-key-file", "Private key file").Var(&g.privateKeyFile)
	fs.String("config-file", "Config file path").Var(&g.configFile).Required()
	fs.String("schedule", "Check schedule").Var(&g.schedule).Default("0 * * * *")
	fs.Bool("oneshot", "Oneshot execution").Var(&g.oneshot)
}

func (g *githubTaskCommand) Execute(ctx context.Context) error {
	if g.githubToken == "" && os.Getenv("GITHUB_TOKEN") != "" {
		g.githubToken = os.Getenv("GITHUB_TOKEN")
	}
	if g.githubToken == "" && !(g.appId > 0 && g.installationId > 0 && g.privateKeyFile != "") {
		return xerrors.New("personal access token or GitHub App is required")
	}
	if g.notionToken == "" && os.Getenv("NOTION_TOKEN") != "" {
		g.notionToken = os.Getenv("NOTION_TOKEN")
	}
	if g.notionTokenFile != "" {
		b, err := os.ReadFile(g.notionTokenFile)
		if err != nil {
			return xerrors.WithStack(err)
		}
		g.notionToken = strings.TrimSpace(string(b))
	}
	if g.notionToken == "" {
		return xerrors.New("--notion-token, --notion-token-file or NOTION_TOKEN is required")
	}

	var ghTask *notion.GitHubTask
	if g.githubToken != "" {
		t, err := notion.NewGitHubTaskWithToken(g.githubToken, g.notionToken, g.configFile)
		if err != nil {
			return xerrors.WithStack(err)
		}
		ghTask = t
	} else {
		t, err := notion.NewGitHubTask(g.appId, g.installationId, g.privateKeyFile, g.notionToken, g.configFile)
		if err != nil {
			return xerrors.WithStack(err)
		}
		ghTask = t
	}

	if g.oneshot {
		if err := ghTask.Execute(); err != nil {
			return xerrors.WithStack(err)
		}
		return nil
	}

	go ghTask.Start(g.schedule)

	<-ctx.Done()

	return nil
}

func notionGitHubTask(args []string) error {
	githubTask := newGitHubTaskCommand()
	cmd := &cli.Command{
		Use: "notion-github-task",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return githubTask.Execute(ctx)
		},
	}
	githubTask.Flags(cmd.Flags())

	return cmd.Execute(args)
}

func main() {
	if err := notionGitHubTask(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
