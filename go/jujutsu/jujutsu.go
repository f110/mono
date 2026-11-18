// Package jujutsu provides the operations of the repositories of Jujutsu by the jj command.
package jujutsu

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"go.f110.dev/xerrors"
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

// fieldSeparator separates the gutter from the commit in the output of jj log.
const fieldSeparator = "\x1F"

// logTemplate outputs the commit as JSON after fieldSeparator. The lists are built by join because jj can't
// serialize the list of the templates.
const logTemplate = `"\x1F{" ++ ` +
	`"\"change_id\":" ++ json(change_id.short()) ++ ` +
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
	`",\"trailers\":[" ++ trailers.map(|x| "[" ++ json(x.key()) ++ "," ++ json(x.value()) ++ "]").join(",") ++ "]" ++ ` +
	`"}\n"`

// logArgs returns the arguments of jj log. The style of the graph is fixed because the parser depends on it.
// --ignore-working-copy avoids taking the snapshot of the working copy on every read.
func logArgs(revset string, limit int) []string {
	args := []string{"log", "--color", "never", "--quiet", "--ignore-working-copy", "--config", "ui.graph.style=curved"}
	if revset != "" {
		args = append(args, "-r", revset)
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

const graphRunes = " │─╮╯╭╰├┤┬┴┼~"

func (r *Repository) Log(ctx context.Context, revset string, limit int) ([]*LogRow, error) {
	out, err := run(ctx, r.root, logArgs(revset, limit)...)
	if err != nil {
		return nil, err
	}
	return parseLog(out)
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

func run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "jj", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, xerrors.WithStack(&Error{Args: args, Stderr: stderr.String(), err: err})
	}
	return stdout.Bytes(), nil
}
