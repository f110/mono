package server

import (
	"errors"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/codexel/config"
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
	h := NewRepositoryHandler(conf, nil)

	res, err := h.ListRepositories(t.Context(), connect.NewRequest(&api.RequestListRepositories{}))
	assertion.MustNoError(t, err)
	assertion.MustLen(t, res.Msg.GetRepositories(), 2)
	assertion.Equal(t, res.Msg.GetRepositories()[0].GetPath(), "/src/mono")
	assertion.Equal(t, res.Msg.GetRepositories()[0].GetName(), "mono")

	_, err = h.CloseRepository(t.Context(), connect.NewRequest(api.RequestCloseRepository_builder{Path: new("/src/mono")}.Build()))
	assertion.MustNoError(t, err)
	c, err := conf.Load()
	assertion.MustNoError(t, err)
	assertion.MustLen(t, c.Repositories, 1)
	assertion.Equal(t, c.Repositories[0].Path, "/src/ops")
}

func TestLogRejectsRepositoryNotOpened(t *testing.T) {
	h := NewRevisionHandler(newConfig(t, "/src/mono"))

	_, err := h.Log(t.Context(), connect.NewRequest(api.RequestLog_builder{Path: new("/etc")}.Build()))
	assertion.MustError(t, err)
	connectErr, ok := errors.AsType[*connect.Error](err)
	assertion.True(t, ok)
	assertion.Equal(t, connectErr.Code(), connect.CodeNotFound)
}

func TestOperationRejectsRepositoryNotOpened(t *testing.T) {
	h := NewRevisionHandler(newConfig(t, "/src/mono"))

	_, err := h.Undo(t.Context(), connect.NewRequest(api.RequestUndo_builder{Path: new("/etc")}.Build()))
	assertion.MustError(t, err)
	connectErr, ok := errors.AsType[*connect.Error](err)
	assertion.True(t, ok)
	assertion.Equal(t, connectErr.Code(), connect.CodeNotFound)
}
