package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/notion"
	"go.f110.dev/mono/go/pkg/signals"
)

func notionToDoServer(args []string) error {
	addr := ":7000"
	conf := ""
	token := ""
	fs := pflag.NewFlagSet("notion-doc-server", pflag.ContinueOnError)
	fs.StringVar(&addr, "addr", addr, "Listen addr")
	fs.StringVar(&conf, "conf", conf, "Config file path")
	fs.StringVar(&token, "token", "", "API token for notion")
	logger.Flags(fs)
	if err := fs.Parse(args); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if err := logger.Init(); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if token == "" && os.Getenv("NOTION_TOKEN") != "" {
		token = os.Getenv("NOTION_TOKEN")
	}
	if token == "" {
		return xerrors.Errorf("--token or NOTION_TOKEN is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	signals.SetupSignalHandler(cancel)

	srv, err := notion.NewDatabaseDocServer(addr, conf, token)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	go srv.Start()
	<-ctx.Done()
	cancel()

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Stop(ctx); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func main() {
	if err := notionToDoServer(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
