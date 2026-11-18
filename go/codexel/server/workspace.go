package server

import (
	"context"
	"errors"
	"path/filepath"
	"slices"

	"connectrpc.com/connect"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/codexel/config"
	"go.f110.dev/mono/go/jujutsu"
)

type WorkspaceHandler struct {
	config *config.File
}

var _ api.WorkspaceServiceHandler = (*WorkspaceHandler)(nil)

func NewWorkspaceHandler(conf *config.File) *WorkspaceHandler {
	return &WorkspaceHandler{config: conf}
}

func (h *WorkspaceHandler) ListRepositoryWorkspaces(ctx context.Context, req *connect.Request[api.RequestListRepositoryWorkspaces]) (*connect.Response[api.ResponseListRepositoryWorkspaces], error) {
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	workspaces, err := repo.Workspaces(ctx)
	if err != nil {
		return nil, toJujutsuError(err)
	}
	res := make([]*api.RepositoryWorkspace, 0, len(workspaces))
	for _, v := range workspaces {
		res = append(res, api.RepositoryWorkspace_builder{Name: new(v.Name), Target: new(v.Target), Root: new(v.Root)}.Build())
	}
	return connect.NewResponse(api.ResponseListRepositoryWorkspaces_builder{Workspaces: res}.Build()), nil
}

func (h *WorkspaceHandler) AddRepositoryWorkspace(ctx context.Context, req *connect.Request[api.RequestAddRepositoryWorkspace]) (*connect.Response[api.ResponseAddRepositoryWorkspace], error) {
	name := req.Msg.GetName()
	if name == "" || req.Msg.GetRevision() == "" || filepath.Base(name) != name || name == "." || name == ".." {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("valid name and revision are required"))
	}
	// The workspace is created next to the repository like lightjj.
	root := filepath.Join(filepath.Dir(req.Msg.GetPath()), filepath.Base(req.Msg.GetPath())+"-"+name)
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) {
		return r.WorkspaceAdd(ctx, name, req.Msg.GetRevision(), root)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseAddRepositoryWorkspace_builder{Output: new(out), Root: new(root)}.Build()), nil
}

func (h *WorkspaceHandler) ForgetRepositoryWorkspace(ctx context.Context, req *connect.Request[api.RequestForgetRepositoryWorkspace]) (*connect.Response[api.ResponseForgetRepositoryWorkspace], error) {
	if req.Msg.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	workspaces, err := repo.Workspaces(ctx)
	if err != nil {
		return nil, toJujutsuError(err)
	}
	conf, err := h.config.Load()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, w := range workspaces {
		if w.Name == req.Msg.GetName() && slices.ContainsFunc(conf.Repositories, func(r *config.Repository) bool { return sameDir(r.Path, w.Root) }) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, xerrors.Newf("%s is opened. Close it first", w.Root))
		}
	}
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.WorkspaceForget(ctx, req.Msg.GetName()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseForgetRepositoryWorkspace_builder{Output: new(out)}.Build()), nil
}

func (h *WorkspaceHandler) RenameRepositoryWorkspace(ctx context.Context, req *connect.Request[api.RequestRenameRepositoryWorkspace]) (*connect.Response[api.ResponseRenameRepositoryWorkspace], error) {
	if req.Msg.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.WorkspaceRename(ctx, req.Msg.GetName()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseRenameRepositoryWorkspace_builder{Output: new(out)}.Build()), nil
}

func (h *WorkspaceHandler) UpdateStale(ctx context.Context, req *connect.Request[api.RequestUpdateStale]) (*connect.Response[api.ResponseUpdateStale], error) {
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.WorkspaceUpdateStale(ctx) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseUpdateStale_builder{Output: new(out)}.Build()), nil
}

// sameDir reports whether a and b are the same directory. jj returns the path whose symbolic links are resolved (e.g.
// /private/tmp on macOS).
func sameDir(a, b string) bool {
	ra, err := filepath.EvalSymlinks(a)
	if err != nil {
		ra = a
	}
	rb, err := filepath.EvalSymlinks(b)
	if err != nil {
		rb = b
	}
	return filepath.Clean(ra) == filepath.Clean(rb)
}
