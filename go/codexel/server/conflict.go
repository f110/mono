package server

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/codexel/config"
	"go.f110.dev/mono/go/jujutsu"
)

type ConflictHandler struct {
	config *config.File
}

var _ api.ConflictServiceHandler = (*ConflictHandler)(nil)

func NewConflictHandler(conf *config.File) *ConflictHandler {
	return &ConflictHandler{config: conf}
}

func (h *ConflictHandler) ListConflicts(ctx context.Context, req *connect.Request[api.RequestListConflicts]) (*connect.Response[api.ResponseListConflicts], error) {
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	commits, err := repo.Conflicts(ctx)
	if err != nil {
		return nil, toJujutsuError(err)
	}
	res := make([]*api.ConflictedCommit, 0, len(commits))
	for _, c := range commits {
		files := make([]*api.ConflictedFile, 0, len(c.Files))
		for _, f := range c.Files {
			files = append(files, api.ConflictedFile_builder{Path: new(f.Path), Sides: new(int32(f.Sides))}.Build())
		}
		res = append(res, api.ConflictedCommit_builder{
			ChangeId:    new(c.ChangeID),
			CommitId:    new(c.CommitID),
			Description: new(c.Description),
			Files:       files,
			WorkingCopy: new(c.WorkingCopy),
		}.Build())
	}
	return connect.NewResponse(api.ResponseListConflicts_builder{Commits: res}.Build()), nil
}

func (h *ConflictHandler) GetConflict(ctx context.Context, req *connect.Request[api.RequestGetConflict]) (*connect.Response[api.ResponseGetConflict], error) {
	if req.Msg.GetRevision() == "" || req.Msg.GetFile() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("revision and file are required"))
	}
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	content, err := repo.ConflictFile(ctx, req.Msg.GetRevision(), req.Msg.GetFile())
	if err != nil {
		return nil, toJujutsuError(err)
	}
	return connect.NewResponse(api.ResponseGetConflict_builder{Content: new(string(content))}.Build()), nil
}

var resolveSides = map[api.ResolveSide]jujutsu.ResolveSide{
	api.ResolveSide_RESOLVE_SIDE_OURS:   jujutsu.ResolveOurs,
	api.ResolveSide_RESOLVE_SIDE_THEIRS: jujutsu.ResolveTheirs,
}

func (h *ConflictHandler) Resolve(ctx context.Context, req *connect.Request[api.RequestResolve]) (*connect.Response[api.ResponseResolve], error) {
	side, ok := resolveSides[req.Msg.GetSide()]
	if !ok || req.Msg.GetRevision() == "" || req.Msg.GetFile() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("revision, file and side are required"))
	}
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) {
		return r.Resolve(ctx, req.Msg.GetRevision(), req.Msg.GetFile(), side)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseResolve_builder{Output: new(out)}.Build()), nil
}
