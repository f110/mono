package bff

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"

	"go.f110.dev/mono/go/logger/slogger"
)

func newAccessLogInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			t1 := time.Now()
			res, err := next(ctx, req)
			slogger.Log.Info("access log",
				slog.String("endpoint", req.Spec().Procedure),
				slog.Int("status", httpStatusFromError(err)),
				slog.Duration("duration", time.Since(t1)),
			)
			return res, err
		}
	}
}

// httpStatusFromError maps the connect outcome to the HTTP status code the
// client receives over the Connect unary protocol. It mirrors connect-go's
// internal connectCodeToHTTP so the logged status matches what is sent on the
// wire. A nil error means the call succeeded (200).
func httpStatusFromError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	switch connect.CodeOf(err) {
	case connect.CodeCanceled:
		return 499 // client closed request (no http constant)
	case connect.CodeInvalidArgument, connect.CodeFailedPrecondition, connect.CodeOutOfRange:
		return http.StatusBadRequest
	case connect.CodeDeadlineExceeded:
		return http.StatusGatewayTimeout
	case connect.CodeNotFound:
		return http.StatusNotFound
	case connect.CodeAlreadyExists, connect.CodeAborted:
		return http.StatusConflict
	case connect.CodePermissionDenied:
		return http.StatusForbidden
	case connect.CodeResourceExhausted:
		return http.StatusTooManyRequests
	case connect.CodeUnimplemented:
		return http.StatusNotImplemented
	case connect.CodeUnavailable:
		return http.StatusServiceUnavailable
	case connect.CodeUnauthenticated:
		return http.StatusUnauthorized
	default: // CodeUnknown, CodeInternal, CodeDataLoss
		return http.StatusInternalServerError
	}
}
