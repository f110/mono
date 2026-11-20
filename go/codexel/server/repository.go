package server

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/fsnotify/fsnotify"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/codexel/config"
	"go.f110.dev/mono/go/jujutsu"
	"go.f110.dev/mono/go/logger"
)

// snapshotInterval is the interval to snapshot the working copy while the repository is watched. The changes of the
// files are found by the snapshot.
const snapshotInterval = 5 * time.Second

type RepositoryHandler struct {
	config *config.File
}

var _ api.RepositoryServiceHandler = (*RepositoryHandler)(nil)

func NewRepositoryHandler(conf *config.File) *RepositoryHandler {
	return &RepositoryHandler{config: conf}
}

func (h *RepositoryHandler) List(_ context.Context, _ *connect.Request[api.RequestListRepositories]) (*connect.Response[api.ResponseListRepositories], error) {
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

func (h *RepositoryHandler) Open(ctx context.Context, req *connect.Request[api.RequestOpenRepository]) (*connect.Response[api.ResponseOpenRepository], error) {
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

func (h *RepositoryHandler) Close(_ context.Context, req *connect.Request[api.RequestCloseRepository]) (*connect.Response[api.ResponseCloseRepository], error) {
	err := h.config.Update(func(c *config.Config) error {
		c.Repositories = slices.DeleteFunc(c.Repositories, func(r *config.Repository) bool { return r.Path == req.Msg.GetPath() })
		return nil
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&api.ResponseCloseRepository{}), nil
}

func (h *RepositoryHandler) Watch(ctx context.Context, req *connect.Request[api.RequestWatchRepository], stream *connect.ServerStream[api.ResponseWatchRepository]) error {
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

func (h *RepositoryHandler) Log(ctx context.Context, req *connect.Request[api.RequestLog]) (*connect.Response[api.ResponseLog], error) {
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

func (h *RepositoryHandler) ListAgentCommits(ctx context.Context, req *connect.Request[api.RequestListAgentCommits]) (*connect.Response[api.ResponseListAgentCommits], error) {
	if req.Msg.GetCwd() == "" || req.Msg.GetKind() == "" || req.Msg.GetSessionId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cwd, kind and session_id are required"))
	}
	root, err := jujutsu.WorkspaceRoot(ctx, req.Msg.GetCwd())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	repo, err := openedRepository(h.config, root)
	if err != nil {
		return nil, err
	}
	commits, err := repo.AgentCommits(ctx, req.Msg.GetKind(), req.Msg.GetSessionId())
	if err != nil {
		return nil, toJujutsuError(err)
	}
	res := make([]*api.Commit, 0, len(commits))
	for _, v := range commits {
		res = append(res, toCommit(v))
	}
	return connect.NewResponse(api.ResponseListAgentCommits_builder{Repository: toRepository(root), Commits: res}.Build()), nil
}

func (h *RepositoryHandler) GetRevision(ctx context.Context, req *connect.Request[api.RequestGetRevision]) (*connect.Response[api.ResponseGetRevision], error) {
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
	var conflicts []jujutsu.ConflictedFile
	if commit.Conflict {
		conflicts, err = repo.ConflictedFiles(ctx, req.Msg.GetRevision())
		if err != nil {
			return nil, toJujutsuError(err)
		}
	}
	return connect.NewResponse(api.ResponseGetRevision_builder{
		Commit:      toCommit(commit),
		Description: new(commit.FullDescription),
		Files:       toFileDiffs(files),
		Conflicts:   toConflictedFiles(conflicts),
	}.Build()), nil
}

func (h *RepositoryHandler) New(ctx context.Context, req *connect.Request[api.RequestNew]) (*connect.Response[api.ResponseNew], error) {
	if len(req.Msg.GetRevisions()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("revisions are required"))
	}
	out, err := h.mutate(req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.New(ctx, req.Msg.GetRevisions()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseNew_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) Edit(ctx context.Context, req *connect.Request[api.RequestEdit]) (*connect.Response[api.ResponseEdit], error) {
	if req.Msg.GetRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("revision is required"))
	}
	out, err := h.mutate(req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.Edit(ctx, req.Msg.GetRevision()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseEdit_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) Abandon(ctx context.Context, req *connect.Request[api.RequestAbandon]) (*connect.Response[api.ResponseAbandon], error) {
	if len(req.Msg.GetRevisions()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("revisions are required"))
	}
	out, err := h.mutate(req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.Abandon(ctx, req.Msg.GetRevisions()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseAbandon_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) Describe(ctx context.Context, req *connect.Request[api.RequestDescribe]) (*connect.Response[api.ResponseDescribe], error) {
	if req.Msg.GetRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("revision is required"))
	}
	out, err := h.mutate(req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) {
		return r.Describe(ctx, req.Msg.GetRevision(), req.Msg.GetDescription())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseDescribe_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) Commit(ctx context.Context, req *connect.Request[api.RequestCommit]) (*connect.Response[api.ResponseCommit], error) {
	out, err := h.mutate(req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.Commit(ctx, req.Msg.GetDescription()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseCommit_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) Undo(ctx context.Context, req *connect.Request[api.RequestUndo]) (*connect.Response[api.ResponseUndo], error) {
	out, err := h.mutate(req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.Undo(ctx) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseUndo_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) Rebase(ctx context.Context, req *connect.Request[api.RequestRebase]) (*connect.Response[api.ResponseRebase], error) {
	source, ok := rebaseSources[req.Msg.GetSource()]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source is required"))
	}
	target, ok := rebaseTargets[req.Msg.GetTarget()]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target is required"))
	}
	if len(req.Msg.GetRevisions()) == 0 || len(req.Msg.GetDestinations()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("revisions and destinations are required"))
	}
	out, err := h.mutate(req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) {
		return r.Rebase(ctx, req.Msg.GetRevisions(), source, req.Msg.GetDestinations(), target)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseRebase_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) Squash(ctx context.Context, req *connect.Request[api.RequestSquash]) (*connect.Response[api.ResponseSquash], error) {
	if len(req.Msg.GetRevisions()) == 0 || req.Msg.GetInto() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("revisions and into are required"))
	}
	out, err := h.mutate(req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) {
		return r.Squash(ctx, req.Msg.GetRevisions(), req.Msg.GetInto(), req.Msg.GetDescription())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseSquash_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) Evolog(ctx context.Context, req *connect.Request[api.RequestEvolog]) (*connect.Response[api.ResponseEvolog], error) {
	if req.Msg.GetRevision() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("revision is required"))
	}
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	entries, err := repo.Evolog(ctx, req.Msg.GetRevision())
	if err != nil {
		return nil, toJujutsuError(err)
	}
	res := make([]*api.EvologEntry, 0, len(entries))
	for _, v := range entries {
		res = append(res, api.EvologEntry_builder{
			CommitId:     new(v.CommitID),
			Timestamp:    new(v.Timestamp),
			Operation:    new(v.Operation),
			Predecessors: v.Predecessors,
			Files:        toFileDiffs(v.Files),
		}.Build())
	}
	return connect.NewResponse(api.ResponseEvolog_builder{Entries: res}.Build()), nil
}

func (h *RepositoryHandler) Split(ctx context.Context, req *connect.Request[api.RequestSplit]) (*connect.Response[api.ResponseSplit], error) {
	if req.Msg.GetRevision() == "" || len(req.Msg.GetFiles()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("revision and files are required"))
	}
	out, err := h.mutate(req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) {
		return r.Split(ctx, req.Msg.GetRevision(), req.Msg.GetDescription(), req.Msg.GetFiles())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseSplit_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) Restore(ctx context.Context, req *connect.Request[api.RequestRestore]) (*connect.Response[api.ResponseRestore], error) {
	if req.Msg.GetRevision() == "" || len(req.Msg.GetFiles()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("revision and files are required"))
	}
	out, err := h.mutate(req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) {
		return r.Restore(ctx, req.Msg.GetRevision(), req.Msg.GetFiles())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseRestore_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) DiffRange(ctx context.Context, req *connect.Request[api.RequestDiffRange]) (*connect.Response[api.ResponseDiffRange], error) {
	if req.Msg.GetFrom() == "" || req.Msg.GetTo() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("from and to are required"))
	}
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	files, err := repo.DiffRange(ctx, req.Msg.GetFrom(), req.Msg.GetTo())
	if err != nil {
		return nil, toJujutsuError(err)
	}
	return connect.NewResponse(api.ResponseDiffRange_builder{Files: toFileDiffs(files)}.Build()), nil
}

func (h *RepositoryHandler) ListBookmarks(ctx context.Context, req *connect.Request[api.RequestListBookmarks]) (*connect.Response[api.ResponseListBookmarks], error) {
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

func (h *RepositoryHandler) SetBookmark(ctx context.Context, req *connect.Request[api.RequestSetBookmark]) (*connect.Response[api.ResponseSetBookmark], error) {
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

func (h *RepositoryHandler) MoveBookmark(ctx context.Context, req *connect.Request[api.RequestMoveBookmark]) (*connect.Response[api.ResponseMoveBookmark], error) {
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

func (h *RepositoryHandler) DeleteBookmark(ctx context.Context, req *connect.Request[api.RequestDeleteBookmark]) (*connect.Response[api.ResponseDeleteBookmark], error) {
	if req.Msg.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.BookmarkDelete(ctx, req.Msg.GetName()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseDeleteBookmark_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) ForgetBookmark(ctx context.Context, req *connect.Request[api.RequestForgetBookmark]) (*connect.Response[api.ResponseForgetBookmark], error) {
	if req.Msg.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.BookmarkForget(ctx, req.Msg.GetName()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseForgetBookmark_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) TrackBookmark(ctx context.Context, req *connect.Request[api.RequestTrackBookmark]) (*connect.Response[api.ResponseTrackBookmark], error) {
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

func (h *RepositoryHandler) UntrackBookmark(ctx context.Context, req *connect.Request[api.RequestUntrackBookmark]) (*connect.Response[api.ResponseUntrackBookmark], error) {
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

func (h *RepositoryHandler) Fetch(ctx context.Context, req *connect.Request[api.RequestFetch]) (*connect.Response[api.ResponseFetch], error) {
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.GitFetch(ctx) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseFetch_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) Push(ctx context.Context, req *connect.Request[api.RequestPush]) (*connect.Response[api.ResponsePush], error) {
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.GitPush(ctx, req.Msg.GetBookmark()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponsePush_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) ListOperations(ctx context.Context, req *connect.Request[api.RequestListOperations]) (*connect.Response[api.ResponseListOperations], error) {
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

func (h *RepositoryHandler) RevertOperation(ctx context.Context, req *connect.Request[api.RequestRevertOperation]) (*connect.Response[api.ResponseRevertOperation], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.OperationRevert(ctx, req.Msg.GetId()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseRevertOperation_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) RestoreOperation(ctx context.Context, req *connect.Request[api.RequestRestoreOperation]) (*connect.Response[api.ResponseRestoreOperation], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.OperationRestore(ctx, req.Msg.GetId()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseRestoreOperation_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) GetFile(ctx context.Context, req *connect.Request[api.RequestGetFile]) (*connect.Response[api.ResponseGetFile], error) {
	if req.Msg.GetRevision() == "" || req.Msg.GetFile() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("revision and file are required"))
	}
	if !filepath.IsLocal(req.Msg.GetFile()) {
		return nil, connect.NewError(connect.CodeInvalidArgument, xerrors.Newf("the file is outside of the repository: %s", req.Msg.GetFile()))
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

func (h *RepositoryHandler) ListFiles(ctx context.Context, req *connect.Request[api.RequestListFiles]) (*connect.Response[api.ResponseListFiles], error) {
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

func (h *RepositoryHandler) WriteFile(ctx context.Context, req *connect.Request[api.RequestWriteFile]) (*connect.Response[api.ResponseWriteFile], error) {
	repo, err := openedRepository(h.config, req.Msg.GetPath())
	if err != nil {
		return nil, err
	}
	if err := repo.WriteFile(ctx, req.Msg.GetFile(), req.Msg.GetContent()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(&api.ResponseWriteFile{}), nil
}

func (h *RepositoryHandler) ListConflicts(ctx context.Context, req *connect.Request[api.RequestListConflicts]) (*connect.Response[api.ResponseListConflicts], error) {
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
		res = append(res, api.ConflictedCommit_builder{
			ChangeId:    new(c.ChangeID),
			CommitId:    new(c.CommitID),
			Description: new(c.Description),
			Files:       toConflictedFiles(c.Files),
			WorkingCopy: new(c.WorkingCopy),
		}.Build())
	}
	return connect.NewResponse(api.ResponseListConflicts_builder{Commits: res}.Build()), nil
}

func (h *RepositoryHandler) GetConflict(ctx context.Context, req *connect.Request[api.RequestGetConflict]) (*connect.Response[api.ResponseGetConflict], error) {
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

func (h *RepositoryHandler) Resolve(ctx context.Context, req *connect.Request[api.RequestResolve]) (*connect.Response[api.ResponseResolve], error) {
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

func (h *RepositoryHandler) ListWorkspaces(ctx context.Context, req *connect.Request[api.RequestListRepositoryWorkspaces]) (*connect.Response[api.ResponseListRepositoryWorkspaces], error) {
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

func (h *RepositoryHandler) AddWorkspace(ctx context.Context, req *connect.Request[api.RequestAddRepositoryWorkspace]) (*connect.Response[api.ResponseAddRepositoryWorkspace], error) {
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

func (h *RepositoryHandler) ForgetWorkspace(ctx context.Context, req *connect.Request[api.RequestForgetRepositoryWorkspace]) (*connect.Response[api.ResponseForgetRepositoryWorkspace], error) {
	name := req.Msg.GetName()
	out, err := mutateClosedWorkspace(ctx, h.config, req.Msg.GetPath(), name, func(r *jujutsu.Repository) (string, error) { return r.WorkspaceForget(ctx, name) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseForgetRepositoryWorkspace_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) RemoveWorkspace(ctx context.Context, req *connect.Request[api.RequestRemoveRepositoryWorkspace]) (*connect.Response[api.ResponseRemoveRepositoryWorkspace], error) {
	name := req.Msg.GetName()
	out, err := mutateClosedWorkspace(ctx, h.config, req.Msg.GetPath(), name, func(r *jujutsu.Repository) (string, error) { return r.WorkspaceRemove(ctx, name) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseRemoveRepositoryWorkspace_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) RenameWorkspace(ctx context.Context, req *connect.Request[api.RequestRenameRepositoryWorkspace]) (*connect.Response[api.ResponseRenameRepositoryWorkspace], error) {
	if req.Msg.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.WorkspaceRename(ctx, req.Msg.GetName()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseRenameRepositoryWorkspace_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) UpdateStale(ctx context.Context, req *connect.Request[api.RequestUpdateStale]) (*connect.Response[api.ResponseUpdateStale], error) {
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.WorkspaceUpdateStale(ctx) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseUpdateStale_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) ListAliases(ctx context.Context, req *connect.Request[api.RequestListAliases]) (*connect.Response[api.ResponseListAliases], error) {
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

func (h *RepositoryHandler) RunAlias(ctx context.Context, req *connect.Request[api.RequestRunAlias]) (*connect.Response[api.ResponseRunAlias], error) {
	out, err := mutate(h.config, req.Msg.GetPath(), func(r *jujutsu.Repository) (string, error) { return r.RunAlias(ctx, req.Msg.GetName()) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(api.ResponseRunAlias_builder{Output: new(out)}.Build()), nil
}

func (h *RepositoryHandler) mutate(path string, fn func(*jujutsu.Repository) (string, error)) (string, error) {
	return mutate(h.config, path, fn)
}

var rebaseSources = map[api.RebaseSource]jujutsu.RebaseSource{
	api.RebaseSource_REBASE_SOURCE_REVISIONS:   jujutsu.RebaseRevisions,
	api.RebaseSource_REBASE_SOURCE_DESCENDANTS: jujutsu.RebaseSourceDescendants,
	api.RebaseSource_REBASE_SOURCE_BRANCH:      jujutsu.RebaseBranch,
}

var rebaseTargets = map[api.RebaseTarget]jujutsu.RebaseTarget{
	api.RebaseTarget_REBASE_TARGET_ONTO:          jujutsu.RebaseOnto,
	api.RebaseTarget_REBASE_TARGET_INSERT_AFTER:  jujutsu.RebaseInsertAfter,
	api.RebaseTarget_REBASE_TARGET_INSERT_BEFORE: jujutsu.RebaseInsertBefore,
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

var resolveSides = map[api.ResolveSide]jujutsu.ResolveSide{
	api.ResolveSide_RESOLVE_SIDE_OURS:   jujutsu.ResolveOurs,
	api.ResolveSide_RESOLVE_SIDE_THEIRS: jujutsu.ResolveTheirs,
}

// openAgentRepositories opens the workspaces where the agents work. The home directory is excluded because the agent
// started there is not for the repository.
func openAgentRepositories(ctx context.Context, conf *config.File, cwds []string, workspaceRoot func(context.Context, string) (string, error)) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return xerrors.WithStack(err)
	}
	var roots []string
	for _, v := range cwds {
		if v == home {
			continue
		}
		// The agent may work in the directory which is not the repository of jj.
		root, err := workspaceRoot(ctx, v)
		if err != nil {
			continue
		}
		if !slices.Contains(roots, root) {
			roots = append(roots, root)
		}
	}
	c, err := conf.Load()
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(roots, func(root string) bool { return !isOpened(c, root) }) {
		return nil
	}
	return conf.Update(func(c *config.Config) error {
		for _, v := range roots {
			if !isOpened(c, v) {
				c.Repositories = append(c.Repositories, &config.Repository{Path: v})
			}
		}
		return nil
	})
}

func isOpened(c *config.Config, path string) bool {
	return slices.ContainsFunc(c.Repositories, func(r *config.Repository) bool { return r.Path == path })
}

// mutate runs the operation on the opened repository.
func mutate(conf *config.File, path string, fn func(*jujutsu.Repository) (string, error)) (string, error) {
	repo, err := openedRepository(conf, path)
	if err != nil {
		return "", err
	}
	out, err := fn(repo)
	if err != nil {
		return "", toJujutsuError(err)
	}
	return out, nil
}

// mutateClosedWorkspace runs fn on the workspace of name in the repository of path. The workspace opened in Codexel is
// rejected because fn stops tracking the workspace.
func mutateClosedWorkspace(ctx context.Context, conf *config.File, path, name string, fn func(*jujutsu.Repository) (string, error)) (string, error) {
	if name == "" {
		return "", connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	repo, err := openedRepository(conf, path)
	if err != nil {
		return "", err
	}
	workspaces, err := repo.Workspaces(ctx)
	if err != nil {
		return "", toJujutsuError(err)
	}
	c, err := conf.Load()
	if err != nil {
		return "", connect.NewError(connect.CodeInternal, err)
	}
	for _, w := range workspaces {
		if w.Name == name && slices.ContainsFunc(c.Repositories, func(r *config.Repository) bool { return sameDir(r.Path, w.Root) }) {
			return "", connect.NewError(connect.CodeFailedPrecondition, xerrors.Newf("%s is opened. Close it first", w.Root))
		}
	}
	out, err := fn(repo)
	if err != nil {
		return "", toJujutsuError(err)
	}
	return out, nil
}

func toFileDiffs(files []*jujutsu.FileDiff) []*api.FileDiff {
	res := make([]*api.FileDiff, 0, len(files))
	for _, f := range files {
		res = append(res, toFileDiff(f))
	}
	return res
}

func toConflictedFiles(files []jujutsu.ConflictedFile) []*api.ConflictedFile {
	res := make([]*api.ConflictedFile, 0, len(files))
	for _, f := range files {
		res = append(res, api.ConflictedFile_builder{Path: new(f.Path), Sides: new(int32(f.Sides))}.Build())
	}
	return res
}

func toFileDiff(f *jujutsu.FileDiff) *api.FileDiff {
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
	return api.FileDiff_builder{
		Path:    new(f.Path),
		OldPath: new(f.OldPath),
		Status:  new(fileStatuses[f.Status]),
		Binary:  new(f.Binary),
		Hunks:   hunks,
		Added:   new(int32(f.Added)),
		Removed: new(int32(f.Removed)),
	}.Build()
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
		WorkingCopies:      c.WorkingCopies,
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
