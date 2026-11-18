package server

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/codexel/config"
)

type FileHandler struct {
	config *config.File
}

var _ api.FileServiceHandler = (*FileHandler)(nil)

func NewFileHandler(conf *config.File) *FileHandler {
	return &FileHandler{config: conf}
}

func (h *FileHandler) GetFile(ctx context.Context, req *connect.Request[api.RequestGetFile]) (*connect.Response[api.ResponseGetFile], error) {
	if req.Msg.GetRevision() == "" || req.Msg.GetFile() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("revision and file are required"))
	}
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	content, err := repo.File(ctx, req.Msg.GetRevision(), req.Msg.GetFile())
	if err != nil {
		return nil, toJujutsuError(err)
	}
	return connect.NewResponse(api.ResponseGetFile_builder{Content: content}.Build()), nil
}

func (h *FileHandler) ListFiles(ctx context.Context, req *connect.Request[api.RequestListFiles]) (*connect.Response[api.ResponseListFiles], error) {
	if req.Msg.GetRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("revision is required"))
	}
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	files, err := repo.Files(ctx, req.Msg.GetRevision())
	if err != nil {
		return nil, toJujutsuError(err)
	}
	return connect.NewResponse(api.ResponseListFiles_builder{Files: files}.Build()), nil
}
