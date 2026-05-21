package grpcutil

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"

	"go.f110.dev/mono/go/logger/slogger"
)

func WithLogging() grpc.DialOption {
	return grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if !slogger.Enabled() {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		return Logging(slogger.Log, ctx, method, req, reply, cc, invoker, opts...)
	})
}

func Logging(l *slog.Logger, ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	t1 := time.Now()
	defer l.Debug(method, slog.Any("req", req), slog.Duration("elapsed", time.Since(t1)))
	return invoker(ctx, method, req, reply, cc, opts...)
}
