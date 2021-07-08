package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/prometheus/exporter"
	"go.uber.org/zap"
)

func inkbirdExporter(args []string) error {
	id := ""
	minimumInterval := 1 * time.Minute
	port := 9400
	fs := pflag.NewFlagSet("inkbird-exporter", pflag.ContinueOnError)
	fs.StringVar(&id, "id", id, "")
	fs.DurationVar(&minimumInterval, "minimum-interval", minimumInterval, "")
	fs.IntVar(&port, "port", port, "")
	logger.Flags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if id == "" {
		return errors.New("--id is required")
	}
	id = strings.ToLower(id)

	if err := logger.Init(); err != nil {
		return err
	}

	inkbirdExporter := exporter.NewInkBirdExporter(id, minimumInterval)
	prometheus.MustRegister(inkbirdExporter)

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
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
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
