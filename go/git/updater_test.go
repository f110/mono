package git

import (
	"context"
	"fmt"
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

	"go.f110.dev/mono/go/git/database"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

func TestUpdater_UpdateRepo(t *testing.T) {
	logger.SetLogLevel("debug")
	logger.Init()

	sourceRepo := makeUpdaterSourceRepository(t)
	mockStorage := storage.NewMock()
	repoPath := sourceRepo.Storer.(*filesystem.Storage).Filesystem().Root()
	_, err := InitObjectStorageRepository(context.Background(), mockStorage, repoPath, "test", nil)
	require.NoError(t, err)

	updater := NewUpdater(&ObjectStorageBackend{Client: mockStorage}, nil, NewStaticRepositoryProvider(nil), 1)

	masterRef, err := sourceRepo.Reference(plumbing.NewBranchReferenceName("master"), false)
	require.NoError(t, err)
	err = sourceRepo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("foobar"), masterRef.Hash()))
	require.NoError(t, err)
	_, err = sourceRepo.CreateTag("baz", masterRef.Hash(), nil)
	require.NoError(t, err)

	s := NewObjectStorageStorer(mockStorage, "test", nil, nil)
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

func TestUpdater_Sync(t *testing.T) {
	sourceRepo := makeUpdaterSourceRepository(t)
	mockStorage := storage.NewMock()
	repoPath := sourceRepo.Storer.(*filesystem.Storage).Filesystem().Root()
	_, err := InitObjectStorageRepository(context.Background(), mockStorage, repoPath, "test", nil)
	require.NoError(t, err)

	masterRef, err := sourceRepo.Reference(plumbing.NewBranchReferenceName("master"), false)
	require.NoError(t, err)
	err = sourceRepo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("foobar"), masterRef.Hash()))
	require.NoError(t, err)

	repo, err := goGit.Open(NewObjectStorageStorer(mockStorage, "test", nil, nil), nil)
	require.NoError(t, err)
	updater := NewUpdater(&ObjectStorageBackend{Client: mockStorage}, nil, NewStaticRepositoryProvider([]*RepositoryConfig{{Name: "test", URL: repoPath, Prefix: "test", goGit: repo}}), 1)

	err = updater.Sync(context.Background(), "https://example.com/unknown.git")
	assert.ErrorIs(t, err, ErrRepositoryNotTracked)

	err = updater.Sync(context.Background(), repoPath)
	require.NoError(t, err)
	n, err := repo.Reference(plumbing.NewBranchReferenceName("foobar"), false)
	require.NoError(t, err)
	assert.Equal(t, masterRef.Hash(), n.Hash())
}

func TestUpdater_SyncNotOpenedRepository(t *testing.T) {
	sourceRepo := makeUpdaterSourceRepository(t)
	mockStorage := storage.NewMock()
	repoPath := sourceRepo.Storer.(*filesystem.Storage).Filesystem().Root()

	repoConfig := &RepositoryConfig{Name: "test", URL: repoPath, Prefix: "test"}
	updater := NewUpdater(&ObjectStorageBackend{Client: mockStorage}, nil, NewStaticRepositoryProvider([]*RepositoryConfig{repoConfig}), 1)

	err := updater.Sync(context.Background(), repoPath)
	require.NoError(t, err)
	require.NotNil(t, repoConfig.goGit)

	masterRef, err := sourceRepo.Reference(plumbing.NewBranchReferenceName("master"), false)
	require.NoError(t, err)
	n, err := repoConfig.goGit.Reference(plumbing.NewBranchReferenceName("master"), false)
	require.NoError(t, err)
	assert.Equal(t, masterRef.Hash(), n.Hash())
}

func makeUpdaterSourceRepository(t *testing.T) *goGit.Repository {
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

func TestUpdater_SyncWithDatabase(t *testing.T) {
	sourceRepo := makeUpdaterSourceRepository(t)
	repoPath := sourceRepo.Storer.(*filesystem.Storage).Filesystem().Root()

	repoConfig := &RepositoryConfig{Name: "test", URL: repoPath}
	updater := NewUpdater(&DatabaseBackend{Conn: newTestDatabase(t)}, nil, NewStaticRepositoryProvider([]*RepositoryConfig{repoConfig}), 1)

	err := updater.Sync(context.Background(), repoPath)
	require.NoError(t, err)
	require.NotNil(t, repoConfig.goGit)
	masterRef, err := sourceRepo.Reference(plumbing.NewBranchReferenceName("master"), false)
	require.NoError(t, err)
	n, err := repoConfig.goGit.Reference(plumbing.NewBranchReferenceName("master"), false)
	require.NoError(t, err)
	assert.Equal(t, masterRef.Hash(), n.Hash())

	wt, err := sourceRepo.Worktree()
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(wt.Filesystem.Root(), "README.md"), []byte("Updated"), 0644)
	require.NoError(t, err)
	_, err = wt.Add("README.md")
	require.NoError(t, err)
	newCommit, err := wt.Commit("Update", &goGit.CommitOptions{
		Author: &object.Signature{Name: t.Name(), When: time.Now(), Email: "test@localhost"},
	})
	require.NoError(t, err)
	err = sourceRepo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("foobar"), newCommit))
	require.NoError(t, err)

	err = updater.Sync(context.Background(), repoPath)
	require.NoError(t, err)
	for _, name := range []string{"master", "foobar"} {
		n, err := repoConfig.goGit.Reference(plumbing.NewBranchReferenceName(name), false)
		require.NoError(t, err)
		assert.Equal(t, newCommit, n.Hash(), name)
	}
	commit, err := repoConfig.goGit.CommitObject(newCommit)
	require.NoError(t, err)
	file, err := commit.File("README.md")
	require.NoError(t, err)
	content, err := file.Contents()
	require.NoError(t, err)
	assert.Equal(t, "Updated", content)
}

func TestUpdater_RepackWithDatabase(t *testing.T) {
	sourceRepo := makeUpdaterSourceRepository(t)
	repoPath := sourceRepo.Storer.(*filesystem.Storage).Filesystem().Root()
	wt, err := sourceRepo.Worktree()
	require.NoError(t, err)
	var content string
	for i := range 50 {
		content += fmt.Sprintf("Line %d of README.md\n", i)
	}
	for _, v := range []string{content, content + "The last line\n"} {
		err = os.WriteFile(filepath.Join(wt.Filesystem.Root(), "README.md"), []byte(v), 0644)
		require.NoError(t, err)
		_, err = wt.Add("README.md")
		require.NoError(t, err)
		_, err = wt.Commit("Update", &goGit.CommitOptions{Author: &object.Signature{Name: t.Name(), When: time.Now(), Email: "test@localhost"}})
		require.NoError(t, err)
	}

	repoConfig := &RepositoryConfig{Name: "test", URL: repoPath}
	updater := NewUpdater(&DatabaseBackend{Conn: newTestDatabase(t)}, nil, NewStaticRepositoryProvider([]*RepositoryConfig{repoConfig}), 1)
	require.NoError(t, updater.Sync(context.Background(), repoPath))

	updater.repack(context.Background())

	s := repoConfig.goGit.Storer.(*DatabaseStorer)
	encoding, depth, _ := storedObject(t, s, plumbing.ComputeHash(plumbing.BlobObject, []byte(content)))
	assert.Equal(t, database.ObjectEncodingDelta, encoding)
	assert.EqualValues(t, 1, depth)
}
