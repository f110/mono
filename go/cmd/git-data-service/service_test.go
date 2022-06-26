package main

import (
	"context"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goGit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"go.f110.dev/mono/go/pkg/git"
	"go.f110.dev/mono/go/pkg/storage"
)

func TestListRepositories(t *testing.T) {
	mockStorage := storage.NewMock()
	conn := startServer(t, mockStorage, map[string]*goGit.Repository{"test/test1": makeSourceRepository(t), "test/test2": makeSourceRepository(t)})

	gitData := git.NewGitDataClient(conn)
	res, err := gitData.ListRepositories(context.Background(), &git.RequestListRepositories{})
	require.NoError(t, err)
	assert.Len(t, res.Repositories, 2)
}

func startServer(t *testing.T, st *storage.Mock, repos map[string]*goGit.Repository) *grpc.ClientConn {
	var repositories []repository
	for k, repo := range repos {
		_, name := filepath.Split(k)
		registerToStorage(t, st, repo, k)
		repositories = append(repositories, repository{Name: name, Prefix: k})
	}

	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	svc, err := newService(st, repositories)
	require.NoError(t, err)
	git.RegisterGitDataServer(s, svc)
	go func() {
		if err := s.Serve(lis); err != nil {
			require.NoError(t, err)
		}
	}()
	t.Cleanup(func() {
		s.Stop()
	})

	conn, err := grpc.Dial("bufnet", grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
		return lis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	return conn
}

func makeSourceRepository(t *testing.T) *goGit.Repository {
	// Make new git repository
	repoDir := t.TempDir()
	repo, err := goGit.PlainInit(repoDir, false)
	require.NoError(t, err)
	wt, err := repo.Worktree()
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("Hello"), 0644)
	require.NoError(t, err)
	_, err = wt.Add("README.md")
	require.NoError(t, err)
	_, err = wt.Commit("Init", &goGit.CommitOptions{
		Author:    &object.Signature{Name: t.Name(), When: time.Now(), Email: "test@localhost"},
		Committer: &object.Signature{Name: t.Name(), When: time.Now(), Email: "test@localhost"},
	})
	require.NoError(t, err)

	return repo
}

func registerToStorage(t *testing.T, s *storage.Mock, repo *goGit.Repository, prefix string) {
	gitDir := repo.Storer.(*filesystem.Storage).Filesystem().Root()
	err := filepath.Walk(gitDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			t.Log(err)
		}
		if info.IsDir() {
			return nil
		}
		name := strings.TrimPrefix(path, gitDir+"/")
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		s.AddTree(filepath.Join(prefix, name), data)
		return nil
	})
	require.NoError(t, err)
}
