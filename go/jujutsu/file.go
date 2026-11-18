package jujutsu

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"go.f110.dev/xerrors"
)

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

// WriteFile writes the file of the working copy and snapshots it. The path must be in the workspace and must not be in
// the directories of jj and git.
func (r *Repository) WriteFile(ctx context.Context, path string, content []byte) error {
	target, err := r.workingCopyPath(path)
	if err != nil {
		return err
	}
	// The symbolic link in the workspace may point to the outside.
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return xerrors.WithStack(err)
	}
	root, err := filepath.EvalSymlinks(r.root)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if !strings.HasPrefix(resolved, root+string(filepath.Separator)) {
		return xerrors.Newf("the path is outside of the workspace: %s", path)
	}
	fi, err := os.Stat(resolved)
	if err != nil {
		return xerrors.WithStack(err)
	}
	target = resolved
	if err := os.WriteFile(target, content, fi.Mode().Perm()); err != nil {
		return xerrors.WithStack(err)
	}
	return r.Snapshot(ctx)
}

// workingCopyPath returns the absolute path of the file in the working copy. It rejects the path outside of the
// workspace and the path in .jj and .git.
func (r *Repository) workingCopyPath(path string) (string, error) {
	if path == "" || filepath.IsAbs(path) {
		return "", xerrors.Newf("invalid path: %s", path)
	}
	clean := filepath.Clean(path)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", xerrors.Newf("the path is outside of the workspace: %s", path)
	}
	first := strings.Split(clean, string(filepath.Separator))[0]
	if first == ".jj" || first == ".git" {
		return "", xerrors.Newf("the path is in the directory of the repository: %s", path)
	}
	return filepath.Join(r.root, clean), nil
}
