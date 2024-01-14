package main

import (
	"context"
	"fmt"
	"os"

	"go.f110.dev/mono/go/cli"
)

func rotaryPress(args []string) error {
	r := NewRotaryPress()
	c := &cli.Command{
		Use: "rotarypress",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return r.LoopContext(ctx)
		},
	}
	r.SetFlags(c.Flags())
	return c.Execute(args)
}

func main() {
	if err := rotaryPress(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
