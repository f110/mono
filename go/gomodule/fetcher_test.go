package gomodule

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestModuleRoot(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	assertion.MustNoError(t, err)
	wt, err := repo.Worktree()
	assertion.MustNoError(t, err)

	addFile(t, wt, dir, "go.mod", []byte("module github.com/f110/gomodule-proxy-test"))
	addFile(t, wt, dir, "const.go", []byte("package proxy\n\nconst Foo = \"bar\""))
	addFile(t, wt, dir, "pkg/api/go.mod", []byte("module github.com/f110/gomodule-proxy-test/pkg/api"))
	addFile(t, wt, dir, "pkg/api/const2.go", []byte("package api\n\nconst Baz = \"foo\""))

	commitHash, err := wt.Commit("init", &git.CommitOptions{
		Author: &object.Signature{
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	assertion.MustNoError(t, err)
	_, err = repo.CreateTag("v1.0.0", commitHash, &git.CreateTagOptions{
		Tagger: &object.Signature{
			Email: "test@example.com",
			When:  time.Now(),
		},
		Message: "v1.0.0",
	})
	assertion.MustNoError(t, err)
	_, err = repo.CreateTag("pkg/api/v1.5.0", commitHash, &git.CreateTagOptions{
		Tagger: &object.Signature{
			Email: "test@example.com",
			When:  time.Now(),
		},
		Message: "pkg/api/v1.5.0",
	})
	assertion.MustNoError(t, err)

	vcsRepo := NewVCS("git", "", "", nil, nil)
	assertion.MustNoError(t, err)
	vcsRepo.synced = true
	err = vcsRepo.Open(dir)
	assertion.MustNoError(t, err)
	moduleRoot := &ModuleRoot{
		dir:      dir,
		RootPath: "github.com/f110/gomodule-proxy-test",
		vcs:      vcsRepo,
	}
	modules, err := moduleRoot.findModules(context.Background())
	assertion.MustNoError(t, err)
	moduleRoot.Modules = modules
	err = moduleRoot.findVersions()
	assertion.MustNoError(t, err)

	assertion.MustLen(t, modules, 2)
	for _, v := range modules {
		var vers []string
		for _, ver := range v.Versions {
			vers = append(vers, ver.Semver)
		}
		switch v.Path {
		case "github.com/f110/gomodule-proxy-test":
			assert.ElementsMatch(t, []string{"v1.0.0"}, vers)
		case "github.com/f110/gomodule-proxy-test/pkg/api":
			assert.ElementsMatch(t, []string{"v1.5.0"}, vers)
		}
	}

	buf := new(bytes.Buffer)
	err = moduleRoot.Archive(context.Background(), buf, "github.com/f110/gomodule-proxy-test/pkg/api", "v1.5.0", false)
	assertion.MustNoError(t, err)
	zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), 4096)
	assertion.MustNoError(t, err)
	var files []string
	for _, v := range zipReader.File {
		files = append(files, v.Name)
	}
	buf.Reset()
	assert.ElementsMatch(t, []string{
		"github.com/f110/gomodule-proxy-test/pkg/api@v1.5.0/go.mod",
		"github.com/f110/gomodule-proxy-test/pkg/api@v1.5.0/const2.go",
	}, files)

	err = moduleRoot.Archive(context.Background(), buf, "github.com/f110/gomodule-proxy-test", "v1.0.0", false)
	assertion.MustNoError(t, err)
	zipReader, err = zip.NewReader(bytes.NewReader(buf.Bytes()), 4096)
	assertion.MustNoError(t, err)
	files = []string{}
	for _, v := range zipReader.File {
		files = append(files, v.Name)
	}
	assert.ElementsMatch(t, []string{
		"github.com/f110/gomodule-proxy-test@v1.0.0/go.mod",
		"github.com/f110/gomodule-proxy-test@v1.0.0/const.go",
	}, files)
}

func addFile(t *testing.T, wt *git.Worktree, dir, filename string, buf []byte) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(filepath.Join(dir, filename)), 0755)
	assertion.MustNoError(t, err)
	err = os.WriteFile(filepath.Join(dir, filename), buf, 0644)
	assertion.NoError(t, err)
	_, err = wt.Add(filename)
	assertion.NoError(t, err)
}
