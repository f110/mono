package main

import (
	"fmt"
	"net"
	"os"

	"github.com/spf13/cobra"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/text/converter"
)

func docConverterService(args []string) error {
	port := 6391
	cmd := &cobra.Command{
		Use: "doc-converter-service",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := logger.Init(); err != nil {
				return xerrors.WithStack(err)
			}

			s := grpc.NewServer()
			converter.RegisterMarkdownTextConverterServer(s, &converter.MarkdownConverterService{})

			l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err != nil {
				return xerrors.WithStack(err)
			}
			logger.Log.Info("Listen", zap.Int("port", port))
			if err := s.Serve(l); err != nil {
				return xerrors.WithStack(err)
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", port, "Listen port")
	logger.Flags(cmd.Flags())

	cmd.SetArgs(args)
	return cmd.Execute()
}

func main() {
	if err := docConverterService(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
