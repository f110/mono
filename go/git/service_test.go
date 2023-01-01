package git

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	goGit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"go.f110.dev/mono/go/pkg/storage"
)

func TestListRepositories(t *testing.T) {
	mockStorage := storage.NewMock()
	conn := startServer(t, mockStorage, map[string]*goGit.Repository{"test/test1": makeSourceRepository(t), "test/test2": makeSourceRepository(t)})
	gitData := NewGitDataClient(conn)

	res, err := gitData.ListRepositories(context.Background(), &RequestListRepositories{})
	require.NoError(t, err)
	assert.Len(t, res.Repositories, 2)
}

func TestListReferences(t *testing.T) {
	mockStorage := storage.NewMock()
	repo := makeSourceRepository(t)
	conn := startServer(t, mockStorage, map[string]*goGit.Repository{"test/test1": repo})
	gitData := NewGitDataClient(conn)

	res, err := gitData.ListReferences(context.Background(), &RequestListReferences{Repo: "test1"})
	require.NoError(t, err)
	assert.Len(t, res.Refs, 2)

	v, _ := repo.References()
	var expectRefs []string
	err = v.ForEach(func(ref *plumbing.Reference) error {
		expectRefs = append(expectRefs, ref.Name().String())
		return nil
	})
	require.NoError(t, err)
	refs := make(map[string]*Reference)
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
	gitData := NewGitDataClient(conn)

	ref, err := repo.Reference(plumbing.NewBranchReferenceName("master"), false)
	require.NoError(t, err)
	commit, err := gitData.GetCommit(context.Background(), &RequestGetCommit{Repo: "test1", Sha: ref.Hash().String()})
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
	gitData := NewGitDataClient(conn)

	ref, err := repo.Reference(plumbing.NewBranchReferenceName("master"), false)
	require.NoError(t, err)
	commit, err := repo.CommitObject(ref.Hash())
	require.NoError(t, err)
	tree, err := gitData.GetTree(context.Background(), &RequestGetTree{Repo: "test1", Sha: commit.TreeHash.String(), Path: "/"})
	require.NoError(t, err)
	assert.Equal(t, commit.TreeHash.String(), tree.Sha)
	files := make(map[string]*TreeEntry)
	for _, v := range tree.Tree {
		files[v.Path] = v
	}
	assert.Contains(t, files, "README.md")
}

func TestGetBlob(t *testing.T) {
	mockStorage := storage.NewMock()
	repo := makeSourceRepository(t)
	conn := startServer(t, mockStorage, map[string]*goGit.Repository{"test/test1": repo})
	gitData := NewGitDataClient(conn)

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

	blob, err := gitData.GetBlob(context.Background(), &RequestGetBlob{Repo: "test1", Sha: blobHash})
	require.NoError(t, err)
	assert.Equal(t, blobHash, blob.Sha)
	assert.Equal(t, string(blob.Content), expectContent)
}

func TestListTag(t *testing.T) {
	mockStorage := storage.NewMock()
	repo := makeSourceRepository(t)
	conn := startServer(t, mockStorage, map[string]*goGit.Repository{"test/test1": repo})
	gitData := NewGitDataClient(conn)

	masterRef, err := repo.Reference(plumbing.NewBranchReferenceName("master"), false)
	require.NoError(t, err)
	_, err = repo.CreateTag("tag1", masterRef.Hash(), nil)
	require.NoError(t, err)

	tags, err := gitData.ListTag(context.Background(), &RequestListTag{Repo: "test1"})
	require.NoError(t, err)
	if assert.Len(t, tags.Tags, 1) {
		tag := tags.Tags[0]
		assert.Equal(t, "refs/tags/tag1", tag.Name)
		assert.Equal(t, masterRef.Hash().String(), tag.Hash)
	}
}

func TestListBranch(t *testing.T) {
	mockStorage := storage.NewMock()
	repo := makeSourceRepository(t)
	conn := startServer(t, mockStorage, map[string]*goGit.Repository{"test/test1": repo})
	gitData := NewGitDataClient(conn)

	masterRef, err := repo.Reference(plumbing.NewBranchReferenceName("master"), false)
	require.NoError(t, err)
	err = repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("foobar"), masterRef.Hash()))
	require.NoError(t, err)

	branches, err := gitData.ListBranch(context.Background(), &RequestListBranch{Repo: "test1"})
	require.NoError(t, err)
	if assert.Len(t, branches.Branches, 2) {
		b := make(map[string]*Reference)
		for _, v := range branches.Branches {
			b[v.Name] = v
		}

		assert.Contains(t, b, "refs/heads/foobar")
		assert.Contains(t, b, "refs/heads/master")
		assert.Equal(t, b["refs/heads/master"].Hash, b["refs/heads/foobar"].Hash)
	}
}

func TestGetReference(t *testing.T) {
	mockStorage := storage.NewMock()
	repo := makeSourceRepository(t)
	conn := startServer(t, mockStorage, map[string]*goGit.Repository{"test/test1": repo})
	gitData := NewGitDataClient(conn)

	ref, err := gitData.GetReference(context.Background(), &RequestGetReference{Repo: "test1", Ref: plumbing.NewBranchReferenceName("master").String()})
	require.NoError(t, err)
	assert.Equal(t, "refs/heads/master", ref.Ref.Name)
}

func TestGetFile(t *testing.T) {
	mockStorage := storage.NewMock()
	repo := makeSourceRepository(t)
	conn := startServer(t, mockStorage, map[string]*goGit.Repository{"test/test1": repo})
	gitData := NewGitDataClient(conn)

	file, err := gitData.GetFile(context.Background(), &RequestGetFile{Repo: "test1", Ref: plumbing.NewBranchReferenceName("master").String(), Path: "README.md"})
	require.NoError(t, err)
	assert.NotEmpty(t, file.Content)
	assert.Equal(t, "https://raw.githubusercontent.com/f110/test-repo/refs/heads/master/README.md", file.RawUrl)
	assert.Equal(t, "https://github.com/f110/test-repo/edit/refs/heads/master/README.md", file.EditUrl)
	assert.NotEmpty(t, file.Sha)
}

func TestStat(t *testing.T) {
	mockStorage := storage.NewMock()
	repo := makeSourceRepository(t)
	conn := startServer(t, mockStorage, map[string]*goGit.Repository{"test/test1": repo})
	gitData := NewGitDataClient(conn)

	cases := []struct {
		Path string
		Name string
		Mode filemode.FileMode
	}{
		{
			Path: "",
			Name: "",
			Mode: filemode.Dir,
		},
		{
			Path: "/",
			Name: "",
			Mode: filemode.Dir,
		},
		{
			Path: "README.md",
			Name: "README.md",
			Mode: filemode.Regular,
		},
		{
			Path: "/README.md",
			Name: "README.md",
			Mode: filemode.Regular,
		},
		{
			Path: "docs",
			Name: "docs",
			Mode: filemode.Dir,
		},
		{
			Path: "docs/",
			Name: "docs",
			Mode: filemode.Dir,
		},
		{
			Path: "docs/README.md",
			Name: "docs/README.md",
			Mode: filemode.Regular,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Path, func(t *testing.T) {
			stat, err := gitData.Stat(context.Background(), &RequestStat{
				Repo: "test1",
				Ref:  plumbing.NewBranchReferenceName("master").String(),
				Path: tc.Path,
			})
			require.NoError(t, err)
			assert.Equal(t, tc.Name, stat.Name)
			assert.Equal(t, uint32(tc.Mode), stat.Mode)
			assert.NotEmpty(t, stat.Hash)
		})
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
	svc, err := NewDataService(repo)
	require.NoError(t, err)
	RegisterGitDataServer(s, svc)
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
