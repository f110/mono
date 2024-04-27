package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func controllerManager(args []string) error {
	c := New(args)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	return c.LoopContext(ctx)
}

func main() {
	if err := controllerManager(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v", err)
		os.Exit(1)
	}
}
