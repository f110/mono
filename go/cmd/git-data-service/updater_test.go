package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	goGit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/storage"
)

func TestUpdater_UpdateRepo(t *testing.T) {
	logger.SetLogLevel("debug")
	logger.Init()

	// Set up the repository both local and on object storage
	sourceRepo := makeSourceRepository(t)
	mockStorage := storage.NewMock()
	repoPath := sourceRepo.Storer.(*filesystem.Storage).Filesystem().Root()
	_, err := git.InitObjectStorageRepository(context.Background(), mockStorage, repoPath, "test", nil)
	require.NoError(t, err)

	updater, err := newRepositoryUpdater(nil, nil, time.Minute, "", nil, 1)
	require.NoError(t, err)

	// Mutate local repository
	masterRef, err := sourceRepo.Reference(plumbing.NewBranchReferenceName("master"), false)
	require.NoError(t, err)
	err = sourceRepo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("foobar"), masterRef.Hash()))
	require.NoError(t, err)
	_, err = sourceRepo.CreateTag("baz", masterRef.Hash(), nil)
	require.NoError(t, err)

	s := git.NewObjectStorageStorer(mockStorage, "test", nil)
	repo, err := goGit.Open(s, nil)
	require.NoError(t, err)

	updater.updateRepo(context.Background(), repo)

	n, err := repo.Reference(plumbing.NewBranchReferenceName("foobar"), false)
	require.NoError(t, err)
	assert.Equal(t, masterRef.Hash(), n.Hash())

	n, err = repo.Reference(plumbing.NewTagReferenceName("baz"), false)
	require.NoError(t, err)
	assert.Equal(t, masterRef.Hash(), n.Hash())
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
