package bff

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/logger"
)

func newAccessLogInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			t1 := time.Now()
			res, err := next(ctx, req)
			logger.Log.Info("access log", logger.String("endpoint", req.Spec().Procedure), zap.Duration("duration", time.Since(t1)))
			return res, err
		}
	}
}
