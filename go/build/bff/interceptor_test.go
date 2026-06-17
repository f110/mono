package bff

import (
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestHTTPStatusFromError(t *testing.T) {
	cases := []struct {
		Name string
		Err  error
		Want int
	}{
		{Name: "success", Err: nil, Want: http.StatusOK},
		{Name: "internal", Err: connect.NewError(connect.CodeInternal, xerrors.New("boom")), Want: http.StatusInternalServerError},
		{Name: "invalid argument", Err: connect.NewError(connect.CodeInvalidArgument, xerrors.New("bad")), Want: http.StatusBadRequest},
		{Name: "not found", Err: connect.NewError(connect.CodeNotFound, nil), Want: http.StatusNotFound},
		{Name: "unauthenticated", Err: connect.NewError(connect.CodeUnauthenticated, nil), Want: http.StatusUnauthorized},
		{Name: "non-connect error", Err: xerrors.New("plain"), Want: http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			assertion.Equal(t, httpStatusFromError(tc.Err), tc.Want)
		})
	}
}
