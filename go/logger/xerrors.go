package logger

import (
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
)

func Error(err error) zap.Field {
	if err == nil {
		return zap.Skip()
	}
	return zap.String("error", err.Error())
}

func StackTrace(err error) zap.Field {
	frames := xerrors.StackTrace(err)
	if frames == nil {
		return zap.Skip()
	}

	return zap.Array("stacktrace", frames)
}
