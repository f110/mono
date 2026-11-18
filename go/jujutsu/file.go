package jujutsu

import (
	"context"
	"encoding/json"
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
