package grpcutil

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

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

func WithServerLogging() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !slogger.Enabled() {
			return handler(ctx, req)
		}
		t1 := time.Now()

		res, err := handler(ctx, req)
		slogger.Log.Info("access log",
			slog.String("endpoint", info.FullMethod),
			slog.Int("status", int(status.Code(err))),
			slog.Duration("duration", time.Since(t1)),
		)
		return res, err
	}
}

func Logging(l *slog.Logger, ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	t1 := time.Now()
	defer l.Debug(method, slog.Any("req", req), slog.Duration("elapsed", time.Since(t1)))
	return invoker(ctx, method, req, reply, cc, opts...)
}
