package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/notion"
	"go.f110.dev/mono/go/pkg/signals"
)

type docServerCommand struct {
	addr  string
	conf  string
	token string
}

func newDocServerCommand() *docServerCommand {
	return &docServerCommand{
		addr: ":7000",
	}
}

func (s *docServerCommand) Flags(fs *pflag.FlagSet) {
	fs.StringVar(&s.addr, "addr", s.addr, "Listen addr")
	fs.StringVar(&s.conf, "conf", s.conf, "Config file path")
	fs.StringVar(&s.token, "token", "", "API token for notion")
}

func (s *docServerCommand) Execute() error {
	if err := logger.Init(); err != nil {
		return xerrors.WithStack(err)
	}
	if s.token == "" && os.Getenv("NOTION_TOKEN") != "" {
		s.token = os.Getenv("NOTION_TOKEN")
	}
	if s.token == "" {
		return errors.New("--token or NOTION_TOKEN is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	signals.SetupSignalHandler(cancel)

	srv, err := notion.NewDatabaseDocServer(s.addr, s.conf, s.token)
	if err != nil {
		return xerrors.WithStack(err)
	}
	go srv.Start()
	<-ctx.Done()
	cancel()

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Stop(ctx); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func notionDocServer(args []string) error {
	docServerCmd := newDocServerCommand()
	cmd := &cobra.Command{
		Use: "notion-doc-server",
		RunE: func(_ *cobra.Command, _ []string) error {
			return docServerCmd.Execute()
		},
	}
	docServerCmd.Flags(cmd.Flags())
	logger.Flags(cmd.Flags())

	cmd.SetArgs(args)

	return cmd.Execute()
}

func main() {
	if err := notionDocServer(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
