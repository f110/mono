package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/prometheus/exporter"
)

func inkbirdExporter(rawIDs []string, port int) error {
	var ids []string
	for _, v := range rawIDs {
		for _, id := range strings.Split(v, ",") {
			ids = append(ids, strings.ToLower(id))
		}
	}
	if len(ids) == 0 {
		return xerrors.New("--id is required")
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
			logger.Log.Warn("Failed shutdown exporter", logger.E(err))
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
	logger.Log.Info("Start inkbird exporter", slog.Int("port", port))
	err = http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return nil
}

func main() {
	var ids []string
	port := 9400
	cmd := &cli.Command{
		Use: "inkbird-exporter",
		Run: func(_ context.Context, _ *cli.Command, _ []string) error {
			return inkbirdExporter(ids, port)
		},
	}
	fs := cmd.Flags()
	fs.StringArray("id", "").Var(&ids)
	fs.Int("port", "").Var(&port).Default(port)
	if err := cmd.Execute(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
