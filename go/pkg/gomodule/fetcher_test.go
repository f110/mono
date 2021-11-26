package gomodule

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleRoot(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	wt, err := repo.Worktree()
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/f110/gomodule-proxy-test"), 0644)
	require.NoError(t, err)
	_, err = wt.Add("go.mod")
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, "const.go"), []byte("package proxy\n\nconst Foo = \"bar\""), 0644)
	require.NoError(t, err)
	_, err = wt.Add("const.go")
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(dir, "pkg/api"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "pkg/api/go.mod"), []byte("module github.com/f110/gomodule-proxy-test/pkg/api"), 0644)
	require.NoError(t, err)
	_, err = wt.Add("pkg/api/go.mod")
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, "pkg/api/const2.go"), []byte("package api\n\nconst Baz = \"foo\""), 0644)
	require.NoError(t, err)
	_, err = wt.Add("pkg/api/const2.go")
	require.NoError(t, err)

	commitHash, err := wt.Commit("init", &git.CommitOptions{
		Author: &object.Signature{
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)
	_, err = repo.CreateTag("v1.0.0", commitHash, &git.CreateTagOptions{
		Tagger: &object.Signature{
			Email: "test@example.com",
			When:  time.Now(),
		},
		Message: "v1.0.0",
	})
	require.NoError(t, err)
	_, err = repo.CreateTag("pkg/api/v1.5.0", commitHash, &git.CreateTagOptions{
		Tagger: &object.Signature{
			Email: "test@example.com",
			When:  time.Now(),
		},
		Message: "pkg/api/v1.5.0",
	})
	require.NoError(t, err)

	vcsRepo := NewVCS("git", "")
	err = vcsRepo.Open(dir)
	require.NoError(t, err)
	moduleRoot := &ModuleRoot{
		dir:      dir,
		RootPath: "github.com/f110/gomodule-proxy-test",
		vcs:      vcsRepo,
	}
	modules, err := moduleRoot.findModules()
	require.NoError(t, err)
	moduleRoot.Modules = modules
	err = moduleRoot.findVersions()
	require.NoError(t, err)

	for _, v := range modules {
		var vers []string
		for _, ver := range v.Versions {
			vers = append(vers, ver.Semver)
		}
		t.Logf("%s: %v", v.Path, vers)
		switch v.Path {
		case "github.com/f110/gomodule-proxy-test":
			assert.ElementsMatch(t, []string{"v1.0.0"}, vers)
		case "github.com/f110/gomodule-proxy-test/pkg/api":
			assert.ElementsMatch(t, []string{"v1.5.0"}, vers)
		}
	}

	buf := new(bytes.Buffer)
	err = moduleRoot.Archive(buf, "github.com/f110/gomodule-proxy-test/pkg/api", "v1.5.0")
	require.NoError(t, err)
	zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), 4096)
	require.NoError(t, err)
	var files []string
	for _, v := range zipReader.File {
		files = append(files, v.Name)
	}
	buf.Reset()
	assert.ElementsMatch(t, []string{
		"github.com/f110/gomodule-proxy-test/pkg/api@v1.5.0/go.mod",
		"github.com/f110/gomodule-proxy-test/pkg/api@v1.5.0/const2.go",
	}, files)

	err = moduleRoot.Archive(buf, "github.com/f110/gomodule-proxy-test", "v1.0.0")
	require.NoError(t, err)
	zipReader, err = zip.NewReader(bytes.NewReader(buf.Bytes()), 4096)
	require.NoError(t, err)
	files = []string{}
	for _, v := range zipReader.File {
		files = append(files, v.Name)
	}
	assert.ElementsMatch(t, []string{
		"github.com/f110/gomodule-proxy-test@v1.0.0/go.mod",
		"github.com/f110/gomodule-proxy-test@v1.0.0/const.go",
	}, files)
}
