// Package pullrequest provides the record of the pull requests submitted by Codexel.
package pullrequest

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"go.f110.dev/xerrors"
)

// PullRequest is the pull request of the change. Repository is the root of the workspace.
type PullRequest struct {
	Repository string `json:"repository"`
	ChangeID   string `json:"change_id"`
	URL        string `json:"url"`
}

// File is the file of the pull requests. The changes are serialized and written atomically.
type File struct {
	path string
	mu   sync.Mutex
}

// DefaultPath returns ~/.config/codexel/pullrequests.json.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	return filepath.Join(home, ".config", "codexel", "pullrequests.json"), nil
}

func NewFile(path string) *File {
	return &File{path: path}
}

// List returns the pull requests of the repository in order of saving.
func (f *File) List(repository string) ([]*PullRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	doc, err := f.load()
	if err != nil {
		return nil, err
	}
	var prs []*PullRequest
	for _, v := range doc.PullRequests {
		if v.Repository == repository {
			prs = append(prs, v)
		}
	}
	return prs, nil
}

// Save adds the pull requests. The pull request of the same change in the same repository is replaced.
func (f *File) Save(prs []*PullRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	doc, err := f.load()
	if err != nil {
		return err
	}
	for _, v := range prs {
		doc.PullRequests = slices.DeleteFunc(doc.PullRequests, func(p *PullRequest) bool {
			return p.Repository == v.Repository && p.ChangeID == v.ChangeID
		})
		doc.PullRequests = append(doc.PullRequests, v)
	}
	buf, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return xerrors.WithStack(err)
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0755); err != nil {
		return xerrors.WithStack(err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(f.path), ".pullrequests-*.json")
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(buf, '\n')); err != nil {
		tmp.Close()
		return xerrors.WithStack(err)
	}
	if err := tmp.Close(); err != nil {
		return xerrors.WithStack(err)
	}
	if err := os.Rename(tmp.Name(), f.path); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (f *File) load() (*document, error) {
	buf, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return &document{}, nil
	}
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	doc := &document{}
	if err := json.Unmarshal(buf, doc); err != nil {
		return nil, xerrors.WithMessagef(err, "failed to parse %s", f.path)
	}
	return doc, nil
}

type document struct {
	PullRequests []*PullRequest `json:"pull_requests"`
}
