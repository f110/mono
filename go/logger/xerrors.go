package logger

import (
	"errors"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"
)

func Error(err error) zap.Field {
	if err == nil {
		return zap.Skip()
	}
	return zap.String("error", err.Error())
}

func Verbose(err error) zap.Field {
	for {
		v, ok := err.(interface{ Verbose() string })
		if ok {
			return zap.String("verbose", v.Verbose())
		}
		err = errors.Unwrap(err)
		if err == nil {
			break
		}
	}
	return zap.Skip()
}

func StackTrace(err error) zap.Field {
	frames := xerrors.StackTrace(err)
	if frames == nil {
		return zap.Skip()
	}

	return zap.Array("stacktrace", frames)
}
