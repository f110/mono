package git

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/storage"
)

func TestObjectStorageStorer(t *testing.T) {
	originalRepo := makeSourceRepository(t)

	storagePrefix := "test"
	mockStorage := storage.NewMock()
	registerToStorage(t, mockStorage, originalRepo, storagePrefix)

	s := NewObjectStorageStorer(mockStorage, storagePrefix, nil)
	repo, err := git.Open(s, nil)
	require.NoError(t, err)
	commitIter, err := repo.Log(&git.LogOptions{All: true})
	require.NoError(t, err)
	var commits []*object.Commit
	for {
		commit, err := commitIter.Next()
		if err == io.EOF {
			break
		}
		commits = append(commits, commit)
	}
	assert.Len(t, commits, 1)
}

func TestObjectStorageStorerWorkWithRemoteRepository(t *testing.T) {
	originalRepo := makeSourceRepository(t)

	mockStorage := storage.NewMock()
	s := NewObjectStorageStorer(mockStorage, "test", nil)
	repo, err := git.Init(s, nil)
	require.NoError(t, err)
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{originalRepo.Storer.(*filesystem.Storage).Filesystem().Root()},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	err = repo.FetchContext(ctx, &git.FetchOptions{RemoteName: "origin"})
	cancel()
	require.NoError(t, err)
}

func TestObjectStorageStorerReadDeltaObject(t *testing.T) {
	// Regression test: a blob stored as an OFS_DELTA inside a packfile must be
	// delta-applied when served straight from the pack. The fixture pack was
	// extracted from a real repository; dcc85ca is an OFS_DELTA of depth 2.
	const (
		prefix   = "repo"
		packName = "pack-b6ae1dd35be667fe13654be48e9c17e8e6c4aad6"
		blobHash = "dcc85ca6908c0e6c9bcf9a7637935a2a98bab8e6"
	)
	packData, err := os.ReadFile(filepath.Join("testdata", "delta_pack", packName+".pack"))
	require.NoError(t, err)
	idxData, err := os.ReadFile(filepath.Join("testdata", "delta_pack", packName+".idx"))
	require.NoError(t, err)

	mockStorage := storage.NewMock()
	mockStorage.AddTree(path.Join(prefix, "objects/pack", packName+".pack"), packData)
	mockStorage.AddTree(path.Join(prefix, "objects/pack", packName+".idx"), idxData)

	s := NewObjectStorageStorer(mockStorage, prefix, nil)
	obj, err := s.EncodedObject(plumbing.BlobObject, plumbing.NewHash(blobHash))
	require.NoError(t, err)

	r, err := obj.Reader()
	require.NoError(t, err)
	content, err := io.ReadAll(r)
	require.NoError(t, err)

	// The reconstructed content must hash back to the requested object, and its
	// length must match the reported size.
	assert.Equal(t, blobHash, plumbing.ComputeHash(plumbing.BlobObject, content).String())
	assert.Equal(t, int64(len(content)), obj.Size())
}

func TestEncodedObjectJSON(t *testing.T) {
	obj := &EncodedObject{
		hash: plumbing.NewHash("51f74bc12156490c6a51a5f53b7bc2fb4aa1b310"),
		typ:  plumbing.BlobObject,
		r:    io.NopCloser(strings.NewReader("foobar blob object")),
	}
	buf, err := json.Marshal(obj)
	require.NoError(t, err)

	newObj := &EncodedObject{}
	err = json.Unmarshal(buf, newObj)
	require.NoError(t, err)
	assert.Equal(t, "51f74bc12156490c6a51a5f53b7bc2fb4aa1b310", newObj.Hash().String())
	assert.Equal(t, plumbing.BlobObject, newObj.Type())
	r, err := newObj.Reader()
	require.NoError(t, err)
	content, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, []byte("foobar blob object"), content)
}

func makeSourceRepository(t *testing.T) *git.Repository {
	// Make new git repository
	repoDir := t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)
	wt, err := repo.Worktree()
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("Hello"), 0644)
	require.NoError(t, err)
	_, err = wt.Add("README.md")
	require.NoError(t, err)

	err = os.Mkdir(filepath.Join(repoDir, "docs"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(repoDir, "docs/README.md"), []byte("docs"), 0644)
	require.NoError(t, err)
	_, err = wt.Add("docs/README.md")
	require.NoError(t, err)

	err = os.Mkdir(filepath.Join(repoDir, "docs/design"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(repoDir, "docs/design/README.md"), []byte("design"), 0644)
	require.NoError(t, err)
	_, err = wt.Add("docs/design/README.md")
	require.NoError(t, err)

	_, err = wt.Commit("Init", &git.CommitOptions{
		Author:    &object.Signature{Name: t.Name(), When: time.Now(), Email: "test@localhost"},
		Committer: &object.Signature{Name: t.Name(), When: time.Now(), Email: "test@localhost"},
	})
	require.NoError(t, err)

	_, err = repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{"https://github.com/f110/test-repo.git"}})
	require.NoError(t, err)

	return repo
}

func registerToStorage(t *testing.T, s *storage.Mock, repo *git.Repository, prefix string) {
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
