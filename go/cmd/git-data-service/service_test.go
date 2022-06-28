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
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
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

func TestListReferences(t *testing.T) {
	mockStorage := storage.NewMock()
	repo := makeSourceRepository(t)
	conn := startServer(t, mockStorage, map[string]*goGit.Repository{"test/test1": repo})
	gitData := git.NewGitDataClient(conn)

	res, err := gitData.ListReferences(context.Background(), &git.RequestListReferences{Repo: "test1"})
	require.NoError(t, err)
	assert.Len(t, res.Refs, 2)

	v, _ := repo.References()
	var expectRefs []string
	err = v.ForEach(func(ref *plumbing.Reference) error {
		expectRefs = append(expectRefs, ref.Name().String())
		return nil
	})
	require.NoError(t, err)
	refs := make(map[string]*git.Reference)
	for _, v := range res.Refs {
		refs[v.Name] = v
	}
	for _, expectRef := range expectRefs {
		assert.Contains(t, refs, expectRef)
	}
	assert.Equal(t, "refs/heads/master", refs["HEAD"].Target)
}

func TestGetCommit(t *testing.T) {
	mockStorage := storage.NewMock()
	repo := makeSourceRepository(t)
	conn := startServer(t, mockStorage, map[string]*goGit.Repository{"test/test1": repo})
	gitData := git.NewGitDataClient(conn)

	ref, err := repo.Reference(plumbing.NewBranchReferenceName("master"), false)
	require.NoError(t, err)
	commit, err := gitData.GetCommit(context.Background(), &git.RequestGetCommit{Repo: "test1", Sha: ref.Hash().String()})
	require.NoError(t, err)
	assert.Equal(t, ref.Hash().String(), commit.Commit.Sha)
	assert.NotEmpty(t, commit.Commit.Tree)
	assert.NotEmpty(t, commit.Commit.Message)
	assert.NotNil(t, commit.Commit.Author)
	assert.NotNil(t, commit.Commit.Committer)
}

func TestGetTree(t *testing.T) {
	mockStorage := storage.NewMock()
	repo := makeSourceRepository(t)
	conn := startServer(t, mockStorage, map[string]*goGit.Repository{"test/test1": repo})
	gitData := git.NewGitDataClient(conn)

	ref, err := repo.Reference(plumbing.NewBranchReferenceName("master"), false)
	require.NoError(t, err)
	tree, err := gitData.GetTree(context.Background(), &git.RequestGetTree{Repo: "test1", Sha: ref.Hash().String()})
	require.NoError(t, err)
	assert.Equal(t, ref.Hash().String(), tree.Sha)
	files := make(map[string]*git.TreeEntry)
	for _, v := range tree.Tree {
		files[v.Path] = v
	}
	assert.Contains(t, files, "README.md")
}

func TestGetBlob(t *testing.T) {
	mockStorage := storage.NewMock()
	repo := makeSourceRepository(t)
	conn := startServer(t, mockStorage, map[string]*goGit.Repository{"test/test1": repo})
	gitData := git.NewGitDataClient(conn)

	ref, err := repo.Reference(plumbing.NewBranchReferenceName("master"), false)
	require.NoError(t, err)
	commit, err := repo.CommitObject(ref.Hash())
	require.NoError(t, err)
	tree, err := commit.Tree()
	require.NoError(t, err)

	var blobHash, expectContent string
	err = tree.Files().ForEach(func(file *object.File) error {
		if file.Name == "README.md" {
			blobHash = file.Hash.String()
			expectContent, err = file.Contents()
			if err != nil {
				return err
			}
			return storer.ErrStop
		}
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, blobHash)

	blob, err := gitData.GetBlob(context.Background(), &git.RequestGetBlob{Repo: "test1", Sha: blobHash})
	require.NoError(t, err)
	assert.Equal(t, blobHash, blob.Sha)
	assert.Equal(t, string(blob.Content), expectContent)
}

func TestListTag(t *testing.T) {
	mockStorage := storage.NewMock()
	repo := makeSourceRepository(t)
	conn := startServer(t, mockStorage, map[string]*goGit.Repository{"test/test1": repo})
	gitData := git.NewGitDataClient(conn)

	masterRef, err := repo.Reference(plumbing.NewBranchReferenceName("master"), false)
	require.NoError(t, err)
	_, err = repo.CreateTag("tag1", masterRef.Hash(), nil)
	require.NoError(t, err)

	tags, err := gitData.ListTag(context.Background(), &git.RequestListTag{Repo: "test1"})
	require.NoError(t, err)
	if assert.Len(t, tags.Tags, 1) {
		tag := tags.Tags[0]
		assert.Equal(t, "refs/tags/tag1", tag.Name)
		assert.Equal(t, masterRef.Hash().String(), tag.Hash)
	}
}

func startServer(t *testing.T, st *storage.Mock, repos map[string]*goGit.Repository) *grpc.ClientConn {
	repo := make(map[string]*goGit.Repository)
	for k, v := range repos {
		_, name := filepath.Split(k)
		registerToStorage(t, st, v, k)
		repo[name] = v
	}

	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	svc, err := newService(repo)
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
