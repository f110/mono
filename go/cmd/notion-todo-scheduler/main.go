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

type toDoSchedulerCommand struct {
	conf     string
	token    string
	schedule string
	dryRun   bool
	oneshot  bool
}

func newToDoSchedulerCommand() *toDoSchedulerCommand {
	return &toDoSchedulerCommand{
		schedule: "0 * * * *",
	}
}

func (s *toDoSchedulerCommand) Flags(fs *pflag.FlagSet) {
	fs.StringVar(&s.conf, "conf", s.conf, "Config file path")
	fs.StringVar(&s.token, "token", s.token, "API token for notion")
	fs.BoolVar(&s.dryRun, "dry-run", s.dryRun, "Dry run")
	fs.BoolVar(&s.oneshot, "oneshot", s.oneshot, "Execute only once")
	fs.StringVar(&s.schedule, "schedule", s.schedule, "Check schedule")
}

func (s *toDoSchedulerCommand) Execute() error {
	if err := logger.Init(); err != nil {
		return xerrors.WithStack(err)
	}
	if s.token == "" && os.Getenv("NOTION_TOKEN") != "" {
		s.token = os.Getenv("NOTION_TOKEN")
	}
	if s.token == "" {
		return xerrors.New("--token or NOTION_TOKEN is required")
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

	cmd := &cobra.Command{
		Use: "notion-todo-scheduler",
		RunE: func(_ *cobra.Command, _ []string) error {
			return todoSchedulerCmd.Execute()
		},
	}
	todoSchedulerCmd.Flags(cmd.Flags())
	logger.Flags(cmd.Flags())

	cmd.SetArgs(args)
	return cmd.Execute()
}

func main() {
	if err := notionToDoScheduler(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
