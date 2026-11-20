package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/codexel/config"
	"go.f110.dev/mono/go/jujutsu"
	"go.f110.dev/mono/go/testing/assertion"
)

func newConfig(t *testing.T, paths ...string) *config.File {
	t.Helper()

	conf := config.NewFile(filepath.Join(t.TempDir(), "config.json"))
	err := conf.Update(func(c *config.Config) error {
		for _, v := range paths {
			c.Repositories = append(c.Repositories, &config.Repository{Path: v})
		}
		return nil
	})
	assertion.MustNoError(t, err)
	return conf
}

func TestListAndCloseRepositories(t *testing.T) {
	conf := newConfig(t, "/src/mono", "/src/ops")
	h := NewRepositoryHandler(conf)

	res, err := h.List(t.Context(), connect.NewRequest(&api.RequestListRepositories{}))
	assertion.MustNoError(t, err)
	assertion.MustLen(t, res.Msg.GetRepositories(), 2)
	assertion.Equal(t, res.Msg.GetRepositories()[0].GetPath(), "/src/mono")
	assertion.Equal(t, res.Msg.GetRepositories()[0].GetName(), "mono")

	_, err = h.Close(t.Context(), connect.NewRequest(api.RequestCloseRepository_builder{Path: new("/src/mono")}.Build()))
	assertion.MustNoError(t, err)
	c, err := conf.Load()
	assertion.MustNoError(t, err)
	assertion.MustLen(t, c.Repositories, 1)
	assertion.Equal(t, c.Repositories[0].Path, "/src/ops")
}

func TestLogRejectsRepositoryNotOpened(t *testing.T) {
	h := NewRepositoryHandler(newConfig(t, "/src/mono"))

	_, err := h.Log(t.Context(), connect.NewRequest(api.RequestLog_builder{Path: new("/etc")}.Build()))
	assertion.MustError(t, err)
	connectErr, ok := errors.AsType[*connect.Error](err)
	assertion.True(t, ok)
	assertion.Equal(t, connectErr.Code(), connect.CodeNotFound)
}

func TestOperationRejectsRepositoryNotOpened(t *testing.T) {
	h := NewRepositoryHandler(newConfig(t, "/src/mono"))

	_, err := h.Undo(t.Context(), connect.NewRequest(api.RequestUndo_builder{Path: new("/etc")}.Build()))
	assertion.MustError(t, err)
	connectErr, ok := errors.AsType[*connect.Error](err)
	assertion.True(t, ok)
	assertion.Equal(t, connectErr.Code(), connect.CodeNotFound)
}

func TestRemoveWorkspaceRejectsOpenedWorkspace(t *testing.T) {
	if *jjBinaryPath == "" {
		t.Skipf("Skip %s because -test.jj-binary is not set", t.Name())
	}

	repo, _ := newStackRepository(t)
	workspace := repo + "-x"
	runCommand(t, repo, "jj", "workspace", "add", "--name=x", workspace)
	h := NewRepositoryHandler(newConfig(t, repo, workspace))

	_, err := h.RemoveWorkspace(t.Context(), connect.NewRequest(api.RequestRemoveRepositoryWorkspace_builder{Path: new(repo), Name: new("x")}.Build()))
	assertion.MustError(t, err)
	connectErr, ok := errors.AsType[*connect.Error](err)
	assertion.True(t, ok)
	assertion.Equal(t, connectErr.Code(), connect.CodeFailedPrecondition)
	_, err = os.Stat(workspace)
	assertion.MustNoError(t, err)
}

func TestGetFileRejectsPathOutsideRepository(t *testing.T) {
	h := NewRepositoryHandler(newConfig(t, "/src/mono"))

	for _, file := range []string{"../ops/secret", "go/../../ops/secret", "/etc/passwd", ".."} {
		t.Run(file, func(t *testing.T) {
			_, err := h.GetFile(t.Context(), connect.NewRequest(api.RequestGetFile_builder{Path: new("/src/mono"), Revision: new("@"), File: new(file)}.Build()))
			assertion.MustError(t, err)
			connectErr, ok := errors.AsType[*connect.Error](err)
			assertion.True(t, ok)
			assertion.Equal(t, connectErr.Code(), connect.CodeInvalidArgument)
			assertion.Contains(t, connectErr.Message(), "outside of the repository")
		})
	}
}

func TestListAgentCommitsRequiresAgent(t *testing.T) {
	h := NewRepositoryHandler(newConfig(t, "/src/mono"))

	_, err := h.ListAgentCommits(t.Context(), connect.NewRequest(api.RequestListAgentCommits_builder{Cwd: new("/src/mono"), Kind: new("claude")}.Build()))
	assertion.MustError(t, err)
	connectErr, ok := errors.AsType[*connect.Error](err)
	assertion.True(t, ok)
	assertion.Equal(t, connectErr.Code(), connect.CodeInvalidArgument)
}

func TestOpenAgentRepositories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	conf := newConfig(t, "/src/mono")
	roots := map[string]string{"/src/mono/go": "/src/mono", "/src/ops": "/src/ops", "/src/ops/manifests": "/src/ops", home: home}
	workspaceRoot := func(_ context.Context, dir string) (string, error) {
		if v, ok := roots[dir]; ok {
			return v, nil
		}
		return "", errors.New("not a workspace")
	}

	err := openAgentRepositories(t.Context(), conf, []string{"/src/mono/go", "/src/ops/manifests", "/src/ops", "/tmp", home}, workspaceRoot)
	assertion.MustNoError(t, err)
	c, err := conf.Load()
	assertion.MustNoError(t, err)
	assertion.MustLen(t, c.Repositories, 2)
	assertion.Equal(t, c.Repositories[0].Path, "/src/mono")
	assertion.Equal(t, c.Repositories[1].Path, "/src/ops")
}

func TestToCommit(t *testing.T) {
	c := toCommit(&jujutsu.Commit{ChangeID: "abc", WorkingCopies: []string{"default", "second"}})
	assertion.Equal(t, c.GetChangeId(), "abc")
	assertion.Equal(t, c.GetWorkingCopies(), []string{"default", "second"})
}

func TestToConflictedFiles(t *testing.T) {
	files := toConflictedFiles([]jujutsu.ConflictedFile{{Path: "a.txt", Sides: 2}, {Path: "b.txt", Sides: 3}})
	assertion.MustLen(t, files, 2)
	assertion.Equal(t, files[0].GetPath(), "a.txt")
	assertion.Equal(t, files[0].GetSides(), int32(2))
	assertion.Equal(t, files[1].GetPath(), "b.txt")
	assertion.Equal(t, files[1].GetSides(), int32(3))
}
