package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/notion"
)

type toDoSchedulerCommand struct {
	conf     string
	token    string
	schedule string
	dryRun   bool
	oneshot  bool
}

func newToDoSchedulerCommand() *toDoSchedulerCommand {
	return &toDoSchedulerCommand{}
}

func (s *toDoSchedulerCommand) Flags(fs *cli.FlagSet) {
	fs.String("conf", "Config file path").Var(&s.conf)
	fs.String("token", "API token for notion").Var(&s.token)
	fs.Bool("dry-run", "Dry run").Var(&s.dryRun)
	fs.Bool("oneshot", "Execute only once").Var(&s.oneshot)
	fs.String("schedule", "Check schedule").Var(&s.schedule).Default("0 * * * *")
}

func (s *toDoSchedulerCommand) Execute() error {
	if s.token == "" && os.Getenv("NOTION_TOKEN") != "" {
		s.token = os.Getenv("NOTION_TOKEN")
	}
	if s.token == "" {
		return xerrors.Define("--token or NOTION_TOKEN is required").WithStack()
	}
	if s.conf == "" {
		return xerrors.Define("--conf is required").WithStack()
	}

	scheduler, err := notion.NewToDoScheduler(s.conf, s.token)
	if err != nil {
		return xerrors.WithStack(err)
	}

	if s.oneshot {
		if err := scheduler.Execute(s.dryRun); err != nil {
			return xerrors.WithStack(err)
		}
		return nil
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer cancel()

	if err := scheduler.Start(s.schedule); err != nil {
		return xerrors.WithStack(err)
	}
	<-ctx.Done()

	return nil
}

func notionToDoScheduler(args []string) error {
	todoSchedulerCmd := newToDoSchedulerCommand()

	cmd := &cli.Command{
		Use: "notion-todo-scheduler",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return todoSchedulerCmd.Execute()
		},
	}
	todoSchedulerCmd.Flags(cmd.Flags())

	return cmd.Execute(args)
}

func main() {
	if err := notionToDoScheduler(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
