package grpcutil

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/pkg/logger"
)

func WithLogging() grpc.DialOption {
	return grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if !logger.Enabled() {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		l := logger.Log.WithOptions(zap.AddCallerSkip(4))
		return Logging(l, ctx, method, req, reply, cc, invoker, opts...)
	})
}

func Logging(l *zap.Logger, ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	l.Debug(method, zap.Any("req", req))
	return invoker(ctx, method, req, reply, cc, opts...)
}
