package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/prometheus/exporter"
)

func inkbirdExporter(args []string) error {
	var ids []string
	port := 9400
	fs := pflag.NewFlagSet("inkbird-exporter", pflag.ContinueOnError)
	fs.StringSliceVar(&ids, "id", ids, "")
	fs.IntVar(&port, "port", port, "")
	logger.Flags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(ids) == 0 {
		return xerrors.New("--id is required")
	}
	for i := range ids {
		ids[i] = strings.ToLower(ids[i])
	}

	if err := logger.Init(); err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	inkbirdExporter, err := exporter.NewInkBirdExporter(ctx, ids)
	if err != nil {
		return xerrors.WithStack(err)
	}
	prometheus.MustRegister(inkbirdExporter)

	go func() {
		<-ctx.Done()
		cancel()
		if err := inkbirdExporter.Shutdown(); err != nil {
			logger.Log.Warn("Failed shutdown exporter", zap.Error(err))
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
 <head><title>Inkbird Exporter</title></head>
 <body>
 <h1>Inkbird Exporter</h1>
 <p><a href='/metrics'>Metrics</a></p>
 </body>
 </html>`))
	})
	logger.Log.Info("Start inkbird exporter", zap.Int("port", port))
	err = http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return nil
}

func main() {
	if err := inkbirdExporter(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
