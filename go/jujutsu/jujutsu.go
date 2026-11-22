// Package jujutsu provides the operations of the repositories of Jujutsu by the jj command.
package jujutsu

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.f110.dev/xerrors"
)

const (
	// fieldSeparator separates the gutter from the commit in the output of jj log.
	fieldSeparator = "\x1F"

	// logTemplate outputs the commit as JSON after fieldSeparator.
	logTemplate = `"\x1F{" ++ ` + commitFields + ` ++ "}\n"`

	// showTemplate outputs the commit as JSON with the whole description.
	showTemplate = `"{" ++ ` + commitFields + ` ++ ",\"full_description\":" ++ json(description) ++ "}"`

	// commitFields is the fields of Commit in JSON. The lists are built by join because jj can't serialize the list of
	// the templates.
	commitFields = `"\"change_id\":" ++ json(change_id.short()) ++ ` +
		`",\"change_prefix\":" ++ json(change_id.shortest().prefix()) ++ ` +
		`",\"commit_id\":" ++ json(commit_id.short()) ++ ` +
		`",\"commit_prefix\":" ++ json(commit_id.shortest().prefix()) ++ ` +
		`",\"description\":" ++ json(description.first_line()) ++ ` +
		`",\"author\":" ++ json(author.name()) ++ ` +
		`",\"email\":" ++ json(author.email()) ++ ` +
		`",\"timestamp\":" ++ json(committer.timestamp().format("%s")) ++ ` +
		`",\"working_copies\":[" ++ working_copies.map(|x| json(x.name())).join(",") ++ "]" ++ ` +
		`",\"current_working_copy\":" ++ json(current_working_copy) ++ ` +
		`",\"immutable\":" ++ json(immutable) ++ ` +
		`",\"conflict\":" ++ json(conflict) ++ ` +
		`",\"empty\":" ++ json(empty) ++ ` +
		`",\"divergent\":" ++ json(divergent) ++ ` +
		`",\"hidden\":" ++ json(self.hidden()) ++ ` +
		`",\"parents\":[" ++ parents.map(|x| json(x.commit_id().short())).join(",") ++ "]" ++ ` +
		`",\"local_bookmarks\":[" ++ local_bookmarks.map(|x| json(x.name())).join(",") ++ "]" ++ ` +
		`",\"remote_bookmarks\":[" ++ remote_bookmarks.map(|x| "[" ++ json(x.name()) ++ "," ++ json(x.remote()) ++ "]").join(",") ++ "]" ++ ` +
		`",\"trailers\":[" ++ trailers.map(|x| "[" ++ json(x.key()) ++ "," ++ json(x.value()) ++ "]").join(",") ++ "]"`

	graphRunes = " │─╮╯╭╰├┤┬┴┼~"

	// The trailers which link the commit to the session of the agent.
	TrailerAgent        = "Agent"
	TrailerAgentSession = "Agent-Session"

	bookmarkTemplate = `"{" ++ ` +
		`"\"name\":" ++ json(name) ++ ` +
		`",\"remote\":" ++ json(if(remote, remote, "")) ++ ` +
		`",\"tracked\":" ++ json(tracked) ++ ` +
		`",\"conflict\":" ++ json(conflict) ++ ` +
		`",\"synced\":" ++ json(synced) ++ ` +
		`",\"target\":" ++ json(if(normal_target, normal_target.commit_id().short(), "")) ++ ` +
		`",\"ahead\":" ++ if(tracked, self.tracking_ahead_count().lower(), "0") ++ ` +
		`",\"behind\":" ++ if(tracked, self.tracking_behind_count().lower(), "0") ++ ` +
		`",\"description\":" ++ json(if(normal_target, normal_target.description().first_line(), "")) ++ ` +
		`"}\n"`

	workspaceTemplate = `"{\"name\":" ++ json(name) ++ ",\"target\":" ++ json(target.commit_id().short()) ++ ",\"root\":" ++ json(self.root()) ++ "}\n"`

	conflictTemplate = `"{" ++ ` +
		`"\"change_id\":" ++ json(change_id.short()) ++ ` +
		`",\"commit_id\":" ++ json(commit_id.short()) ++ ` +
		`",\"description\":" ++ json(description.first_line()) ++ ` +
		`",\"working_copy\":" ++ json(current_working_copy) ++ ` +
		`",\"files\":[" ++ self.conflicted_files().map(|f| "{\"path\":" ++ json(f.path()) ++ ",\"sides\":" ++ f.conflict_side_count() ++ "}").join(",") ++ "]" ++ ` +
		`"}\n"`

	operationTemplate = `"{" ++ ` +
		`"\"id\":" ++ json(self.id().short()) ++ ` +
		`",\"description\":" ++ json(self.description()) ++ ` +
		`",\"user\":" ++ json(self.user()) ++ ` +
		`",\"time\":" ++ json(self.time().start().format("%s")) ++ ` +
		`",\"current\":" ++ json(self.current_operation()) ++ ` +
		`"}\n"`

	// evologTemplate outputs the entry as JSON, the record separator and the diff in the git format. The entries are
	// separated by the group separator.
	evologTemplate = `"{" ++ ` +
		`"\"commit_id\":" ++ json(commit.commit_id().short()) ++ ` +
		`",\"time\":" ++ json(commit.committer().timestamp().format("%s")) ++ ` +
		`",\"operation\":" ++ json(operation.description()) ++ ` +
		`",\"predecessors\":[" ++ predecessors.map(|p| json(p.commit_id().short())).join(",") ++ "]" ++ ` +
		`"}\x1E" ++ self.inter_diff().git() ++ "\x1D"`

	aliasTemplate = `"{\"name\":" ++ json(name) ++ ",\"value\":" ++ json(value) ++ "}\n"`
)

const (
	// RebaseRevisions rebases only the revisions (-r).
	RebaseRevisions RebaseSource = "revisions"
	// RebaseSourceDescendants rebases the revisions and the descendants (-s).
	RebaseSourceDescendants RebaseSource = "source"
	// RebaseBranch rebases the whole branch relative to the destination (-b).
	RebaseBranch RebaseSource = "branch"
)

const (
	RebaseOnto         RebaseTarget = "onto"
	RebaseInsertAfter  RebaseTarget = "insert-after"
	RebaseInsertBefore RebaseTarget = "insert-before"
)

const (
	FileStatusModified FileStatus = iota
	FileStatusAdded
	FileStatusDeleted
	FileStatusRenamed
	FileStatusCopied
)

const (
	DiffLineContext DiffLineKind = iota
	DiffLineAdded
	DiffLineRemoved
)

// Repository is the workspace of the repository. All commands run in the root of the workspace.
type Repository struct {
	root string
}

func NewRepository(root string) *Repository {
	return &Repository{root: root}
}

func (r *Repository) Root() string {
	return r.root
}

// WorkspaceRoot returns the root of the workspace which contains dir.
func WorkspaceRoot(ctx context.Context, dir string) (string, error) {
	out, err := run(ctx, dir, "workspace", "root")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ShortChangeID returns the change ID in the same form as Commit.ChangeID, which is the first 12 characters by
// change_id.short() of jj.
func ShortChangeID(changeID string) string {
	return changeID[:min(len(changeID), 12)]
}

func (r *Repository) Log(ctx context.Context, revset string, limit int) ([]*LogRow, error) {
	out, err := run(ctx, r.root, logArgs(revset, limit)...)
	if err != nil {
		return nil, err
	}
	return parseLog(out)
}

// AgentCommits returns the commits created by the session of the agent.
func (r *Repository) AgentCommits(ctx context.Context, kind, sessionID string) ([]*Commit, error) {
	rows, err := r.Log(ctx, agentRevset(kind, sessionID), 0)
	if err != nil {
		return nil, err
	}
	var commits []*Commit
	for _, v := range rows {
		if v.Commit.CreatedBy(kind, sessionID) {
			commits = append(commits, v.Commit)
		}
	}
	return commits, nil
}

func (r *Repository) Show(ctx context.Context, rev string) (*Commit, error) {
	out, err := run(ctx, r.root, showArgs(rev)...)
	if err != nil {
		return nil, err
	}
	return parseShow(out)
}

// OperationHeadsDir returns the directory of the heads of the operations. The name of the file in the directory is the
// ID of the operation and jj replaces the file on every operation.
func (r *Repository) OperationHeadsDir() (string, error) {
	repo := filepath.Join(r.root, ".jj", "repo")
	fi, err := os.Stat(repo)
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	// .jj/repo of the secondary workspace is the file which has the path to the repository.
	if !fi.IsDir() {
		buf, err := os.ReadFile(repo)
		if err != nil {
			return "", xerrors.WithStack(err)
		}
		repo = strings.TrimSpace(string(buf))
		if !filepath.IsAbs(repo) {
			repo = filepath.Join(r.root, ".jj", repo)
		}
	}
	return filepath.Join(repo, "op_heads", "heads"), nil
}

// OperationID returns the ID of the current operation.
func (r *Repository) OperationID(ctx context.Context) (string, error) {
	dir, err := r.OperationHeadsDir()
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	if len(entries) == 1 {
		return entries[0].Name(), nil
	}
	// The heads diverge while the concurrent operations are not merged yet. jj merges them.
	out, err := run(ctx, r.root, "op", "log", "--no-graph", "--limit", "1", "-T", "self.id()")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Snapshot snapshots the working copy. The operation changes if the working copy has the changes.
func (r *Repository) Snapshot(ctx context.Context) error {
	_, err := run(ctx, r.root, "util", "snapshot")
	return err
}

func (r *Repository) New(ctx context.Context, revs []string) (string, error) {
	return r.mutate(ctx, newArgs(revs)...)
}

func (r *Repository) Edit(ctx context.Context, rev string) (string, error) {
	return r.mutate(ctx, editArgs(rev)...)
}

func (r *Repository) Abandon(ctx context.Context, revs []string) (string, error) {
	return r.mutate(ctx, abandonArgs(revs)...)
}

func (r *Repository) Describe(ctx context.Context, rev, message string) (string, error) {
	return r.mutate(ctx, describeArgs(rev, message)...)
}

// Commit describes the working copy and creates the new working copy on it.
func (r *Repository) Commit(ctx context.Context, message string) (string, error) {
	return r.mutate(ctx, commitArgs(message)...)
}

func (r *Repository) Undo(ctx context.Context) (string, error) {
	return r.mutate(ctx, "undo")
}

func (r *Repository) mutate(ctx context.Context, args ...string) (string, error) {
	_, stderr, err := runWithStderr(ctx, r.root, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(stderr)), nil
}

func (r *Repository) Rebase(ctx context.Context, revs []string, source RebaseSource, destinations []string, target RebaseTarget) (string, error) {
	return r.mutate(ctx, rebaseArgs(revs, source, destinations, target)...)
}

func (r *Repository) Squash(ctx context.Context, from []string, into, message string) (string, error) {
	return r.mutate(ctx, squashArgs(from, into, message)...)
}

func (r *Repository) Split(ctx context.Context, rev, message string, files []string) (string, error) {
	return r.mutate(ctx, splitArgs(rev, message, files)...)
}

func (r *Repository) Restore(ctx context.Context, rev string, files []string) (string, error) {
	return r.mutate(ctx, restoreArgs(rev, files)...)
}

// Bookmarks returns the local and the remote bookmarks. The remote "git" of the colocated repository is excluded.
func (r *Repository) Bookmarks(ctx context.Context) ([]*Bookmark, error) {
	out, err := run(ctx, r.root, bookmarksArgs()...)
	if err != nil {
		return nil, err
	}
	return parseBookmarks(out)
}

// BookmarkSet creates the bookmark or moves it to the revision.
func (r *Repository) BookmarkSet(ctx context.Context, name, rev string) (string, error) {
	return r.mutate(ctx, bookmarkSetArgs(name, rev)...)
}

// BookmarkMove moves the bookmark to the revision even if it moves backwards or sideways.
func (r *Repository) BookmarkMove(ctx context.Context, name, rev string) (string, error) {
	return r.mutate(ctx, bookmarkMoveArgs(name, rev)...)
}

// BookmarkDelete deletes the bookmark. The tracked remote bookmarks are deleted on the next push.
func (r *Repository) BookmarkDelete(ctx context.Context, name string) (string, error) {
	return r.mutate(ctx, bookmarkDeleteArgs(name)...)
}

// BookmarkForget forgets the bookmark without deleting the remote bookmarks.
func (r *Repository) BookmarkForget(ctx context.Context, name string) (string, error) {
	return r.mutate(ctx, bookmarkForgetArgs(name)...)
}

func (r *Repository) BookmarkTrack(ctx context.Context, name, remote string) (string, error) {
	return r.mutate(ctx, bookmarkTrackArgs(name, remote)...)
}

func (r *Repository) BookmarkUntrack(ctx context.Context, name, remote string) (string, error) {
	return r.mutate(ctx, bookmarkUntrackArgs(name, remote)...)
}

func (r *Repository) GitFetch(ctx context.Context) (string, error) {
	return r.mutate(ctx, gitFetchArgs()...)
}

func (r *Repository) GitPush(ctx context.Context, bookmarks []string) (string, error) {
	return r.mutate(ctx, gitPushArgs(bookmarks)...)
}

func (r *Repository) Workspaces(ctx context.Context) ([]*Workspace, error) {
	out, err := run(ctx, r.root, "workspace", "list", "--ignore-working-copy", "-T", workspaceTemplate)
	if err != nil {
		return nil, err
	}
	var workspaces []*Workspace
	for line := range strings.Lines(string(out)) {
		w := &Workspace{}
		if err := json.Unmarshal([]byte(line), w); err != nil {
			return nil, xerrors.WithMessagef(err, "failed to parse the workspace: %s", line)
		}
		workspaces = append(workspaces, w)
	}
	return workspaces, nil
}

// WorkspaceAdd creates the workspace in dir whose working copy is on the revision.
func (r *Repository) WorkspaceAdd(ctx context.Context, name, rev, dir string) (string, error) {
	return r.mutate(ctx, workspaceAddArgs(name, rev, dir)...)
}

// WorkspaceForget stops tracking the workspace. The directory is left.
func (r *Repository) WorkspaceForget(ctx context.Context, name string) (string, error) {
	return r.mutate(ctx, "workspace", "forget", "--", name)
}

// WorkspaceRemove removes the workspace and its directory. The working copy is snapshotted before removing.
func (r *Repository) WorkspaceRemove(ctx context.Context, name string) (string, error) {
	return r.mutate(ctx, "workspace", "remove", "--", name)
}

// WorkspaceRename renames the workspace of the repository.
func (r *Repository) WorkspaceRename(ctx context.Context, name string) (string, error) {
	return r.mutate(ctx, "workspace", "rename", "--", name)
}

// WorkspaceUpdateStale updates the stale working copy of the workspace.
func (r *Repository) WorkspaceUpdateStale(ctx context.Context) (string, error) {
	return r.mutate(ctx, "workspace", "update-stale")
}

// File returns the content of the file at the revision.
func (r *Repository) File(ctx context.Context, rev, path string) ([]byte, error) {
	return run(ctx, r.root, fileShowArgs(rev, path)...)
}

// Files returns the paths of all files at the revision.
func (r *Repository) Files(ctx context.Context, rev string) ([]string, error) {
	out, err := run(ctx, r.root, fileListArgs(rev)...)
	if err != nil {
		return nil, err
	}
	var paths []string
	for line := range strings.Lines(string(out)) {
		var p string
		if err := json.Unmarshal([]byte(line), &p); err != nil {
			return nil, xerrors.WithMessagef(err, "failed to parse the path: %s", line)
		}
		paths = append(paths, p)
	}
	return paths, nil
}

func (r *Repository) Diff(ctx context.Context, rev string) ([]*FileDiff, error) {
	out, err := run(ctx, r.root, diffArgs(rev)...)
	if err != nil {
		return nil, err
	}
	return parseGitDiff(string(out))
}

// DiffRange returns the diff between two revisions.
func (r *Repository) DiffRange(ctx context.Context, from, to string) ([]*FileDiff, error) {
	out, err := run(ctx, r.root, diffRangeArgs(from, to)...)
	if err != nil {
		return nil, err
	}
	return parseGitDiff(string(out))
}

// Interdiff returns the difference between the changes of two revisions. The difference of the parents is ignored and
// the difference of the descriptions is the file JJ-COMMIT-DESCRIPTION.
func (r *Repository) Interdiff(ctx context.Context, from, to string) ([]*FileDiff, error) {
	out, err := run(ctx, r.root, interdiffArgs(from, to)...)
	if err != nil {
		return nil, err
	}
	return parseGitDiff(string(out))
}

// ConflictedFiles returns the conflicted files of the revision. It includes the files which aren't changed by the
// revision because the conflict is inherited from the parents.
func (r *Repository) ConflictedFiles(ctx context.Context, rev string) ([]ConflictedFile, error) {
	commits, err := r.conflicts(ctx, rev)
	if err != nil {
		return nil, err
	}
	if len(commits) != 1 {
		return nil, xerrors.Newf("the revision must be a commit: %s", rev)
	}
	return commits[0].Files, nil
}

func (r *Repository) conflicts(ctx context.Context, revset string) ([]*ConflictedCommit, error) {
	out, err := run(ctx, r.root, conflictsArgs(revset)...)
	if err != nil {
		return nil, err
	}
	var commits []*ConflictedCommit
	for line := range strings.Lines(string(out)) {
		c := &ConflictedCommit{}
		if err := json.Unmarshal([]byte(line), c); err != nil {
			return nil, xerrors.WithMessagef(err, "failed to parse the conflict: %s", line)
		}
		commits = append(commits, c)
	}
	return commits, nil
}

// Operations returns the operations from the newest.
func (r *Repository) Operations(ctx context.Context, limit int) ([]*Operation, error) {
	out, err := run(ctx, r.root, operationsArgs(limit)...)
	if err != nil {
		return nil, err
	}
	var ops []*Operation
	for line := range strings.Lines(string(out)) {
		op := &Operation{}
		if err := json.Unmarshal([]byte(line), op); err != nil {
			return nil, xerrors.WithMessagef(err, "failed to parse the operation: %s", line)
		}
		ops = append(ops, op)
	}
	return ops, nil
}

// OperationRevert creates the operation which reverts the operation.
func (r *Repository) OperationRevert(ctx context.Context, id string) (string, error) {
	return r.mutate(ctx, "op", "revert", "--", id)
}

// OperationRestore restores the repository to the state at the operation. The operations after it are discarded.
func (r *Repository) OperationRestore(ctx context.Context, id string) (string, error) {
	return r.mutate(ctx, "op", "restore", "--", id)
}

// Evolog returns the versions of the revision from the newest.
func (r *Repository) Evolog(ctx context.Context, rev string) ([]*EvologEntry, error) {
	out, err := run(ctx, r.root, evologArgs(rev)...)
	if err != nil {
		return nil, err
	}
	return parseEvolog(string(out))
}

// Aliases returns the aliases configured by the user. The default aliases of jj are not included.
func (r *Repository) Aliases(ctx context.Context) ([]*Alias, error) {
	out, err := run(ctx, r.root, "config", "list", "--ignore-working-copy", "-T", aliasTemplate, "aliases")
	if err != nil {
		return nil, err
	}
	return parseAliases(out)
}

// RunAlias runs the alias configured by the user and returns the output of stdout and stderr.
func (r *Repository) RunAlias(ctx context.Context, name string) (string, error) {
	aliases, err := r.Aliases(ctx)
	if err != nil {
		return "", err
	}
	if !slices.ContainsFunc(aliases, func(a *Alias) bool { return a.Name == name }) || strings.HasPrefix(name, "-") {
		return "", xerrors.Newf("unknown alias: %s", name)
	}
	stdout, stderr, err := runWithStderr(ctx, r.root, name)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(stdout) + string(stderr)), nil
}

type RemoteBookmark struct {
	Name   string
	Remote string
}

func (b *RemoteBookmark) UnmarshalJSON(buf []byte) error {
	var v [2]string
	if err := json.Unmarshal(buf, &v); err != nil {
		return xerrors.WithStack(err)
	}
	b.Name, b.Remote = v[0], v[1]
	return nil
}

type Trailer struct {
	Key   string
	Value string
}

func (t *Trailer) UnmarshalJSON(buf []byte) error {
	var v [2]string
	if err := json.Unmarshal(buf, &v); err != nil {
		return xerrors.WithStack(err)
	}
	t.Key, t.Value = v[0], v[1]
	return nil
}

type Commit struct {
	ChangeID string `json:"change_id"`
	// ChangePrefix is the shortest unique prefix of ChangeID.
	ChangePrefix string `json:"change_prefix"`
	CommitID     string `json:"commit_id"`
	// CommitPrefix is the shortest unique prefix of CommitID.
	CommitPrefix string `json:"commit_prefix"`
	// Description is the first line of the description.
	Description string `json:"description"`
	Author      string `json:"author"`
	Email       string `json:"email"`
	// Timestamp is the unix time of the committer.
	Timestamp          int64            `json:"timestamp,string"`
	WorkingCopies      []string         `json:"working_copies"`
	CurrentWorkingCopy bool             `json:"current_working_copy"`
	Immutable          bool             `json:"immutable"`
	Conflict           bool             `json:"conflict"`
	Empty              bool             `json:"empty"`
	Divergent          bool             `json:"divergent"`
	Hidden             bool             `json:"hidden"`
	Parents            []string         `json:"parents"`
	LocalBookmarks     []string         `json:"local_bookmarks"`
	RemoteBookmarks    []RemoteBookmark `json:"remote_bookmarks"`
	Trailers           []Trailer        `json:"trailers"`
	// FullDescription is the whole description. It is set only by Show.
	FullDescription string `json:"full_description"`
}

// CreatedBy reports whether the commit has the trailers of the session of the agent.
func (c *Commit) CreatedBy(kind, sessionID string) bool {
	return slices.Contains(c.Trailers, Trailer{Key: TrailerAgent, Value: kind}) &&
		slices.Contains(c.Trailers, Trailer{Key: TrailerAgentSession, Value: sessionID})
}

// GraphLine is a line of the graph of jj log. Gutter is the characters of the graph (e.g. "│ ○  ").
type GraphLine struct {
	Gutter string
	// Content is the text after the gutter of the line which is not the node (e.g. "(elided revisions)").
	Content string
	Node    bool
}

// LogRow is a commit and the lines of the graph. Lines starts with the line of the node and continues with the
// lines connecting to the next commit.
type LogRow struct {
	Commit *Commit
	Lines  []GraphLine
}

// Error is the error of the jj command. Stderr is the message from jj.
type Error struct {
	Args   []string
	Stderr string
	err    error
}

func (e *Error) Error() string {
	return "jj " + e.Args[0] + ": " + strings.TrimSpace(e.Stderr)
}

func (e *Error) Unwrap() error {
	return e.err
}

// RebaseSource is what to rebase.
type RebaseSource string

// RebaseTarget is where to rebase.
type RebaseTarget string

type Bookmark struct {
	Name string `json:"name"`
	// Remote is empty for the local bookmark.
	Remote   string `json:"remote"`
	Tracked  bool   `json:"tracked"`
	Conflict bool   `json:"conflict"`
	// Synced is true if the local and the tracked remote bookmarks point to the same commit.
	Synced bool `json:"synced"`
	// Target is the commit ID which the bookmark points to. It is empty for the conflicted bookmark.
	Target string `json:"target"`
	// Ahead and Behind are set to the tracked remote bookmark. Ahead is the number of the commits of the remote bookmark
	// which the local one doesn't have, and Behind is the number of the commits of the local one which the remote one
	// doesn't have.
	Ahead       int    `json:"ahead"`
	Behind      int    `json:"behind"`
	Description string `json:"description"`
}

type Workspace struct {
	Name string `json:"name"`
	// Target is the commit ID of the working copy of the workspace.
	Target string `json:"target"`
	Root   string `json:"root"`
}

type FileStatus int

type DiffLineKind int

type DiffLine struct {
	Kind DiffLineKind
	Text string
	// OldLine and NewLine are the line numbers in the old and the new file. 0 means the line doesn't exist in the file.
	OldLine int
	NewLine int
	// NoNewline is true if the line is the end of the file without the newline.
	NoNewline bool
}

type Hunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	// Section is the text after the range of the header.
	Section string
	Lines   []*DiffLine
}

type FileDiff struct {
	Path string
	// OldPath is the path before the rename or the copy.
	OldPath string
	Status  FileStatus
	Binary  bool
	Hunks   []*Hunk
	Added   int
	Removed int
}

type ConflictedFile struct {
	Path string `json:"path"`
	// Sides is the number of the sides of the conflict. 2 is the conflict between two changes.
	Sides int `json:"sides"`
}

type ConflictedCommit struct {
	ChangeID    string           `json:"change_id"`
	CommitID    string           `json:"commit_id"`
	Description string           `json:"description"`
	Files       []ConflictedFile `json:"files"`
	// WorkingCopy is true if the commit is the working copy. Only the working copy can be resolved by writing the file.
	WorkingCopy bool `json:"working_copy"`
}

type Operation struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	User        string `json:"user"`
	// Time is the unix time when the operation started.
	Time    int64 `json:"time,string"`
	Current bool  `json:"current"`
}

type EvologEntry struct {
	CommitID string `json:"commit_id"`
	// Timestamp is the unix time of the committer.
	Timestamp    int64    `json:"time,string"`
	Operation    string   `json:"operation"`
	Predecessors []string `json:"predecessors"`
	// Files is the changes which the version adds relative to its parents. It is not the diff from the predecessor, so
	// the rebase doesn't appear as the change.
	Files []*FileDiff `json:"-"`
}

type Alias struct {
	Name string
	// Args is the arguments of jj which the alias runs.
	Args []string
}

// logArgs returns the arguments of jj log. The style of the graph is fixed because the parser depends on it. The revset
// is joined with "=" not to be interpreted as the option (e.g. "-r --config=x" is the option --config).
// --ignore-working-copy avoids taking the snapshot of the working copy on every read.
func logArgs(revset string, limit int) []string {
	args := []string{"log", "--color", "never", "--quiet", "--ignore-working-copy", "--config", "ui.graph.style=curved"}
	if revset != "" {
		args = append(args, "--revisions="+revset)
	}
	if limit > 0 {
		args = append(args, "--limit", strconv.Itoa(limit))
	}
	return append(args, "-T", logTemplate)
}

// parseLog parses the output of jj log by logArgs.
func parseLog(out []byte) ([]*LogRow, error) {
	var rows []*LogRow
	for line := range strings.Lines(string(out)) {
		line = strings.TrimSuffix(line, "\n")
		if gutter, commit, ok := strings.Cut(line, fieldSeparator); ok {
			c := &Commit{}
			if err := json.Unmarshal([]byte(commit), c); err != nil {
				return nil, xerrors.WithMessagef(err, "failed to parse the commit: %s", commit)
			}
			// The remote "git" is the synthetic remote of the colocated repository.
			c.RemoteBookmarks = slices.DeleteFunc(c.RemoteBookmarks, func(b RemoteBookmark) bool { return b.Remote == "git" })
			rows = append(rows, &LogRow{Commit: c, Lines: []GraphLine{{Gutter: gutter, Node: true}}})
			continue
		}
		if len(rows) == 0 {
			continue
		}
		gutter, content := splitGutter(line)
		last := rows[len(rows)-1]
		last.Lines = append(last.Lines, GraphLine{Gutter: gutter, Content: content})
	}
	return rows, nil
}

// splitGutter splits the connector line into the characters of the graph and the text (e.g. "~  (elided revisions)").
func splitGutter(line string) (string, string) {
	for i, r := range line {
		if strings.ContainsRune(graphRunes, r) {
			continue
		}
		return line[:i], line[i:]
	}
	return line, ""
}

// agentRevset returns the revset of the commits whose description has the lines of the trailers of the agent. The
// lines may be outside the trailers, so the result has to be checked by CreatedBy.
func agentRevset(kind, sessionID string) string {
	return "description(" + lineRegex(TrailerAgent+": "+kind) + ") & description(" + lineRegex(TrailerAgentSession+": "+sessionID) + ")"
}

// lineRegex returns the string pattern of the revset which matches the line.
func lineRegex(line string) string {
	re := "(?m)^" + regexp.QuoteMeta(line) + "$"
	return `regex:"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(re) + `"`
}

// Show returns the commit of the revision with the whole description. rev must be a single revision.
func showArgs(rev string) []string {
	return []string{"log", "--no-graph", "--color", "never", "--ignore-working-copy", "--revisions=" + rev, "-T", showTemplate}
}

func parseShow(out []byte) (*Commit, error) {
	c := &Commit{}
	if err := json.Unmarshal(out, c); err != nil {
		return nil, xerrors.WithMessagef(err, "failed to parse the commit: %s", out)
	}
	c.RemoteBookmarks = slices.DeleteFunc(c.RemoteBookmarks, func(b RemoteBookmark) bool { return b.Remote == "git" })
	return c, nil
}

func run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	stdout, _, err := runWithStderr(ctx, dir, args...)
	return stdout, err
}

func runWithStderr(ctx context.Context, dir string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, "jj", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, nil, xerrors.WithStack(&Error{Args: args, Stderr: stderr.String(), err: err})
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}

// The operations return the message of jj. jj writes the message (e.g. "Working copy (@) now at: ...") to stderr.
// The revisions are passed after "--" and the message is joined with "=" not to be interpreted as the options
// (e.g. "-r --config=x" is the option --config).

func newArgs(revs []string) []string {
	return append([]string{"new", "--"}, revs...)
}

func editArgs(rev string) []string {
	return []string{"edit", "--", rev}
}

func abandonArgs(revs []string) []string {
	return append([]string{"abandon", "--"}, revs...)
}

func describeArgs(rev, message string) []string {
	return []string{"describe", "--message=" + message, "--", rev}
}

func commitArgs(message string) []string {
	return []string{"commit", "--message=" + message}
}

func rebaseArgs(revs []string, source RebaseSource, destinations []string, target RebaseTarget) []string {
	args := []string{"rebase"}
	for _, v := range revs {
		args = append(args, "--"+string(source)+"="+v)
	}
	for _, v := range destinations {
		args = append(args, "--"+string(target)+"="+v)
	}
	return args
}

// squashArgs returns the arguments of jj squash. The description of the destination is kept if message is empty,
// otherwise message becomes the description. jj opens the editor without them if both have the descriptions.
func squashArgs(from []string, into, message string) []string {
	args := []string{"squash"}
	for _, v := range from {
		args = append(args, "--from="+v)
	}
	args = append(args, "--into="+into)
	if message == "" {
		return append(args, "--use-destination-message")
	}
	return append(args, "--message="+message)
}

// splitArgs returns the arguments of jj split which moves the files into the new revision before the revision.
// message is the description of the new revision. The rest keeps the original description.
func splitArgs(rev, message string, files []string) []string {
	args := []string{"split", "--revision=" + rev, "--message=" + message, "--"}
	for _, f := range files {
		args = append(args, rootFile(f))
	}
	return args
}

// restoreArgs returns the arguments of jj restore which discards the changes of the files in the revision.
func restoreArgs(rev string, files []string) []string {
	args := []string{"restore", "--changes-in=" + rev, "--"}
	for _, f := range files {
		args = append(args, rootFile(f))
	}
	return args
}

func bookmarksArgs() []string {
	return []string{"bookmark", "list", "--all-remotes", "--ignore-working-copy", "-T", bookmarkTemplate}
}

func parseBookmarks(out []byte) ([]*Bookmark, error) {
	var bookmarks []*Bookmark
	for line := range strings.Lines(string(out)) {
		b := &Bookmark{}
		if err := json.Unmarshal([]byte(line), b); err != nil {
			return nil, xerrors.WithMessagef(err, "failed to parse the bookmark: %s", line)
		}
		if b.Remote == "git" {
			continue
		}
		bookmarks = append(bookmarks, b)
	}
	return bookmarks, nil
}

// The names of the bookmarks are glob patterns for most of the commands, so the name is passed with "exact:".

func bookmarkSetArgs(name, rev string) []string {
	return []string{"bookmark", "set", "--revision=" + rev, "--", name}
}

func bookmarkMoveArgs(name, rev string) []string {
	return []string{"bookmark", "move", "--to=" + rev, "--allow-backwards", "--", "exact:" + name}
}

func bookmarkDeleteArgs(name string) []string {
	return []string{"bookmark", "delete", "--", "exact:" + name}
}

func bookmarkForgetArgs(name string) []string {
	return []string{"bookmark", "forget", "--", "exact:" + name}
}

func bookmarkTrackArgs(name, remote string) []string {
	return []string{"bookmark", "track", "--remote=exact:" + remote, "--", "exact:" + name}
}

func bookmarkUntrackArgs(name, remote string) []string {
	return []string{"bookmark", "untrack", "--remote=exact:" + remote, "--", "exact:" + name}
}

func gitFetchArgs() []string {
	return []string{"git", "fetch"}
}

// gitPushArgs returns the arguments of jj git push. It pushes the bookmarks of revsets.git-push if bookmarks is empty.
func gitPushArgs(bookmarks []string) []string {
	args := []string{"git", "push"}
	for _, v := range bookmarks {
		args = append(args, "--bookmark=exact:"+v)
	}
	return args
}

func workspaceAddArgs(name, rev, dir string) []string {
	return []string{"workspace", "add", "--name=" + name, "--revision=" + rev, "--", dir}
}

// rootFile returns the fileset which matches the file exactly. The path is relative to the root of the workspace.
func rootFile(path string) string {
	path = strings.ReplaceAll(path, `\`, `\\`)
	path = strings.ReplaceAll(path, `"`, `\"`)
	return `root-file:"` + path + `"`
}

func fileShowArgs(rev, path string) []string {
	return []string{"file", "show", "--ignore-working-copy", "--revision=" + rev, "--", rootFile(path)}
}

func fileListArgs(rev string) []string {
	return []string{"file", "list", "--ignore-working-copy", "--revision=" + rev, "-T", `json(path) ++ "\n"`}
}

// diffArgs returns the arguments of jj diff in the git format. The prefixes of the paths are fixed because the parser
// depends on them.
func diffArgs(rev string) []string {
	return []string{"diff", "--git", "--color", "never", "--ignore-working-copy", "--config", "diff.git.show-path-prefix=true", "--revisions=" + rev}
}

// parseGitDiff parses the diff in the git format.
func parseGitDiff(out string) ([]*FileDiff, error) {
	var files []*FileDiff
	var file *FileDiff
	var hunk *Hunk
	var oldLine, newLine int
	for line := range strings.Lines(out) {
		line = strings.TrimSuffix(line, "\n")
		switch {
		case strings.HasPrefix(line, "diff --git "):
			oldPath, newPath, err := parseDiffHeader(line)
			if err != nil {
				return nil, err
			}
			file = &FileDiff{Path: newPath, OldPath: oldPath}
			files = append(files, file)
			hunk = nil
			continue
		case file == nil:
			continue
		case hunk != nil && (strings.HasPrefix(line, " ") || line == ""):
			hunk.Lines = append(hunk.Lines, &DiffLine{Kind: DiffLineContext, Text: strings.TrimPrefix(line, " "), OldLine: oldLine, NewLine: newLine})
			oldLine++
			newLine++
			continue
		case hunk != nil && strings.HasPrefix(line, "+"):
			hunk.Lines = append(hunk.Lines, &DiffLine{Kind: DiffLineAdded, Text: line[1:], NewLine: newLine})
			newLine++
			file.Added++
			continue
		case hunk != nil && strings.HasPrefix(line, "-"):
			hunk.Lines = append(hunk.Lines, &DiffLine{Kind: DiffLineRemoved, Text: line[1:], OldLine: oldLine})
			oldLine++
			file.Removed++
			continue
		case hunk != nil && strings.HasPrefix(line, `\`):
			if len(hunk.Lines) > 0 {
				hunk.Lines[len(hunk.Lines)-1].NoNewline = true
			}
			continue
		}

		switch {
		case strings.HasPrefix(line, "@@ "):
			h, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			hunk = h
			file.Hunks = append(file.Hunks, hunk)
			oldLine, newLine = h.OldStart, h.NewStart
		case strings.HasPrefix(line, "new file mode"):
			file.Status = FileStatusAdded
		case strings.HasPrefix(line, "deleted file mode"):
			file.Status = FileStatusDeleted
		case strings.HasPrefix(line, "rename from "):
			file.Status = FileStatusRenamed
		case strings.HasPrefix(line, "copy from "):
			file.Status = FileStatusCopied
		case strings.HasPrefix(line, "Binary files "):
			file.Binary = true
		}
	}
	for _, v := range files {
		if v.Status != FileStatusRenamed && v.Status != FileStatusCopied {
			v.OldPath = ""
		}
	}
	return files, nil
}

// parseDiffHeader parses "diff --git a/<old> b/<new>". The paths may contain the spaces, so the header is split at
// " b/" which makes the old and the new path have the same length when the file is not renamed.
func parseDiffHeader(line string) (string, string, error) {
	rest := strings.TrimPrefix(line, "diff --git ")
	if !strings.HasPrefix(rest, "a/") {
		return "", "", xerrors.Newf("unexpected header of the diff: %s", line)
	}
	if len(rest)%2 == 1 {
		if half := (len(rest) - 1) / 2; rest[half:half+3] == " b/" && rest[2:half] == rest[half+3:] {
			return rest[2:half], rest[half+3:], nil
		}
	}
	i := strings.Index(rest, " b/")
	if i < 0 {
		return "", "", xerrors.Newf("unexpected header of the diff: %s", line)
	}
	return rest[2:i], rest[i+3:], nil
}

// parseHunkHeader parses "@@ -<start>,<lines> +<start>,<lines> @@ <section>". The number of the lines is omitted if it
// is 1.
func parseHunkHeader(line string) (*Hunk, error) {
	body, section, ok := strings.Cut(strings.TrimPrefix(line, "@@ "), " @@")
	if !ok {
		return nil, xerrors.Newf("unexpected header of the hunk: %s", line)
	}
	oldRange, newRange, ok := strings.Cut(body, " ")
	if !ok || !strings.HasPrefix(oldRange, "-") || !strings.HasPrefix(newRange, "+") {
		return nil, xerrors.Newf("unexpected header of the hunk: %s", line)
	}
	oldStart, oldLines, err := parseRange(oldRange[1:])
	if err != nil {
		return nil, err
	}
	newStart, newLines, err := parseRange(newRange[1:])
	if err != nil {
		return nil, err
	}
	return &Hunk{OldStart: oldStart, OldLines: oldLines, NewStart: newStart, NewLines: newLines, Section: strings.TrimSpace(section)}, nil
}

func parseRange(s string) (int, int, error) {
	start, lines, ok := strings.Cut(s, ",")
	if !ok {
		lines = "1"
	}
	a, err := strconv.Atoi(start)
	if err != nil {
		return 0, 0, xerrors.WithStack(err)
	}
	b, err := strconv.Atoi(lines)
	if err != nil {
		return 0, 0, xerrors.WithStack(err)
	}
	return a, b, nil
}

func diffRangeArgs(from, to string) []string {
	return []string{"diff", "--git", "--color", "never", "--ignore-working-copy", "--config", "diff.git.show-path-prefix=true", "--from=" + from, "--to=" + to}
}

func interdiffArgs(from, to string) []string {
	return []string{"interdiff", "--git", "--color", "never", "--ignore-working-copy", "--config", "diff.git.show-path-prefix=true", "--from=" + from, "--to=" + to}
}

func conflictsArgs(revset string) []string {
	return []string{"log", "--no-graph", "--ignore-working-copy", "--revisions=" + revset, "-T", conflictTemplate}
}

func operationsArgs(limit int) []string {
	args := []string{"op", "log", "--no-graph", "--ignore-working-copy", "-T", operationTemplate}
	if limit > 0 {
		args = append(args, "--limit", strconv.Itoa(limit))
	}
	return args
}

func evologArgs(rev string) []string {
	return []string{"evolog", "--no-graph", "--color", "never", "--ignore-working-copy", "--config", "diff.git.show-path-prefix=true", "--revisions=" + rev, "-T", evologTemplate}
}

func parseEvolog(out string) ([]*EvologEntry, error) {
	var entries []*EvologEntry
	for record := range strings.SplitSeq(out, "\x1D") {
		if strings.TrimSpace(record) == "" {
			continue
		}
		head, diff, _ := strings.Cut(record, "\x1E")
		e := &EvologEntry{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(head)), e); err != nil {
			return nil, xerrors.WithMessagef(err, "failed to parse the entry: %s", head)
		}
		files, err := parseGitDiff(diff)
		if err != nil {
			return nil, err
		}
		e.Files = files
		entries = append(entries, e)
	}
	return entries, nil
}

func parseAliases(out []byte) ([]*Alias, error) {
	var aliases []*Alias
	for line := range strings.Lines(string(out)) {
		var v struct {
			Name  string          `json:"name"`
			Value json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			return nil, xerrors.WithMessagef(err, "failed to parse the alias: %s", line)
		}
		var args []string
		// The alias which is not the list of the arguments (e.g. the table of the subcommands) is ignored.
		if err := json.Unmarshal(v.Value, &args); err != nil {
			continue
		}
		aliases = append(aliases, &Alias{Name: strings.TrimPrefix(v.Name, "aliases."), Args: args})
	}
	return aliases, nil
}
