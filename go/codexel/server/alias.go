package server

import (
	"context"

	"connectrpc.com/connect"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/codexel/config"
	"go.f110.dev/mono/go/jujutsu"
)

type AliasHandler struct {
	config *config.File
}

var _ api.AliasServiceHandler = (*AliasHandler)(nil)

func NewAliasHandler(conf *config.File) *AliasHandler {
	return &AliasHandler{config: conf}
}

func (h *AliasHandler) ListAliases(ctx context.Context, req *connect.Request[api.RequestListAliases]) (*connect.Response[api.ResponseListAliases], error) {
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	aliases, err := repo.Aliases(ctx)
	if err != nil {
		return nil, toJujutsuError(err)
	}
	res := make([]*api.Alias, 0, len(aliases))
	for _, v := range aliases {
		res = append(res, api.Alias_builder{Name: new(v.Name), Args: v.Args}.Build())
	}
	return connect.NewResponse(api.ResponseListAliases_builder{Aliases: res}.Build()), nil
}

func (h *AliasHandler) RunAlias(ctx context.Context, req *connect.Request[api.RequestRunAlias]) (*connect.Response[api.ResponseRunAlias], error) {
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.RunAlias(ctx, req.Msg.GetName()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseRunAlias_builder{Output: new(out)}.Build()), nil
}
