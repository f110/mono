package sidecar

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	goGit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"go.f110.dev/mono/go/git"
)

func TestExportToDir(t *testing.T) {
	repo, head := makeRepository(t)
	client := git.NewGitDataClient(startGitDataService(t, map[string]*goGit.Repository{"test-repo": repo}))

	dir := t.TempDir()
	err := exportToDir(context.Background(), client, "test-repo", head, dir)
	require.NoError(t, err)

	cases := map[string]string{
		"README.md":             "Hello",
		"docs/README.md":        "docs",
		"docs/design/README.md": "design",
	}
	for name, want := range cases {
		got, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		assert.Equal(t, want, string(got))
	}
}

func makeRepository(t *testing.T) (*goGit.Repository, string) {
	repoDir := t.TempDir()
	repo, err := goGit.PlainInit(repoDir, false)
	require.NoError(t, err)
	wt, err := repo.Worktree()
	require.NoError(t, err)

	files := map[string]string{
		"README.md":             "Hello",
		"docs/README.md":        "docs",
		"docs/design/README.md": "design",
	}
	for name, content := range files {
		require.NoError(t, os.MkdirAll(filepath.Join(repoDir, filepath.Dir(name)), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, name), []byte(content), 0644))
		_, err = wt.Add(name)
		require.NoError(t, err)
	}

	commit, err := wt.Commit("Init", &goGit.CommitOptions{
		Author:    &object.Signature{Name: t.Name(), Email: "test@localhost", When: time.Now()},
		Committer: &object.Signature{Name: t.Name(), Email: "test@localhost", When: time.Now()},
	})
	require.NoError(t, err)

	return repo, commit.String()
}

func startGitDataService(t *testing.T, repos map[string]*goGit.Repository) *grpc.ClientConn {
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	svc, err := git.NewDataServiceWithGoGit(repos)
	require.NoError(t, err)
	git.RegisterGitDataServer(s, svc)
	go func() {
		if err := s.Serve(lis); err != nil {
			t.Log(err)
		}
	}()
	t.Cleanup(s.Stop)

	conn, err := grpc.NewClient("passthrough://bufnet", grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
		return lis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	return conn
}
