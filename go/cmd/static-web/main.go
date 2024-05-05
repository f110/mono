package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"go.uber.org/zap"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/http/httpserver"
	"go.f110.dev/mono/go/logger"
)

type Mode string

const (
	ModeSPA    Mode = "spa"
	ModeSimple Mode = "simple"
)

func staticWeb() error {
	var documentRoot, listenAddr, mode string
	cmd := &cli.Command{
		Use:   "static-web",
		Short: "Serve static files",
		Run: func(ctx context.Context, cmd *cli.Command, _ []string) error {
			http.Handle("/favicon.ico", http.NotFoundHandler())
			switch Mode(mode) {
			case ModeSPA:
				http.Handle("/", httpserver.SinglePageApplication(documentRoot))
			case ModeSimple, "":
				http.Handle("/", http.FileServer(http.Dir(documentRoot)))
			}

			s := &http.Server{
				Addr:    listenAddr,
				Handler: http.DefaultServeMux,
			}
			go func() {
				<-ctx.Done()
				logger.Log.Info("Shutdown")
				s.Shutdown(context.Background())
			}()
			logger.Log.Info("Start server", zap.String("addr", listenAddr), zap.String("root", documentRoot), zap.String("mode", mode))
			if err := s.ListenAndServe(); errors.Is(err, http.ErrServerClosed) {
				return nil
			} else if err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().String("document-root", "The document root").Var(&documentRoot)
	cmd.Flags().String("listen-addr", "Listen address").Var(&listenAddr).Default(":8050")
	cmd.Flags().String("mode", "").Var(&mode).Default(string(ModeSimple))

	return cmd.Execute(os.Args)
}

func main() {
	if err := staticWeb(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
