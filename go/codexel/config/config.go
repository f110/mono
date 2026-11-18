// Package config provides the config file of Codexel.
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"go.f110.dev/xerrors"
)

type Repository struct {
	// Path is the root of the workspace.
	Path string `json:"path"`
}

type Config struct {
	// Repositories are the repositories opened at startup.
	Repositories []*Repository `json:"repositories"`
	// AutoViewedPatterns are the patterns of the files marked as viewed when the review starts and the round starts.
	// e.g. "*.lock" matches the name, "gen/" matches the files under the directory and "go/*.pb.go" matches the path.
	AutoViewedPatterns []string `json:"auto_viewed_patterns,omitempty"`
}

// File is the config file. The changes are serialized and written atomically.
type File struct {
	path string
	mu   sync.Mutex
}

// DefaultPath returns ~/.config/codexel/config.json.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	return filepath.Join(home, ".config", "codexel", "config.json"), nil
}

func NewFile(path string) *File {
	return &File{path: path}
}

// Load returns the config. It returns the empty config if the file doesn't exist.
func (f *File) Load() (*Config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.load()
}

// Update applies fn to the config and writes it.
func (f *File) Update(fn func(*Config) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	conf, err := f.load()
	if err != nil {
		return err
	}
	if err := fn(conf); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return xerrors.WithStack(err)
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0755); err != nil {
		return xerrors.WithStack(err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(f.path), ".config-*.json")
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

func (f *File) load() (*Config, error) {
	buf, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	conf := &Config{}
	if err := json.Unmarshal(buf, conf); err != nil {
		return nil, xerrors.WithMessagef(err, "failed to parse %s", f.path)
	}
	return conf, nil
}
