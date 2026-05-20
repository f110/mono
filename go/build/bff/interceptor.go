package bff

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect"

	"go.f110.dev/mono/go/logger/slogger"
)

func newAccessLogInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			t1 := time.Now()
			res, err := next(ctx, req)
			slogger.Log.Info("access log", slog.String("endpoint", req.Spec().Procedure), slog.Duration("duration", time.Since(t1)))
			return res, err
		}
	}
}
