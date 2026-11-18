package config

import (
	"path/filepath"
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestFile(t *testing.T) {
	f := NewFile(filepath.Join(t.TempDir(), "codexel", "config.json"))

	conf, err := f.Load()
	assertion.MustNoError(t, err)
	assertion.Len(t, conf.Repositories, 0)

	err = f.Update(func(c *Config) error {
		c.Repositories = append(c.Repositories, &Repository{Path: "/repo"})
		return nil
	})
	assertion.MustNoError(t, err)

	conf, err = f.Load()
	assertion.MustNoError(t, err)
	assertion.MustLen(t, conf.Repositories, 1)
	assertion.Equal(t, conf.Repositories[0].Path, "/repo")
}
