package server

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/fsnotify/fsnotify"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/codexel/config"
	"go.f110.dev/mono/go/herdr"
	"go.f110.dev/mono/go/jujutsu"
	"go.f110.dev/mono/go/logger"
)

// snapshotInterval is the interval to snapshot the working copy while the repository is watched. The changes of the
// files are found by the snapshot.
const snapshotInterval = 5 * time.Second

type RepositoryHandler struct {
	config *config.File
	herdr  *herdr.Client
}

var _ api.RepositoryServiceHandler = (*RepositoryHandler)(nil)

func NewRepositoryHandler(conf *config.File, herdrClient *herdr.Client) *RepositoryHandler {
	return &RepositoryHandler{config: conf, herdr: herdrClient}
}

func (h *RepositoryHandler) ListRepositories(_ context.Context, _ *connect.Request[api.RequestListRepositories]) (*connect.Response[api.ResponseListRepositories], error) {
	conf, err := h.config.Load()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	repos := make([]*api.Repository, 0, len(conf.Repositories))
	for _, v := range conf.Repositories {
		repos = append(repos, toRepository(v.Path))
	}
	return connect.NewResponse(api.ResponseListRepositories_builder{Repositories: repos}.Build()), nil
}

func (h *RepositoryHandler) ListCandidates(ctx context.Context, _ *connect.Request[api.RequestListCandidates]) (*connect.Response[api.ResponseListCandidates], error) {
	conf, err := h.config.Load()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	agents, err := h.herdr.ListAgents(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}

	var paths []string
	for _, v := range agents {
		// The agent may work in the directory which is not the repository of jj.
		root, err := jujutsu.WorkspaceRoot(ctx, v.Cwd)
		if err != nil {
			continue
		}
		if slices.Contains(paths, root) || slices.ContainsFunc(conf.Repositories, func(r *config.Repository) bool { return r.Path == root }) {
			continue
		}
		paths = append(paths, root)
	}
	slices.Sort(paths)
	repos := make([]*api.Repository, 0, len(paths))
	for _, v := range paths {
		repos = append(repos, toRepository(v))
	}
	return connect.NewResponse(api.ResponseListCandidates_builder{Repositories: repos}.Build()), nil
}

func (h *RepositoryHandler) OpenRepository(ctx context.Context, req *connect.Request[api.RequestOpenRepository]) (*connect.Response[api.ResponseOpenRepository], error) {
	if req.Msg.GetPath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("path is required"))
	}
	root, err := jujutsu.WorkspaceRoot(ctx, req.Msg.GetPath())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	err = h.config.Update(func(c *config.Config) error {
		if !slices.ContainsFunc(c.Repositories, func(r *config.Repository) bool { return r.Path == root }) {
			c.Repositories = append(c.Repositories, &config.Repository{Path: root})
		}
		return nil
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(api.ResponseOpenRepository_builder{Repository: toRepository(root)}.Build()), nil
}

func (h *RepositoryHandler) CloseRepository(_ context.Context, req *connect.Request[api.RequestCloseRepository]) (*connect.Response[api.ResponseCloseRepository], error) {
	err := h.config.Update(func(c *config.Config) error {
		c.Repositories = slices.DeleteFunc(c.Repositories, func(r *config.Repository) bool { return r.Path == req.Msg.GetPath() })
		return nil
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&api.ResponseCloseRepository{}), nil
}

func (h *RepositoryHandler) WatchRepository(ctx context.Context, req *connect.Request[api.RequestWatchRepository], stream *connect.ServerStream[api.ResponseWatchRepository]) error {
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return err
	}
	dir, err := repo.OperationHeadsDir()
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return connect.NewError(connect.CodeInternal, xerrors.WithStack(err))
	}
	defer w.Close()
	if err := w.Add(dir); err != nil {
		return connect.NewError(connect.CodeInternal, xerrors.WithStack(err))
	}

	var current string
	send := func() error {
		id, err := repo.OperationID(ctx)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if id == current {
			return nil
		}
		current = id
		if err := stream.Send(api.ResponseWatchRepository_builder{OperationId: new(id)}.Build()); err != nil {
			return xerrors.WithStack(err)
		}
		return nil
	}
	if err := send(); err != nil {
		return err
	}

	ticker := time.NewTicker(snapshotInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := repo.Snapshot(ctx); err != nil && ctx.Err() == nil {
				logger.Log.Info("Failed to snapshot the working copy", slog.String("path", repo.Root()), logger.E(err))
			}
		case _, ok := <-w.Events:
			if !ok {
				return nil
			}
			if err := send(); err != nil {
				return err
			}
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			return connect.NewError(connect.CodeInternal, xerrors.WithStack(err))
		}
	}
}

type RevisionHandler struct {
	config *config.File
}

var _ api.RevisionServiceHandler = (*RevisionHandler)(nil)

func NewRevisionHandler(conf *config.File) *RevisionHandler {
	return &RevisionHandler{config: conf}
}

func (h *RevisionHandler) Log(ctx context.Context, req *connect.Request[api.RequestLog]) (*connect.Response[api.ResponseLog], error) {
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	rows, err := repo.Log(ctx, req.Msg.GetRevset(), int(req.Msg.GetLimit()))
	if err != nil {
		return nil, toJujutsuError(err)
	}
	res := make([]*api.LogRow, 0, len(rows))
	for _, v := range rows {
		lines := make([]*api.GraphLine, 0, len(v.Lines))
		for _, l := range v.Lines {
			lines = append(lines, api.GraphLine_builder{Gutter: new(l.Gutter), Content: new(l.Content), Node: new(l.Node)}.Build())
		}
		res = append(res, api.LogRow_builder{Commit: toCommit(v.Commit), Lines: lines}.Build())
	}
	return connect.NewResponse(api.ResponseLog_builder{Rows: res}.Build()), nil
}

func (h *RevisionHandler) GetRevision(ctx context.Context, req *connect.Request[api.RequestGetRevision]) (*connect.Response[api.ResponseGetRevision], error) {
	if req.Msg.GetRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("revision is required"))
	}
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	commit, err := repo.Show(ctx, req.Msg.GetRevision())
	if err != nil {
		return nil, toJujutsuError(err)
	}
	files, err := repo.Diff(ctx, req.Msg.GetRevision())
	if err != nil {
		return nil, toJujutsuError(err)
	}
	return connect.NewResponse(api.ResponseGetRevision_builder{
		Commit:      toCommit(commit),
		Description: new(commit.FullDescription),
		Files:       toFileDiffs(files),
	}.Build()), nil
}

func toFileDiffs(files []*jujutsu.FileDiff) []*api.FileDiff {
	res := make([]*api.FileDiff, 0, len(files))
	for _, f := range files {
		hunks := make([]*api.Hunk, 0, len(f.Hunks))
		for _, h := range f.Hunks {
			lines := make([]*api.DiffLine, 0, len(h.Lines))
			for _, l := range h.Lines {
				lines = append(lines, api.DiffLine_builder{
					Kind:      new(diffLineKinds[l.Kind]),
					Text:      new(l.Text),
					OldLine:   new(int32(l.OldLine)),
					NewLine:   new(int32(l.NewLine)),
					NoNewline: new(l.NoNewline),
				}.Build())
			}
			hunks = append(hunks, api.Hunk_builder{
				OldStart: new(int32(h.OldStart)),
				OldLines: new(int32(h.OldLines)),
				NewStart: new(int32(h.NewStart)),
				NewLines: new(int32(h.NewLines)),
				Section:  new(h.Section),
				Lines:    lines,
			}.Build())
		}
		res = append(res, api.FileDiff_builder{
			Path:    new(f.Path),
			OldPath: new(f.OldPath),
			Status:  new(fileStatuses[f.Status]),
			Binary:  new(f.Binary),
			Hunks:   hunks,
			Added:   new(int32(f.Added)),
			Removed: new(int32(f.Removed)),
		}.Build())
	}
	return res
}

var fileStatuses = map[jujutsu.FileStatus]api.FileStatus{
	jujutsu.FileStatusModified: api.FileStatus_FILE_STATUS_MODIFIED,
	jujutsu.FileStatusAdded:    api.FileStatus_FILE_STATUS_ADDED,
	jujutsu.FileStatusDeleted:  api.FileStatus_FILE_STATUS_DELETED,
	jujutsu.FileStatusRenamed:  api.FileStatus_FILE_STATUS_RENAMED,
	jujutsu.FileStatusCopied:   api.FileStatus_FILE_STATUS_COPIED,
}

var diffLineKinds = map[jujutsu.DiffLineKind]api.DiffLineKind{
	jujutsu.DiffLineContext: api.DiffLineKind_DIFF_LINE_KIND_CONTEXT,
	jujutsu.DiffLineAdded:   api.DiffLineKind_DIFF_LINE_KIND_ADDED,
	jujutsu.DiffLineRemoved: api.DiffLineKind_DIFF_LINE_KIND_REMOVED,
}

// openedRepository returns the repository only if it is opened. It prevents running jj in any directory.
func openedRepository(conf *config.File, path string) (*jujutsu.Repository, error) {
	c, err := conf.Load()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !slices.ContainsFunc(c.Repositories, func(r *config.Repository) bool { return r.Path == path }) {
		return nil, connect.NewError(connect.CodeNotFound, xerrors.Newf("%s is not opened", path))
	}
	return jujutsu.NewRepository(path), nil
}

func toRepository(path string) *api.Repository {
	return api.Repository_builder{Path: new(path), Name: new(filepath.Base(path))}.Build()
}

func toCommit(c *jujutsu.Commit) *api.Commit {
	remotes := make([]*api.RemoteBookmark, 0, len(c.RemoteBookmarks))
	for _, v := range c.RemoteBookmarks {
		remotes = append(remotes, api.RemoteBookmark_builder{Name: new(v.Name), Remote: new(v.Remote)}.Build())
	}
	trailers := make([]*api.Trailer, 0, len(c.Trailers))
	for _, v := range c.Trailers {
		trailers = append(trailers, api.Trailer_builder{Key: new(v.Key), Value: new(v.Value)}.Build())
	}
	return api.Commit_builder{
		ChangeId:           new(c.ChangeID),
		ChangeIdPrefix:     new(c.ChangePrefix),
		CommitId:           new(c.CommitID),
		CommitIdPrefix:     new(c.CommitPrefix),
		Description:        new(c.Description),
		Author:             new(c.Author),
		Email:              new(c.Email),
		Timestamp:          new(c.Timestamp),
		CurrentWorkingCopy: new(c.CurrentWorkingCopy),
		Immutable:          new(c.Immutable),
		Conflict:           new(c.Conflict),
		Empty:              new(c.Empty),
		Divergent:          new(c.Divergent),
		Hidden:             new(c.Hidden),
		Parents:            c.Parents,
		LocalBookmarks:     c.LocalBookmarks,
		RemoteBookmarks:    remotes,
		Trailers:           trailers,
	}.Build()
}

// toJujutsuError returns the error of jj with the message of jj because it is mostly caused by the input (e.g. the
// invalid revset).
func toJujutsuError(err error) error {
	if jjErr, ok := errors.AsType[*jujutsu.Error](err); ok {
		return connect.NewError(connect.CodeInvalidArgument, errors.New(jjErr.Error()))
	}
	return connect.NewError(connect.CodeInternal, err)
}
