package jujutsu

import (
	"context"
	"encoding/json"
	"strings"

	"go.f110.dev/xerrors"
)

type Workspace struct {
	Name string `json:"name"`
	// Target is the commit ID of the working copy of the workspace.
	Target string `json:"target"`
	Root   string `json:"root"`
}

const workspaceTemplate = `"{\"name\":" ++ json(name) ++ ",\"target\":" ++ json(target.commit_id().short()) ++ ",\"root\":" ++ json(self.root()) ++ "}\n"`

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

func workspaceAddArgs(name, rev, dir string) []string {
	return []string{"workspace", "add", "--name=" + name, "--revision=" + rev, "--", dir}
}

// WorkspaceAdd creates the workspace in dir whose working copy is on the revision.
func (r *Repository) WorkspaceAdd(ctx context.Context, name, rev, dir string) (string, error) {
	return r.mutate(ctx, workspaceAddArgs(name, rev, dir)...)
}

// WorkspaceForget stops tracking the workspace. The directory is left.
func (r *Repository) WorkspaceForget(ctx context.Context, name string) (string, error) {
	return r.mutate(ctx, "workspace", "forget", "--", name)
}

// WorkspaceRename renames the workspace of the repository.
func (r *Repository) WorkspaceRename(ctx context.Context, name string) (string, error) {
	return r.mutate(ctx, "workspace", "rename", "--", name)
}

// WorkspaceUpdateStale updates the stale working copy of the workspace.
func (r *Repository) WorkspaceUpdateStale(ctx context.Context) (string, error) {
	return r.mutate(ctx, "workspace", "update-stale")
}
