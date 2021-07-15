package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/ble/inkbird"
	"go.f110.dev/mono/go/pkg/logger"
)

func inkbirdScanner() error {
	ctx, cancel := signal.NotifyContext(context.Background())
	defer cancel()

	logger.Init()

	if err := inkbird.Scan(ctx); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	return nil
}

func main() {
	if err := inkbirdScanner(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
