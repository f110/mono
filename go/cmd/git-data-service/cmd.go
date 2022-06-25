package main

import (
	"context"
	"net"

	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/pkg/git"
	"go.f110.dev/mono/go/pkg/logger"
)

type gitDataServiceCommand struct {
	listen string
}

func (c *gitDataServiceCommand) Run(ctx context.Context) error {
	s := grpc.NewServer()
	service := newService()
	git.RegisterGitDataServer(s, service)
	lis, err := net.Listen("tcp", c.listen)
	if err != nil {
		return xerrors.WithStack(err)
	}

	logger.Log.Info("Start listen", zap.String("addr", c.listen))
	if err := s.Serve(lis); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (c *gitDataServiceCommand) Flags(fs *pflag.FlagSet) {
	fs.StringVar(&c.listen, "listen", ":8056", "Listen addr")
}
