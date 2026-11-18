package jujutsu

import (
	"context"
	"encoding/json"
	"strings"

	"go.f110.dev/xerrors"
)

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

const conflictTemplate = `"{" ++ ` +
	`"\"change_id\":" ++ json(change_id.short()) ++ ` +
	`",\"commit_id\":" ++ json(commit_id.short()) ++ ` +
	`",\"description\":" ++ json(description.first_line()) ++ ` +
	`",\"working_copy\":" ++ json(current_working_copy) ++ ` +
	`",\"files\":[" ++ self.conflicted_files().map(|f| "{\"path\":" ++ json(f.path()) ++ ",\"sides\":" ++ f.conflict_side_count() ++ "}").join(",") ++ "]" ++ ` +
	`"}\n"`

func conflictsArgs() []string {
	return []string{"log", "--no-graph", "--ignore-working-copy", "--revisions=conflicts() & mutable()", "-T", conflictTemplate}
}

// Conflicts returns the mutable commits which have the conflicts.
func (r *Repository) Conflicts(ctx context.Context) ([]*ConflictedCommit, error) {
	out, err := run(ctx, r.root, conflictsArgs()...)
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

// conflictFileArgs returns the arguments to read the conflicted file. The markers are the snapshot style which has the
// whole content of each side, so the sides can be extracted from it.
func conflictFileArgs(rev, path string) []string {
	return []string{"file", "show", "--ignore-working-copy", "--config", "ui.conflict-marker-style=snapshot", "--revision=" + rev, "--", rootFile(path)}
}

// ConflictFile returns the content of the conflicted file with the markers of the snapshot style.
func (r *Repository) ConflictFile(ctx context.Context, rev, path string) ([]byte, error) {
	return run(ctx, r.root, conflictFileArgs(rev, path)...)
}

// ResolveSide is the side to take for the conflict of two sides.
type ResolveSide string

const (
	ResolveOurs   ResolveSide = ":ours"
	ResolveTheirs ResolveSide = ":theirs"
)

func resolveArgs(rev, path string, side ResolveSide) []string {
	return []string{"resolve", "--revision=" + rev, "--tool=" + string(side), "--", rootFile(path)}
}

// Resolve resolves the conflict of two sides by taking one side.
func (r *Repository) Resolve(ctx context.Context, rev, path string, side ResolveSide) (string, error) {
	return r.mutate(ctx, resolveArgs(rev, path, side)...)
}
