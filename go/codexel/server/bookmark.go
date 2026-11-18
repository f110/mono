package server

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/codexel/config"
	"go.f110.dev/mono/go/jujutsu"
)

type BookmarkHandler struct {
	config *config.File
}

var _ api.BookmarkServiceHandler = (*BookmarkHandler)(nil)

func NewBookmarkHandler(conf *config.File) *BookmarkHandler {
	return &BookmarkHandler{config: conf}
}

func (h *BookmarkHandler) ListBookmarks(ctx context.Context, req *connect.Request[api.RequestListBookmarks]) (*connect.Response[api.ResponseListBookmarks], error) {
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	bookmarks, err := repo.Bookmarks(ctx)
	if err != nil {
		return nil, toJujutsuError(err)
	}
	res := make([]*api.Bookmark, 0, len(bookmarks))
	for _, v := range bookmarks {
		res = append(res, api.Bookmark_builder{
			Name:        new(v.Name),
			Remote:      new(v.Remote),
			Tracked:     new(v.Tracked),
			Conflict:    new(v.Conflict),
			Synced:      new(v.Synced),
			Target:      new(v.Target),
			Ahead:       new(int32(v.Ahead)),
			Behind:      new(int32(v.Behind)),
			Description: new(v.Description),
		}.Build())
	}
	return connect.NewResponse(api.ResponseListBookmarks_builder{Bookmarks: res}.Build()), nil
}

func (h *BookmarkHandler) SetBookmark(ctx context.Context, req *connect.Request[api.RequestSetBookmark]) (*connect.Response[api.ResponseSetBookmark], error) {
	if req.Msg.GetName() == "" || req.Msg.GetRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name and revision are required"))
	}
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) {
		return r.BookmarkSet(ctx, req.Msg.GetName(), req.Msg.GetRevision())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseSetBookmark_builder{Output: new(out)}.Build()), nil
}

func (h *BookmarkHandler) MoveBookmark(ctx context.Context, req *connect.Request[api.RequestMoveBookmark]) (*connect.Response[api.ResponseMoveBookmark], error) {
	if req.Msg.GetName() == "" || req.Msg.GetRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name and revision are required"))
	}
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) {
		return r.BookmarkMove(ctx, req.Msg.GetName(), req.Msg.GetRevision())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseMoveBookmark_builder{Output: new(out)}.Build()), nil
}

func (h *BookmarkHandler) DeleteBookmark(ctx context.Context, req *connect.Request[api.RequestDeleteBookmark]) (*connect.Response[api.ResponseDeleteBookmark], error) {
	if req.Msg.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.BookmarkDelete(ctx, req.Msg.GetName()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseDeleteBookmark_builder{Output: new(out)}.Build()), nil
}

func (h *BookmarkHandler) ForgetBookmark(ctx context.Context, req *connect.Request[api.RequestForgetBookmark]) (*connect.Response[api.ResponseForgetBookmark], error) {
	if req.Msg.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.BookmarkForget(ctx, req.Msg.GetName()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseForgetBookmark_builder{Output: new(out)}.Build()), nil
}

func (h *BookmarkHandler) TrackBookmark(ctx context.Context, req *connect.Request[api.RequestTrackBookmark]) (*connect.Response[api.ResponseTrackBookmark], error) {
	if req.Msg.GetName() == "" || req.Msg.GetRemote() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name and remote are required"))
	}
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) {
		return r.BookmarkTrack(ctx, req.Msg.GetName(), req.Msg.GetRemote())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseTrackBookmark_builder{Output: new(out)}.Build()), nil
}

func (h *BookmarkHandler) UntrackBookmark(ctx context.Context, req *connect.Request[api.RequestUntrackBookmark]) (*connect.Response[api.ResponseUntrackBookmark], error) {
	if req.Msg.GetName() == "" || req.Msg.GetRemote() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name and remote are required"))
	}
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) {
		return r.BookmarkUntrack(ctx, req.Msg.GetName(), req.Msg.GetRemote())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseUntrackBookmark_builder{Output: new(out)}.Build()), nil
}

type GitHandler struct {
	config *config.File
}

var _ api.GitServiceHandler = (*GitHandler)(nil)

func NewGitHandler(conf *config.File) *GitHandler {
	return &GitHandler{config: conf}
}

func (h *GitHandler) Fetch(ctx context.Context, req *connect.Request[api.RequestFetch]) (*connect.Response[api.ResponseFetch], error) {
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.GitFetch(ctx) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseFetch_builder{Output: new(out)}.Build()), nil
}

func (h *GitHandler) Push(ctx context.Context, req *connect.Request[api.RequestPush]) (*connect.Response[api.ResponsePush], error) {
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.GitPush(ctx, req.Msg.GetBookmark()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponsePush_builder{Output: new(out)}.Build()), nil
}
