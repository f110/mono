package server

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/codexel/config"
	"go.f110.dev/mono/go/jujutsu"
)

type OperationHandler struct {
	config *config.File
}

var _ api.OperationServiceHandler = (*OperationHandler)(nil)

func NewOperationHandler(conf *config.File) *OperationHandler {
	return &OperationHandler{config: conf}
}

func (h *OperationHandler) ListOperations(ctx context.Context, req *connect.Request[api.RequestListOperations]) (*connect.Response[api.ResponseListOperations], error) {
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	ops, err := repo.Operations(ctx, int(req.Msg.GetLimit()))
	if err != nil {
		return nil, toJujutsuError(err)
	}
	res := make([]*api.Operation, 0, len(ops))
	for _, v := range ops {
		res = append(res, api.Operation_builder{
			Id:          new(v.ID),
			Description: new(v.Description),
			User:        new(v.User),
			Time:        new(v.Time),
			Current:     new(v.Current),
		}.Build())
	}
	return connect.NewResponse(api.ResponseListOperations_builder{Operations: res}.Build()), nil
}

func (h *OperationHandler) RevertOperation(ctx context.Context, req *connect.Request[api.RequestRevertOperation]) (*connect.Response[api.ResponseRevertOperation], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.OperationRevert(ctx, req.Msg.GetId()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseRevertOperation_builder{Output: new(out)}.Build()), nil
}

func (h *OperationHandler) RestoreOperation(ctx context.Context, req *connect.Request[api.RequestRestoreOperation]) (*connect.Response[api.ResponseRestoreOperation], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.OperationRestore(ctx, req.Msg.GetId()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseRestoreOperation_builder{Output: new(out)}.Build()), nil
}
