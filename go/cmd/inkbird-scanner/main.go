package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/ble/inkbird"
	"go.f110.dev/mono/go/logger"
)

func inkbirdScanner() error {
	ctx, cancel := signal.NotifyContext(context.Background())
	defer cancel()

	logger.Init()

	if err := inkbird.Scan(ctx); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func main() {
	if err := inkbirdScanner(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
