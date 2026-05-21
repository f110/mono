package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	"go.f110.dev/xerrors"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/text/converter"
)

func docConverterService(args []string) error {
	port := 6391
	cmd := &cli.Command{
		Use: "doc-converter-service",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			s := grpc.NewServer()
			converter.RegisterMarkdownTextConverterServer(s, &converter.MarkdownConverterService{})

			l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err != nil {
				return xerrors.WithStack(err)
			}
			slogger.Log.Info("Listen", slog.Int("port", port))
			if err := s.Serve(l); err != nil {
				return xerrors.WithStack(err)
			}
			return nil
		},
	}
	cmd.Flags().Int("port", "Listen port").Var(&port).Shorthand("p").Default(port)

	return cmd.Execute(args)
}

func main() {
	if err := docConverterService(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
