package main

import (
	"bytes"
	"context"
	"os"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/fsm"
)

func init() {
	CommandManager.Register(Graph())
}

func graph(dir string) error {
	buf := new(bytes.Buffer)
	if err := fsm.DotOutput(buf, dir); err != nil {
		return xerrors.WithStack(err)
	}
	buf.WriteTo(os.Stdout)
	return nil
}

func Graph() *cli.Command {
	graphCmd := &cli.Command{
		Use:   "graph",
		Short: "Visualize FSM",
		Run: func(_ context.Context, _ *cli.Command, args []string) error {
			return graph(args[0])
		},
	}

	return graphCmd
}
