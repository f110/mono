package git

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	gitStorage "github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/memory"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/database/dbtestutil"
	"go.f110.dev/mono/go/git/database"
	"go.f110.dev/mono/go/git/database/dao"
	"go.f110.dev/mono/go/logger"
)

const (
	deltaPackName = "pack-b6ae1dd35be667fe13654be48e9c17e8e6c4aad6"
	deltaBlobHash = "dcc85ca6908c0e6c9bcf9a7637935a2a98bab8e6"
)

var (
	mysqlPort  int
	databaseNo atomic.Int32
)

func TestMain(m *testing.M) {
	logger.Init()
	os.Exit(runWithMySQL(m))
}

func TestDatabaseStorerCloneFromRemoteRepository(t *testing.T) {
	originalRepo := makeSourceRepository(t)
	s := newDatabaseStorer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	repo, err := git.CloneContext(ctx, s, nil, &git.CloneOptions{URL: originalRepo.Storer.(*filesystem.Storage).Filesystem().Root()})
	require.NoError(t, err)

	head, err := repo.Head()
	require.NoError(t, err)
	originalHead, err := originalRepo.Head()
	require.NoError(t, err)
	assert.Equal(t, originalHead.Hash(), head.Hash())

	commit, err := repo.CommitObject(head.Hash())
	require.NoError(t, err)
	assert.Equal(t, "Init", commit.Message)
	file, err := commit.File("docs/design/README.md")
	require.NoError(t, err)
	content, err := file.Contents()
	require.NoError(t, err)
	assert.Equal(t, "design", content)

	remote, err := repo.Remote("origin")
	require.NoError(t, err)
	assert.Equal(t, []string{originalRepo.Storer.(*filesystem.Storage).Filesystem().Root()}, remote.Config().URLs)
}

func TestDatabaseStorerFetchFromRemoteRepository(t *testing.T) {
	originalRepo := makeSourceRepository(t)
	s := newDatabaseStorer(t)

	repo, err := git.Init(s, nil)
	require.NoError(t, err)
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{originalRepo.Storer.(*filesystem.Storage).Filesystem().Root()},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = repo.FetchContext(ctx, &git.FetchOptions{RemoteName: "origin"})
	cancel()
	require.NoError(t, err)

	commitIter, err := repo.Log(&git.LogOptions{All: true})
	require.NoError(t, err)
	var commits []*object.Commit
	err = commitIter.ForEach(func(c *object.Commit) error {
		commits = append(commits, c)
		return nil
	})
	require.NoError(t, err)
	assert.Len(t, commits, 1)

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	err = repo.FetchContext(ctx, &git.FetchOptions{RemoteName: "origin"})
	cancel()
	assert.ErrorIs(t, err, git.NoErrAlreadyUpToDate)
}

func TestDatabaseStorerPackfileWriter(t *testing.T) {
	packData, err := os.ReadFile(filepath.Join("testdata", "delta_pack", deltaPackName+".pack"))
	require.NoError(t, err)
	s := newDatabaseStorer(t)

	writePackfile(t, s, packData)

	// dcc85ca is an OFS_DELTA of depth 2 in the pack. The stored object must be delta-applied.
	obj, err := s.EncodedObject(plumbing.BlobObject, plumbing.NewHash(deltaBlobHash))
	require.NoError(t, err)
	r, err := obj.Reader()
	require.NoError(t, err)
	content, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, deltaBlobHash, plumbing.ComputeHash(plumbing.BlobObject, content).String())
	assert.Equal(t, int64(len(content)), obj.Size())

	assert.NoError(t, s.HasEncodedObject(plumbing.NewHash(deltaBlobHash)))
	size, err := s.EncodedObjectSize(plumbing.NewHash(deltaBlobHash))
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), size)

	// dcc85ca is an OFS_DELTA of depth 2 in the pack. It is stored as the delta without resolving.
	encoding, depth, baseID := storedObject(t, s, plumbing.NewHash(deltaBlobHash))
	assert.Equal(t, database.ObjectEncodingDelta, encoding)
	assert.EqualValues(t, 2, depth)
	assert.True(t, baseID.Valid)

	// Writing the same pack again doesn't fail and doesn't duplicate the objects.
	before := countObjects(t, s)
	assert.NotZero(t, before)
	writePackfile(t, s, packData)
	assert.Equal(t, before, countObjects(t, s))
}

func TestDatabaseStorerPackfileWriterLargePack(t *testing.T) {
	// The blobs are random bytes so that the compressed contents exceed the size which the storer buffers at once.
	repoDir := t.TempDir()
	originalRepo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)
	wt, err := originalRepo.Worktree()
	require.NoError(t, err)
	var blobs []plumbing.Hash
	for i := range 50 {
		buf := make([]byte, 100*1024)
		_, err := rand.Read(buf)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, fmt.Sprintf("file%d", i)), buf, 0644))
		_, err = wt.Add(fmt.Sprintf("file%d", i))
		require.NoError(t, err)
		blobs = append(blobs, plumbing.ComputeHash(plumbing.BlobObject, buf))
	}
	_, err = wt.Commit("Init", &git.CommitOptions{Author: &object.Signature{Name: t.Name(), When: time.Now(), Email: "test@localhost"}})
	require.NoError(t, err)

	s := newDatabaseStorer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = git.CloneContext(ctx, s, nil, &git.CloneOptions{URL: repoDir})
	require.NoError(t, err)

	assert.Greater(t, payloadSize(t, s), int64(maxBufferedBytes))
	// 50 blobs, a tree and a commit
	assert.Equal(t, 52, countObjects(t, s))
	for _, h := range blobs {
		assertObject(t, s, h, plumbing.BlobObject)
	}
}

func TestDatabaseStorerPackfileWriterDeltaDepth(t *testing.T) {
	// The encoder of go-git makes the chain of the deltas from the bigger object to the smaller object.
	src := memory.NewStorage()
	var hashes []plumbing.Hash
	var content []byte
	for i := range 15 {
		content = append(content, fmt.Sprintf("line %d of the file which is appended every version\n", i)...)
		obj := &plumbing.MemoryObject{}
		obj.SetType(plumbing.BlobObject)
		_, err := obj.Write(content)
		require.NoError(t, err)
		h, err := src.SetEncodedObject(obj)
		require.NoError(t, err)
		hashes = append(hashes, h)
	}
	var buf bytes.Buffer
	_, err := packfile.NewEncoder(&buf, src, false).Encode(hashes, 20)
	require.NoError(t, err)

	s := newDatabaseStorer(t)
	writePackfile(t, s, buf.Bytes())

	// The chain in the packfile is deeper than maxDeltaDepth, so some deltas are stored as the full object.
	require.Greater(t, packDeltaDepth(t, buf.Bytes()), maxDeltaDepth)
	var maxDepth int32
	fulls := 0
	rows, err := s.dao.Object.ListSummary(context.Background(), s.repositoryID)
	require.NoError(t, err)
	for _, v := range rows {
		maxDepth = max(maxDepth, v.Depth)
		if v.Encoding == database.ObjectEncodingFull {
			fulls++
		}
	}
	assert.EqualValues(t, maxDeltaDepth, maxDepth)
	assert.Greater(t, fulls, 1)
	for _, h := range hashes {
		obj, err := s.EncodedObject(plumbing.BlobObject, h)
		require.NoError(t, err)
		assert.Equal(t, h, obj.Hash())
	}
}

func TestDatabaseStorerTree(t *testing.T) {
	s := newDatabaseStorer(t)
	blob := setBlob(t, s, "blob")
	missing := plumbing.ComputeHash(plumbing.BlobObject, []byte("missing"))

	tree := setObject(t, s, plumbing.TreeObject, treeContent("100644 a", blob, "100644 b", missing))
	encoding, _, _ := storedObject(t, s, tree)
	assert.Equal(t, database.ObjectEncodingTree, encoding)
	entries, err := s.dao.TreeEntry.ListByRepository(context.Background(), s.repositoryID)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	assertObject(t, s, tree, plumbing.TreeObject)

	// The child which is not stored is external.
	encoding, _, _ = storedObject(t, s, missing)
	assert.Equal(t, database.ObjectEncodingExternal, encoding)
	assert.ErrorIs(t, s.HasEncodedObject(missing), plumbing.ErrObjectNotFound)
	_, err = s.EncodedObject(plumbing.AnyObject, missing)
	assert.ErrorIs(t, err, plumbing.ErrObjectNotFound)
	iter, err := s.IterEncodedObjects(plumbing.AnyObject)
	require.NoError(t, err)
	n := 0
	require.NoError(t, iter.ForEach(func(plumbing.EncodedObject) error { n++; return nil }))
	assert.Equal(t, 2, n)

	// Storing the external object replaces it.
	assert.Equal(t, missing, setBlob(t, s, "missing"))
	encoding, _, _ = storedObject(t, s, missing)
	assert.Equal(t, database.ObjectEncodingFull, encoding)
	assertObject(t, s, missing, plumbing.BlobObject)
	s.cache.Clear()
	assertObject(t, s, tree, plumbing.TreeObject)

	// The tree which can't be reconstructed from the rows is stored as full.
	padded := setObject(t, s, plumbing.TreeObject, treeContent("040000 dir", tree))
	encoding, _, _ = storedObject(t, s, padded)
	assert.Equal(t, database.ObjectEncodingFull, encoding)
	s.cache.Clear()
	assertObject(t, s, padded, plumbing.TreeObject)
}

func TestDatabaseStorerRepack(t *testing.T) {
	// README.md has 13 versions. The objects are stored as full before repacking. After repacking, the newest version
	// is full and the older versions are the deltas from the next newer version up to maxDeltaDepth.
	repoDir := t.TempDir()
	sourceRepo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)
	wt, err := sourceRepo.Worktree()
	require.NoError(t, err)
	var content []byte
	var versions []plumbing.Hash
	for i := range 13 {
		content = append(content, fmt.Sprintf("line %d of README.md which is appended by every commit\n", i)...)
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "README.md"), content, 0644))
		_, err = wt.Add("README.md")
		require.NoError(t, err)
		_, err = wt.Commit(fmt.Sprintf("Commit %d", i), &git.CommitOptions{
			Author: &object.Signature{Name: t.Name(), When: time.Unix(int64(1700000000+i), 0), Email: "test@localhost"},
		})
		require.NoError(t, err)
		versions = append(versions, plumbing.ComputeHash(plumbing.BlobObject, content))
	}
	s := newDatabaseStorer(t)
	copyRepository(t, sourceRepo, s)

	// The object which is not reachable and depends on the repacked object keeps the delta with the new depth.
	unreachable := append(bytes.Clone(content[:len(content)/2]), "unreachable\n"...)
	unreachableHash := plumbing.ComputeHash(plumbing.BlobObject, unreachable)
	ctx := context.Background()
	w := s.newObjectWriter(ctx)
	require.NoError(t, w.prepare([]plumbing.Hash{versions[0], unreachableHash}))
	firstLine := content[:len("line 0 of README.md which is appended by every commit\n")]
	require.NoError(t, w.addDelta(unreachableHash, plumbing.BlobObject, int64(len(unreachable)), w.ids[versions[0]], 1, packfile.DiffDelta(firstLine, unreachable)))
	require.NoError(t, w.flush())
	before := payloadSize(t, s)

	require.NoError(t, s.Repack(ctx))
	s.cache.Clear()

	for i, h := range slices.Backward(versions) {
		encoding, depth, baseID := storedObject(t, s, h)
		n := len(versions) - 1 - i
		switch {
		case n == 0 || n == maxDeltaDepth+1:
			assert.Equal(t, database.ObjectEncodingFull, encoding, "version %d", i)
		default:
			assert.Equal(t, database.ObjectEncodingDelta, encoding, "version %d", i)
			assert.EqualValues(t, n%(maxDeltaDepth+1), depth, "version %d", i)
			bases, err := s.dao.Object.ListHashesByIds(ctx, s.repositoryID, []int64{baseID.Int64})
			require.NoError(t, err)
			require.Len(t, bases, 1)
			assert.Equal(t, versions[i+1][:], bases[0].Hash, "version %d", i)
		}
		assertObject(t, s, h, plumbing.BlobObject)
	}
	encoding, depth, _ := storedObject(t, s, unreachableHash)
	assert.Equal(t, database.ObjectEncodingDelta, encoding)
	assert.EqualValues(t, 2, depth)
	assertObject(t, s, unreachableHash, plumbing.BlobObject)
	after := payloadSize(t, s)
	assert.Less(t, after, before)

	// Repacking again doesn't change the objects.
	require.NoError(t, s.Repack(ctx))
	assert.Equal(t, after, payloadSize(t, s))
}

func TestDatabaseStorerPackfileWriterEmpty(t *testing.T) {
	s := newDatabaseStorer(t)

	w, err := s.PackfileWriter()
	require.NoError(t, err)
	require.NoError(t, w.Close())
	assert.Zero(t, countObjects(t, s))
}

func TestDatabaseStorerEncodedObject(t *testing.T) {
	s := newDatabaseStorer(t)

	obj := s.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)
	w, err := obj.Writer()
	require.NoError(t, err)
	_, err = w.Write([]byte("foobar blob object"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	h, err := s.SetEncodedObject(obj)
	require.NoError(t, err)
	assert.Equal(t, obj.Hash(), h)
	_, err = s.SetEncodedObject(obj)
	require.NoError(t, err)
	assert.Equal(t, 1, countObjects(t, s))

	got, err := s.EncodedObject(plumbing.AnyObject, h)
	require.NoError(t, err)
	assert.Equal(t, plumbing.BlobObject, got.Type())
	assert.Equal(t, h, got.Hash())
	r, err := got.Reader()
	require.NoError(t, err)
	content, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, "foobar blob object", string(content))

	_, err = s.EncodedObject(plumbing.CommitObject, h)
	assert.ErrorIs(t, err, plumbing.ErrObjectNotFound)
	missing := plumbing.NewHash("51f74bc12156490c6a51a5f53b7bc2fb4aa1b310")
	_, err = s.EncodedObject(plumbing.AnyObject, missing)
	assert.ErrorIs(t, err, plumbing.ErrObjectNotFound)
	assert.ErrorIs(t, s.HasEncodedObject(missing), plumbing.ErrObjectNotFound)
	_, err = s.EncodedObjectSize(missing)
	assert.ErrorIs(t, err, plumbing.ErrObjectNotFound)

	iter, err := s.IterEncodedObjects(plumbing.BlobObject)
	require.NoError(t, err)
	var hashes []plumbing.Hash
	require.NoError(t, iter.ForEach(func(o plumbing.EncodedObject) error {
		hashes = append(hashes, o.Hash())
		return nil
	}))
	assert.Equal(t, []plumbing.Hash{h}, hashes)
	iter, err = s.IterEncodedObjects(plumbing.CommitObject)
	require.NoError(t, err)
	_, err = iter.Next()
	assert.Equal(t, io.EOF, err)

	other := newDatabaseStorer(t)
	assert.ErrorIs(t, other.HasEncodedObject(h), plumbing.ErrObjectNotFound)
}

func TestDatabaseStorerReference(t *testing.T) {
	s := newDatabaseStorer(t)
	commit1 := setBlob(t, s, "commit1")
	commit2 := setBlob(t, s, "commit2")
	main := plumbing.NewBranchReferenceName("main")

	require.NoError(t, s.SetReference(plumbing.NewHashReference(main, commit1)))
	require.NoError(t, s.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, main)))

	ref, err := s.Reference(main)
	require.NoError(t, err)
	assert.Equal(t, commit1, ref.Hash())
	ref, err = s.Reference(plumbing.HEAD)
	require.NoError(t, err)
	assert.Equal(t, plumbing.SymbolicReference, ref.Type())
	assert.Equal(t, main, ref.Target())
	_, err = s.Reference(plumbing.NewBranchReferenceName("missing"))
	assert.ErrorIs(t, err, plumbing.ErrReferenceNotFound)

	require.NoError(t, s.SetReference(plumbing.NewHashReference(main, commit2)))
	ref, err = s.Reference(main)
	require.NoError(t, err)
	assert.Equal(t, commit2, ref.Hash())

	err = s.SetReference(plumbing.NewHashReference(main, plumbing.NewHash("51f74bc12156490c6a51a5f53b7bc2fb4aa1b310")))
	assert.ErrorIs(t, err, plumbing.ErrObjectNotFound)

	err = s.CheckAndSetReference(plumbing.NewHashReference(main, commit1), plumbing.NewHashReference(main, commit1))
	assert.ErrorIs(t, err, gitStorage.ErrReferenceHasChanged)
	require.NoError(t, s.CheckAndSetReference(plumbing.NewHashReference(main, commit1), plumbing.NewHashReference(main, commit2)))
	ref, err = s.Reference(main)
	require.NoError(t, err)
	assert.Equal(t, commit1, ref.Hash())
	feature := plumbing.NewBranchReferenceName("feature")
	require.NoError(t, s.CheckAndSetReference(plumbing.NewHashReference(feature, commit2), plumbing.NewHashReference(feature, commit1)))

	refs := iterReferences(t, s)
	assert.Equal(t, map[plumbing.ReferenceName]string{
		plumbing.HEAD: "ref: " + main.String(),
		main:          commit1.String(),
		feature:       commit2.String(),
	}, refs)

	require.NoError(t, s.RemoveReference(feature))
	require.NoError(t, s.RemoveReference(feature))
	_, err = s.Reference(feature)
	assert.ErrorIs(t, err, plumbing.ErrReferenceNotFound)
	assert.Len(t, iterReferences(t, s), 2)
	assert.Empty(t, iterReferences(t, newDatabaseStorer(t)))
}

func TestDatabaseStorerConfig(t *testing.T) {
	s := newDatabaseStorer(t)

	conf, err := s.Config()
	require.NoError(t, err)
	assert.Empty(t, conf.Remotes)

	conf.Remotes["origin"] = &config.RemoteConfig{Name: "origin", URLs: []string{"https://github.com/f110/test-repo.git"}}
	require.NoError(t, s.SetConfig(conf))

	conf, err = s.Config()
	require.NoError(t, err)
	require.Contains(t, conf.Remotes, "origin")
	assert.Equal(t, []string{"https://github.com/f110/test-repo.git"}, conf.Remotes["origin"].URLs)
}

func TestDatabaseBackendOpen(t *testing.T) {
	originalRepo := makeSourceRepository(t)
	repoPath := originalRepo.Storer.(*filesystem.Storage).Filesystem().Root()
	backend := &DatabaseBackend{Conn: newTestDatabase(t)}

	repo, err := backend.Open(context.Background(), &RepositoryConfig{Name: "test", URL: repoPath}, nil, 10*time.Second)
	require.NoError(t, err)
	head, err := repo.Head()
	require.NoError(t, err)
	originalHead, err := originalRepo.Head()
	require.NoError(t, err)
	assert.Equal(t, originalHead.Hash(), head.Hash())

	// The repository which is already stored is opened without cloning.
	repo, err = backend.Open(context.Background(), &RepositoryConfig{Name: "test", URL: "/not/exist"}, nil, 10*time.Second)
	require.NoError(t, err)
	head, err = repo.Head()
	require.NoError(t, err)
	assert.Equal(t, originalHead.Hash(), head.Hash())

	// The repository which failed to clone is cloned again by the next Open.
	_, err = backend.Open(context.Background(), &RepositoryConfig{Name: "retry", URL: filepath.Join(t.TempDir(), "not-exist")}, nil, 10*time.Second)
	require.Error(t, err)
	row, err := dao.NewRepository(backend.Conn).SelectByName(context.Background(), "retry")
	require.NoError(t, err)
	assert.Nil(t, row.ClonedAt)
	repo, err = backend.Open(context.Background(), &RepositoryConfig{Name: "retry", URL: repoPath}, nil, 10*time.Second)
	require.NoError(t, err)
	head, err = repo.Head()
	require.NoError(t, err)
	assert.Equal(t, originalHead.Hash(), head.Hash())
	row, err = dao.NewRepository(backend.Conn).SelectByName(context.Background(), "retry")
	require.NoError(t, err)
	assert.NotNil(t, row.ClonedAt)
}

func runWithMySQL(m *testing.M) int {
	if !dbtestutil.CanUseTemporaryMySQL() {
		return m.Run()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	mysql, err := dbtestutil.NewTemporaryMySQL(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer mysql.Close()
	if err := mysql.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	mysqlPort = mysql.Port
	return m.Run()
}

func newTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	if mysqlPort == 0 {
		t.Skip("mysqld is not found")
	}
	dbName := fmt.Sprintf("git_test_%d", databaseNo.Add(1))
	root, err := sql.Open("mysql", fmt.Sprintf("root@tcp(127.0.0.1:%d)/", mysqlPort))
	require.NoError(t, err)
	defer root.Close()
	_, err = root.Exec("CREATE DATABASE " + dbName)
	require.NoError(t, err)
	conn, err := sql.Open("mysql", fmt.Sprintf("root@tcp(127.0.0.1:%d)/%s?parseTime=true&multiStatements=true", mysqlPort, dbName))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	_, err = conn.Exec(database.Schema)
	require.NoError(t, err)
	return conn
}

func newDatabaseStorer(t *testing.T) *DatabaseStorer {
	t.Helper()
	conn := newTestDatabase(t)
	repo, err := dao.NewRepository(conn).Create(context.Background(), &database.Repository{Name: t.Name(), NextObjectId: 1})
	require.NoError(t, err)
	return NewDatabaseStorer(conn, repo.Id)
}

func writePackfile(t *testing.T, s *DatabaseStorer, data []byte) {
	t.Helper()
	w, err := s.PackfileWriter()
	require.NoError(t, err)
	_, err = w.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Close())
}

func countObjects(t *testing.T, s *DatabaseStorer) int {
	t.Helper()
	rows, err := s.dao.Object.ListStored(context.Background(), s.repositoryID, storedEncodings)
	require.NoError(t, err)
	return len(rows)
}

func setBlob(t *testing.T, s *DatabaseStorer, content string) plumbing.Hash {
	t.Helper()
	obj := s.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)
	w, err := obj.Writer()
	require.NoError(t, err)
	_, err = w.Write([]byte(content))
	require.NoError(t, err)
	h, err := s.SetEncodedObject(obj)
	require.NoError(t, err)
	return h
}

func iterReferences(t *testing.T, s storer.ReferenceStorer) map[plumbing.ReferenceName]string {
	t.Helper()
	iter, err := s.IterReferences()
	require.NoError(t, err)
	refs := make(map[plumbing.ReferenceName]string)
	require.NoError(t, iter.ForEach(func(ref *plumbing.Reference) error {
		refs[ref.Name()] = ref.Strings()[1]
		return nil
	}))
	return refs
}

func setObject(t *testing.T, s *DatabaseStorer, typ plumbing.ObjectType, content []byte) plumbing.Hash {
	t.Helper()
	obj := s.NewEncodedObject()
	obj.SetType(typ)
	w, err := obj.Writer()
	require.NoError(t, err)
	_, err = w.Write(content)
	require.NoError(t, err)
	h, err := s.SetEncodedObject(obj)
	require.NoError(t, err)
	return h
}

// treeContent returns the content of the tree. args is the pairs of "<mode> <name>" and the hash of the child.
func treeContent(args ...any) []byte {
	var buf bytes.Buffer
	for i := 0; i < len(args); i += 2 {
		buf.WriteString(args[i].(string))
		buf.WriteByte(0)
		h := args[i+1].(plumbing.Hash)
		buf.Write(h[:])
	}
	return buf.Bytes()
}

func storedObject(t *testing.T, s *DatabaseStorer, h plumbing.Hash) (database.ObjectEncoding, int32, sql.NullInt64) {
	t.Helper()
	obj, err := s.dao.Object.SelectByHash(context.Background(), s.repositoryID, h[:])
	require.NoError(t, err)
	var baseID sql.NullInt64
	if obj.BaseId != nil {
		baseID = sql.NullInt64{Int64: *obj.BaseId, Valid: true}
	}
	return obj.Encoding, obj.Depth, baseID
}

// assertObject asserts that the content of the stored object matches the hash.
func assertObject(t *testing.T, s *DatabaseStorer, h plumbing.Hash, typ plumbing.ObjectType) {
	t.Helper()
	obj, err := s.EncodedObject(typ, h)
	require.NoError(t, err)
	r, err := obj.Reader()
	require.NoError(t, err)
	content, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, h, plumbing.ComputeHash(typ, content))
	assert.Equal(t, int64(len(content)), obj.Size())
}

// packDeltaDepth returns the maximum length of the chains of the OFS_DELTA in the packfile.
func packDeltaDepth(t *testing.T, pack []byte) int {
	t.Helper()
	scanner := packfile.NewScanner(bytes.NewReader(pack))
	_, count, err := scanner.Header()
	require.NoError(t, err)
	depth := make(map[int64]int)
	maxDepth := 0
	for range count {
		header, err := scanner.NextObjectHeader()
		require.NoError(t, err)
		if header.Type == plumbing.OFSDeltaObject {
			depth[header.Offset] = depth[header.OffsetReference] + 1
			maxDepth = max(maxDepth, depth[header.Offset])
		}
		_, _, err = scanner.NextObject(io.Discard)
		require.NoError(t, err)
	}
	return maxDepth
}

func payloadSize(t *testing.T, s *DatabaseStorer) int64 {
	t.Helper()
	rows, err := s.dao.Object.ListSummary(context.Background(), s.repositoryID)
	require.NoError(t, err)
	var size int64
	for _, v := range rows {
		size += v.PayloadSize
	}
	return size
}

// copyRepository stores the objects and the references of src into s one by one. All objects are stored as full.
func copyRepository(t *testing.T, src *git.Repository, s *DatabaseStorer) {
	t.Helper()
	iter, err := src.Storer.IterEncodedObjects(plumbing.AnyObject)
	require.NoError(t, err)
	require.NoError(t, iter.ForEach(func(obj plumbing.EncodedObject) error {
		_, err := s.SetEncodedObject(obj)
		return err
	}))
	refs, err := src.References()
	require.NoError(t, err)
	require.NoError(t, refs.ForEach(func(ref *plumbing.Reference) error {
		return s.SetReference(ref)
	}))
}
