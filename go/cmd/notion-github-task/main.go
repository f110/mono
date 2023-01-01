package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/notion"
)

type githubTaskCommand struct {
	notionToken    string
	githubToken    string
	appId          int64
	installationId int64
	privateKeyFile string
	configFile     string
	schedule       string
	oneshot        bool
}

func newGitHubTaskCommand() *githubTaskCommand {
	return &githubTaskCommand{
		schedule: "0 * * * *",
	}
}

func (g *githubTaskCommand) Flags(fs *pflag.FlagSet) {
	fs.StringVar(&g.notionToken, "notion-token", "", "API token for Notion")
	fs.StringVar(&g.githubToken, "github-token", "", "Personal access token of GitHub")
	fs.Int64Var(&g.appId, "github-app-id", 0, "GitHub App Id")
	fs.Int64Var(&g.installationId, "github-installation-id", 0, "GitHub App installation Id")
	fs.StringVar(&g.privateKeyFile, "github-private-key-file", "", "Private key file")
	fs.StringVar(&g.configFile, "config-file", "", "Config file path")
	fs.StringVar(&g.schedule, "schedule", g.schedule, "Check schedule")
	fs.BoolVar(&g.oneshot, "oneshot", false, "Oneshot execution")
}

func (g *githubTaskCommand) RequiredFlags() []string {
	return []string{"config-file"}
}

func (g *githubTaskCommand) Execute() error {
	if err := logger.Init(); err != nil {
		return xerrors.WithStack(err)
	}
	if g.githubToken == "" && os.Getenv("GITHUB_TOKEN") != "" {
		g.githubToken = os.Getenv("GITHUB_TOKEN")
	}
	if g.githubToken == "" && !(g.appId > 0 && g.installationId > 0 && g.privateKeyFile != "") {
		return xerrors.New("personal access token or GitHub App is required")
	}
	if g.notionToken == "" && os.Getenv("NOTION_TOKEN") != "" {
		g.notionToken = os.Getenv("NOTION_TOKEN")
	}
	if g.notionToken == "" {
		return xerrors.New("--notion-token or NOTION_TOKEN is required")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

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
	cmd := &cobra.Command{
		Use: "notion-github-task",
		RunE: func(_ *cobra.Command, _ []string) error {
			return githubTask.Execute()
		},
	}
	githubTask.Flags(cmd.Flags())
	logger.Flags(cmd.Flags())
	for _, v := range githubTask.RequiredFlags() {
		if err := cmd.MarkFlagRequired(v); err != nil {
			return xerrors.WithStack(err)
		}
	}

	cmd.SetArgs(args)
	return cmd.Execute()
}

func main() {
	if err := notionGitHubTask(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
