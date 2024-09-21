package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/notion"
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

func (s *docServerCommand) Flags(fs *cli.FlagSet) {
	fs.String("addr", "Listen addr").Var(&s.addr).Default(":7000")
	fs.String("conf", "Config file path").Var(&s.conf)
	fs.String("token", "API token for notion").Var(&s.token)
}

func (s *docServerCommand) Execute(ctx context.Context) error {
	if s.token == "" && os.Getenv("NOTION_TOKEN") != "" {
		s.token = os.Getenv("NOTION_TOKEN")
	}
	if s.token == "" {
		return xerrors.Define("--token or NOTION_TOKEN is required").WithStack()
	}

	srv, err := notion.NewDatabaseDocServer(s.addr, s.conf, s.token)
	if err != nil {
		return xerrors.WithStack(err)
	}
	go srv.Start()
	<-ctx.Done()

	sCtx, cancel := ctxutil.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := srv.Stop(sCtx); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func notionDocServer(args []string) error {
	docServerCmd := newDocServerCommand()
	cmd := &cli.Command{
		Use: "notion-doc-server",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return docServerCmd.Execute(ctx)
		},
	}
	docServerCmd.Flags(cmd.Flags())

	return cmd.Execute(args)
}

func main() {
	if err := notionDocServer(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
