package buildctl

import (
	"go.f110.dev/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.f110.dev/mono/go/build/api"
)

func newClient(endpoint *string) (api.APIClient, error) {
	conn, err := grpc.NewClient(*endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return api.NewAPIClient(conn), nil
}
