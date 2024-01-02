package main

import (
	"context"
	"fmt"
	"os"

	"go.f110.dev/mono/go/cli"
)

func simpleHTTPServer(args []string) error {
	s := NewSimpleHTTPServer()
	c := &cli.Command{
		Use: "simple-http-server",
		Run: func(ctx context.Context, cmd *cli.Command, _ []string) error {
			return s.LoopContext(ctx)
		},
	}
	s.SetFlags(c.Flags())
	return c.Execute(args)
}

func main() {
	if err := simpleHTTPServer(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
