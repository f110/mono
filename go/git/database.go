package git

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/idxfile"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/storer"
	gitStorage "github.com/go-git/go-git/v5/storage"
	"github.com/go-sql-driver/mysql"
	"github.com/klauspost/compress/zstd"
	"go.f110.dev/protoc-ddl/probe"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/git/database"
	"go.f110.dev/mono/go/git/database/dao"
	githubutil "go.f110.dev/mono/go/github"
	"go.f110.dev/mono/go/logger"
)

const (
	// maxBufferedRows and maxBufferedBytes bound the rows which objectWriter keeps before writing. maxBufferedBytes is
	// the size of a statement which CreateMulti of DAO writes at once.
	maxBufferedRows  = 1000
	maxBufferedBytes = 4 * 1024 * 1024
	// maxLookupHashes bounds the hashes of a query which looks up the IDs.
	maxLookupHashes = 1000
	// maxDeltaDepth is the maximum length of the chain of the deltas. Reading the delta reads the objects of the chain
	// one by one.
	maxDeltaDepth = 10
	// maxChainLength stops following the chain of the deltas if the stored chain is broken.
	maxChainLength = 64
)

var (
	zstdEncoder, _ = zstd.NewWriter(nil)
	zstdDecoder, _ = zstd.NewReader(nil)
	// storedEncodings is the encodings of the objects which are stored. The external object is not stored.
	storedEncodings = []uint32{uint32(database.ObjectEncodingFull), uint32(database.ObjectEncodingDelta), uint32(database.ObjectEncodingTree)}
)

// DatabaseStorer is the storer of go-git which stores the objects, the references and the config of one repository
// into the database of go/git/database. It doesn't have the index, the shallow commits and the submodules.
//
// The object is stored as the full content or the delta of the received packfile, and the tree is stored as the rows
// of the entries. The objects refer to each other by the integer ID instead of the hash.
// The objects and the references of a repository must be written by one writer at the same time. The storer doesn't
// lock the rows.
type DatabaseStorer struct {
	dao          dao.Options
	repositoryID int32
	cache        cache.Object
}

var _ gitStorage.Storer = (*DatabaseStorer)(nil)
var _ storer.PackfileWriter = (*DatabaseStorer)(nil)

// NewDatabaseStorer returns the storer of the repository. The row of the repository must exist.
func NewDatabaseStorer(conn *sql.DB, repositoryID int32) *DatabaseStorer {
	return &DatabaseStorer{dao: dao.NewOptions(conn), repositoryID: repositoryID, cache: cache.NewObjectLRUDefault()}
}

func (s *DatabaseStorer) NewEncodedObject() plumbing.EncodedObject {
	return &plumbing.MemoryObject{}
}

// SetEncodedObject stores the object. Storing the object which already exists does nothing.
func (s *DatabaseStorer) SetEncodedObject(obj plumbing.EncodedObject) (plumbing.Hash, error) {
	if obj.Type().IsDelta() {
		return plumbing.ZeroHash, plumbing.ErrInvalidType
	}
	content, err := readContent(obj)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	h := obj.Hash()
	w := s.newObjectWriter(context.Background())
	if err := w.prepare([]plumbing.Hash{h}); err != nil {
		return plumbing.ZeroHash, err
	}
	if w.stored[h] {
		return h, nil
	}
	if obj.Type() == plumbing.TreeObject {
		if err := w.prepareTrees([][]byte{content}); err != nil {
			return plumbing.ZeroHash, err
		}
	}
	if err := w.add(h, obj.Type(), content); err != nil {
		return plumbing.ZeroHash, err
	}
	if err := w.flush(); err != nil {
		return plumbing.ZeroHash, err
	}
	return h, nil
}

// PackfileWriter implements storer.PackfileWriter. The deltas in the received packfile are stored without computing
// again, and the objects are stored with a few multi-row INSERTs.
func (s *DatabaseStorer) PackfileWriter() (io.WriteCloser, error) {
	fs := osfs.New(os.TempDir())
	f, err := util.TempFile(fs, ".", "git-pack-")
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return &databasePackfileWriter{storer: s, fs: fs, file: f}, nil
}

func (s *DatabaseStorer) EncodedObject(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	if obj, ok := s.cache.Get(h); ok {
		if t != plumbing.AnyObject && obj.Type() != t {
			return nil, plumbing.ErrObjectNotFound
		}
		return obj, nil
	}

	ctx := context.Background()
	row, err := s.dao.Object.SelectByHash(ctx, s.repositoryID, h[:])
	if errors.Is(err, sql.ErrNoRows) {
		return nil, plumbing.ErrObjectNotFound
	}
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	if row.Encoding == database.ObjectEncodingExternal || (t != plumbing.AnyObject && plumbing.ObjectType(row.Type) != t) {
		return nil, plumbing.ErrObjectNotFound
	}

	var content []byte
	switch row.Encoding {
	case database.ObjectEncodingFull:
		content, err = zstdDecoder.DecodeAll(row.Payload, nil)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
	case database.ObjectEncodingDelta:
		content, err = s.restoreDelta(ctx, row)
	case database.ObjectEncodingTree:
		content, err = s.restoreTree(ctx, row.Id)
	default:
		err = xerrors.Definef("git: the object %s has the unknown encoding %d", h, row.Encoding).WithStack()
	}
	if err != nil {
		return nil, err
	}
	if int64(len(content)) != row.Size {
		return nil, xerrors.Definef("git: the size of the object %s is %d but %d is stored", h, len(content), row.Size).WithStack()
	}

	obj := &plumbing.MemoryObject{}
	obj.SetType(plumbing.ObjectType(row.Type))
	if _, err := obj.Write(content); err != nil {
		return nil, xerrors.WithStack(err)
	}
	s.cache.Put(obj)
	return obj, nil
}

func (s *DatabaseStorer) IterEncodedObjects(t plumbing.ObjectType) (storer.EncodedObjectIter, error) {
	var rows []*database.Object
	var err error
	if t == plumbing.AnyObject {
		rows, err = s.dao.Object.ListStored(context.Background(), s.repositoryID, storedEncodings)
	} else {
		rows, err = s.dao.Object.ListStoredByType(context.Background(), s.repositoryID, storedEncodings, uint32(t))
	}
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	hashes := make([]plumbing.Hash, len(rows))
	for i, v := range rows {
		copy(hashes[i][:], v.Hash)
	}
	return storer.NewEncodedObjectLookupIter(s, t, hashes), nil
}

func (s *DatabaseStorer) HasEncodedObject(h plumbing.Hash) error {
	_, err := s.EncodedObjectSize(h)
	return err
}

func (s *DatabaseStorer) EncodedObjectSize(h plumbing.Hash) (int64, error) {
	rows, err := s.dao.Object.ListIdsByHashes(context.Background(), s.repositoryID, [][]byte{h[:]})
	if err != nil {
		return 0, xerrors.WithStack(err)
	}
	if len(rows) == 0 || rows[0].Encoding == database.ObjectEncodingExternal {
		return 0, plumbing.ErrObjectNotFound
	}
	return rows[0].Size, nil
}

func (s *DatabaseStorer) AddAlternate(_ string) error {
	return xerrors.Define("git: the alternates are not supported").WithStack()
}

// SetReference stores the reference. The object of the hash reference must be stored before.
func (s *DatabaseStorer) SetReference(ref *plumbing.Reference) error {
	return s.setReference(context.Background(), ref)
}

// CheckAndSetReference stores ref if the stored reference which has the same name points to the object of old.
// If the reference is not stored, ref is stored as same as the storers of go-git. The reference is not locked between
// reading and writing because the storer has only one writer.
func (s *DatabaseStorer) CheckAndSetReference(ref, old *plumbing.Reference) error {
	if ref == nil {
		return nil
	}
	if old != nil {
		current, err := s.Reference(ref.Name())
		switch {
		case errors.Is(err, plumbing.ErrReferenceNotFound):
		case err != nil:
			return err
		case current.Hash() != old.Hash():
			// The hash of the symbolic reference is ZeroHash as same as plumbing.Reference.
			return gitStorage.ErrReferenceHasChanged
		}
	}
	return s.SetReference(ref)
}

func (s *DatabaseStorer) Reference(name plumbing.ReferenceName) (*plumbing.Reference, error) {
	ctx := context.Background()
	ref, err := s.dao.Reference.Select(ctx, s.repositoryID, name.String())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, plumbing.ErrReferenceNotFound
	}
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	refs, err := s.newReferences(ctx, []*database.Reference{ref})
	if err != nil {
		return nil, err
	}
	return refs[0], nil
}

func (s *DatabaseStorer) IterReferences() (storer.ReferenceIter, error) {
	ctx := context.Background()
	rows, err := s.dao.Reference.ListByRepository(ctx, s.repositoryID)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	refs, err := s.newReferences(ctx, rows)
	if err != nil {
		return nil, err
	}
	return storer.NewReferenceSliceIter(refs), nil
}

func (s *DatabaseStorer) RemoveReference(name plumbing.ReferenceName) error {
	err := s.dao.Reference.Delete(context.Background(), s.repositoryID, name.String())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return xerrors.WithStack(err)
	}
	return nil
}

// CountLooseRefs returns 0 because the storer doesn't have packed-refs.
func (s *DatabaseStorer) CountLooseRefs() (int, error) {
	return 0, nil
}

// PackRefs does nothing because the storer doesn't have packed-refs.
func (s *DatabaseStorer) PackRefs() error {
	return nil
}

func (s *DatabaseStorer) SetShallow(_ []plumbing.Hash) error {
	return xerrors.Define("git: the shallow commits are not supported").WithStack()
}

// Shallow returns nothing because the storer doesn't have the shallow commits.
func (s *DatabaseStorer) Shallow() ([]plumbing.Hash, error) {
	return nil, nil
}

func (s *DatabaseStorer) SetIndex(_ *index.Index) error {
	return xerrors.Define("git: the index is not supported").WithStack()
}

func (s *DatabaseStorer) Index() (*index.Index, error) {
	return nil, xerrors.Define("git: the index is not supported").WithStack()
}

func (s *DatabaseStorer) Config() (*config.Config, error) {
	repo, err := s.dao.Repository.Select(context.Background(), s.repositoryID)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	if repo.Config == "" {
		return config.NewConfig(), nil
	}
	conf, err := config.ReadConfig(strings.NewReader(repo.Config))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return conf, nil
}

func (s *DatabaseStorer) SetConfig(conf *config.Config) error {
	if err := conf.Validate(); err != nil {
		return xerrors.WithStack(err)
	}
	buf, err := conf.Marshal()
	if err != nil {
		return xerrors.WithStack(err)
	}
	ctx := context.Background()
	repo, err := s.dao.Repository.Select(ctx, s.repositoryID)
	if err != nil {
		return xerrors.WithStack(err)
	}
	repo.Config = string(buf)
	if err := s.dao.Repository.Update(ctx, repo); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (s *DatabaseStorer) Module(_ string) (gitStorage.Storer, error) {
	return nil, xerrors.Define("git: the submodules are not supported").WithStack()
}

// restoreDelta follows the chain of the deltas from row one object at a time until the full object or the object in
// the cache, then applies the deltas from it.
func (s *DatabaseStorer) restoreDelta(ctx context.Context, row *database.Object) ([]byte, error) {
	deltas := [][]byte{row.Payload}
	var content []byte
	for cur := row; ; {
		if len(deltas) > maxChainLength {
			return nil, xerrors.Definef("git: the chain of the deltas of the object %d is too long", row.Id).WithStack()
		}
		if cur.BaseId == nil {
			return nil, xerrors.Definef("git: the delta object %d doesn't have the base", cur.Id).WithStack()
		}
		base, err := s.dao.Object.Select(ctx, s.repositoryID, *cur.BaseId)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		var h plumbing.Hash
		copy(h[:], base.Hash)
		if obj, ok := s.cache.Get(h); ok {
			if content, err = readContent(obj); err != nil {
				return nil, err
			}
			break
		}
		if base.Encoding == database.ObjectEncodingFull {
			if content, err = zstdDecoder.DecodeAll(base.Payload, nil); err != nil {
				return nil, xerrors.WithStack(err)
			}
			break
		}
		if base.Encoding != database.ObjectEncodingDelta {
			return nil, xerrors.Definef("git: the base of the delta object %d is not the full object or the delta", cur.Id).WithStack()
		}
		deltas = append(deltas, base.Payload)
		cur = base
	}

	for i := len(deltas) - 1; i >= 0; i-- {
		delta, err := zstdDecoder.DecodeAll(deltas[i], nil)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		if content, err = packfile.PatchDelta(content, delta); err != nil {
			return nil, xerrors.WithStack(err)
		}
	}
	return content, nil
}

// restoreTree builds the content of the tree from the rows of the entries and the hashes of the children.
func (s *DatabaseStorer) restoreTree(ctx context.Context, id int64) ([]byte, error) {
	entries, err := s.dao.TreeEntry.ListByTree(ctx, s.repositoryID, id)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	if len(entries) == 0 {
		return nil, nil
	}
	childIDs := make([]int64, len(entries))
	for i, v := range entries {
		childIDs[i] = v.ChildId
	}
	children, err := s.dao.Object.ListHashesByIds(ctx, s.repositoryID, childIDs)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	hashes := make(map[int64][]byte, len(children))
	for _, v := range children {
		hashes[v.Id] = v.Hash
	}

	var buf bytes.Buffer
	for _, v := range entries {
		h, ok := hashes[v.ChildId]
		if !ok {
			return nil, xerrors.Definef("git: the child %d of the tree %d is not found", v.ChildId, id).WithStack()
		}
		e := treeEntry{mode: v.Mode, name: v.Name}
		copy(e.hash[:], h)
		e.encode(&buf)
	}
	return buf.Bytes(), nil
}

// setReference stores the reference. If the reference is stored, it is updated.
func (s *DatabaseStorer) setReference(ctx context.Context, ref *plumbing.Reference) error {
	var objectID *int64
	var target *string
	switch ref.Type() {
	case plumbing.HashReference:
		h := ref.Hash()
		rows, err := s.dao.Object.ListIdsByHashes(ctx, s.repositoryID, [][]byte{h[:]})
		if err != nil {
			return xerrors.WithStack(err)
		}
		if len(rows) == 0 || rows[0].Encoding == database.ObjectEncodingExternal {
			return xerrors.WithMessagef(plumbing.ErrObjectNotFound, "the object of %s", ref.Name())
		}
		objectID = &rows[0].Id
	case plumbing.SymbolicReference:
		t := ref.Target().String()
		target = &t
	default:
		return xerrors.Definef("git: the reference %s has the invalid type", ref.Name()).WithStack()
	}

	current, err := s.dao.Reference.Select(ctx, s.repositoryID, ref.Name().String())
	if errors.Is(err, sql.ErrNoRows) {
		_, err := s.dao.Reference.Create(ctx, &database.Reference{RepositoryId: s.repositoryID, Name: ref.Name().String(), ObjectId: objectID, Target: target})
		if err != nil {
			return xerrors.WithStack(err)
		}
		return nil
	}
	if err != nil {
		return xerrors.WithStack(err)
	}
	current.ObjectId, current.Target = objectID, target
	if err := s.dao.Reference.Update(ctx, current); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

// newReferences converts the rows of the references. The hashes of the objects are read by one query.
func (s *DatabaseStorer) newReferences(ctx context.Context, rows []*database.Reference) ([]*plumbing.Reference, error) {
	var ids []int64
	for _, v := range rows {
		if v.ObjectId != nil {
			ids = append(ids, *v.ObjectId)
		}
	}
	hashes := make(map[int64][]byte)
	if len(ids) > 0 {
		objects, err := s.dao.Object.ListHashesByIds(ctx, s.repositoryID, ids)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		for _, v := range objects {
			hashes[v.Id] = v.Hash
		}
	}

	refs := make([]*plumbing.Reference, len(rows))
	for i, v := range rows {
		name := plumbing.ReferenceName(v.Name)
		if v.Target != nil {
			refs[i] = plumbing.NewSymbolicReference(name, plumbing.ReferenceName(*v.Target))
			continue
		}
		var h plumbing.Hash
		if v.ObjectId != nil {
			copy(h[:], hashes[*v.ObjectId])
		}
		refs[i] = plumbing.NewHashReference(name, h)
	}
	return refs, nil
}

func (s *DatabaseStorer) newObjectWriter(ctx context.Context) *objectWriter {
	return &objectWriter{
		storer:   s,
		ctx:      ctx,
		ids:      make(map[plumbing.Hash]int64),
		stored:   make(map[plumbing.Hash]bool),
		external: make(map[plumbing.Hash]bool),
	}
}

// DatabaseBackend is the Backend which stores the repositories into the database by DatabaseStorer.
// The repository is identified by RepositoryConfig.Name.
type DatabaseBackend struct {
	Conn *sql.DB
}

var _ Backend = (*DatabaseBackend)(nil)

// Open opens the repository. If the clone of the repository is not completed (repository.cloned_at is null), Open
// clones it again. The objects which are stored by the previous clone are reused.
func (b *DatabaseBackend) Open(ctx context.Context, repo *RepositoryConfig, tokenProvider *githubutil.TokenProvider, initTimeout time.Duration) (*git.Repository, error) {
	repositories := dao.NewRepository(b.Conn)
	row, err := repositories.SelectByName(ctx, repo.Name)
	if errors.Is(err, sql.ErrNoRows) {
		row, err = repositories.Create(ctx, &database.Repository{Name: repo.Name, NextObjectId: 1})
	}
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	s := NewDatabaseStorer(b.Conn, row.Id)
	if row.ClonedAt != nil {
		gitRepo, err := git.Open(s, nil)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		return gitRepo, nil
	}

	logger.Log.Info("Init repository", slog.String("name", repo.Name), slog.String("url", repo.URL))
	initCtx, cancel := ctxutil.WithTimeout(ctx, initTimeout)
	defer cancel()
	gitRepo, err := git.Open(s, nil)
	if errors.Is(err, git.ErrRepositoryNotExists) {
		gitRepo, err = git.Init(s, nil)
	}
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	remote, err := gitRepo.Remote(upstreamRemoteName)
	if err == nil && remote.Config().URLs[0] != repo.URL {
		if err := gitRepo.DeleteRemote(upstreamRemoteName); err != nil {
			return nil, xerrors.WithStack(err)
		}
		err = git.ErrRemoteNotFound
	}
	if errors.Is(err, git.ErrRemoteNotFound) {
		remote, err = gitRepo.CreateRemote(&config.RemoteConfig{Name: upstreamRemoteName, URLs: []string{repo.URL}})
	}
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	auth := basicAuth(initCtx, tokenProvider)
	err = gitRepo.FetchContext(initCtx, &git.FetchOptions{RemoteName: upstreamRemoteName, Auth: auth})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil, xerrors.WithMessage(err, "failed to clone the repository")
	}
	remoteRefs, err := remote.ListContext(initCtx, &git.ListOptions{Auth: auth})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	for _, v := range remoteRefs {
		if v.Name() == plumbing.HEAD && v.Type() == plumbing.SymbolicReference {
			if err := s.SetReference(v); err != nil {
				return nil, err
			}
		}
	}
	if err := updateBranches(gitRepo); err != nil {
		return nil, err
	}

	now := time.Now()
	row.ClonedAt = &now
	if err := repositories.Update(ctx, row); err != nil {
		return nil, xerrors.WithStack(err)
	}
	return gitRepo, nil
}

// OpenDatabase opens the MySQL database for DatabaseBackend and waits for the schema of go/git/database to be migrated
// up to timeout.
func OpenDatabase(ctx context.Context, dsn string, timeout time.Duration) (*sql.DB, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	cfg.ParseTime = true
	cfg.Loc = time.Local
	conn, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	p := probe.NewProbe(conn)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	timeoutCh := time.After(timeout)
	for !p.Ready(ctx, database.SchemaHash) {
		logger.Log.Info("Waiting for the migration of git-data")
		select {
		case <-ticker.C:
		case <-timeoutCh:
			conn.Close()
			return nil, xerrors.Define("waiting for the migration of git-data was timed out").WithStack()
		case <-ctx.Done():
			conn.Close()
			return nil, ctx.Err()
		}
	}
	return conn, nil
}

// databasePackfileWriter buffers the received packfile on the local disk, then on Close stores all objects in it.
type databasePackfileWriter struct {
	storer *DatabaseStorer
	fs     billy.Filesystem
	file   billy.File
}

func (w *databasePackfileWriter) Write(p []byte) (int, error) {
	return w.file.Write(p)
}

// Close stores the objects of the packfile. The delta is stored as it is if the base is stored before it in the
// packfile and the depth is under maxDeltaDepth. The other deltas are resolved and stored as the full object.
// The tree is resolved and stored as the rows of the entries.
func (w *databasePackfileWriter) Close() error {
	defer func() {
		_ = w.file.Close()
		_ = w.fs.Remove(w.file.Name())
	}()

	// go-git opens a packfile writer even when the fetch transfers nothing.
	if pos, err := w.file.Seek(0, io.SeekCurrent); err != nil {
		return xerrors.WithStack(err)
	} else if pos == 0 {
		return nil
	}
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return xerrors.WithStack(err)
	}
	idxWriter := new(idxfile.Writer)
	parser, err := packfile.NewParser(packfile.NewScanner(w.file), idxWriter)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if _, err := parser.Parse(); err != nil {
		return xerrors.WithStack(err)
	}
	idx, err := idxWriter.Index()
	if err != nil {
		return xerrors.WithStack(err)
	}

	resolveFile, err := w.fs.Open(w.file.Name())
	if err != nil {
		return xerrors.WithStack(err)
	}
	pack := packfile.NewPackfile(idx, nil, resolveFile, 0)
	defer pack.Close()
	rawFile, err := w.fs.Open(w.file.Name())
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer rawFile.Close()
	scanner := packfile.NewScanner(rawFile)

	entries, err := idx.EntriesByOffset()
	if err != nil {
		return xerrors.WithStack(err)
	}
	var hashes []plumbing.Hash
	var offsets []int64
	for {
		e, err := entries.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return xerrors.WithStack(err)
		}
		hashes = append(hashes, e.Hash)
		offsets = append(offsets, int64(e.Offset))
	}
	if err := entries.Close(); err != nil {
		return xerrors.WithStack(err)
	}

	ow := w.storer.newObjectWriter(context.Background())
	if err := ow.prepare(hashes); err != nil {
		return err
	}
	trees, err := pack.GetByType(plumbing.TreeObject)
	if err != nil {
		return xerrors.WithStack(err)
	}
	var treeContents [][]byte
	err = trees.ForEach(func(obj plumbing.EncodedObject) error {
		if ow.stored[obj.Hash()] {
			return nil
		}
		content, err := readContent(obj)
		if err != nil {
			return err
		}
		treeContents = append(treeContents, content)
		return nil
	})
	if err != nil {
		return xerrors.WithStack(err)
	}
	if err := ow.prepareTrees(treeContents); err != nil {
		return err
	}

	// written is the type and the depth of the objects of the packfile which are written by the offset.
	type packedObject struct {
		typ   plumbing.ObjectType
		depth int32
	}
	written := make(map[int64]packedObject)
	var buf bytes.Buffer
	for i, h := range hashes {
		if ow.stored[h] {
			continue
		}
		header, err := scanner.SeekObjectHeader(offsets[i])
		if err != nil {
			return xerrors.WithStack(err)
		}
		buf.Reset()
		if _, _, err := scanner.NextObject(&buf); err != nil {
			return xerrors.WithStack(err)
		}

		if !header.Type.IsDelta() {
			if err := ow.add(h, header.Type, bytes.Clone(buf.Bytes())); err != nil {
				return err
			}
			written[offsets[i]] = packedObject{typ: header.Type}
			continue
		}

		baseOffset := header.OffsetReference
		if header.Type == plumbing.REFDeltaObject {
			o, err := idx.FindOffset(header.Reference)
			if err != nil {
				return xerrors.WithStack(err)
			}
			baseOffset = o
		}
		if base, ok := written[baseOffset]; ok && base.typ != plumbing.TreeObject && base.depth < maxDeltaDepth {
			size, err := pack.GetSizeByOffset(offsets[i])
			if err != nil {
				return xerrors.WithStack(err)
			}
			baseHash, err := idx.FindHash(baseOffset)
			if err != nil {
				return xerrors.WithStack(err)
			}
			if err := ow.addDelta(h, base.typ, size, ow.ids[baseHash], base.depth+1, bytes.Clone(buf.Bytes())); err != nil {
				return err
			}
			written[offsets[i]] = packedObject{typ: base.typ, depth: base.depth + 1}
			continue
		}

		obj, err := pack.GetByOffset(offsets[i])
		if err != nil {
			return xerrors.WithStack(err)
		}
		content, err := readContent(obj)
		if err != nil {
			return err
		}
		if err := ow.add(h, obj.Type(), content); err != nil {
			return err
		}
		written[offsets[i]] = packedObject{typ: obj.Type()}
	}
	return ow.flush()
}

// objectWriter writes the objects with the multi-row INSERTs. The IDs of all objects are assigned before writing so
// that the tree and the delta can refer to the objects which are written after them.
type objectWriter struct {
	storer *DatabaseStorer
	ctx    context.Context
	// ids is the IDs of the objects which are stored or will be stored.
	ids map[plumbing.Hash]int64
	// stored is the objects which are already stored. The external objects are not included.
	stored map[plumbing.Hash]bool
	// external is the objects which are stored as the external object.
	external map[plumbing.Hash]bool

	objects     []*database.Object
	objectBytes int
	entries     []*database.TreeEntry
}

// prepare assigns the IDs to the objects which will be written.
func (w *objectWriter) prepare(hashes []plumbing.Hash) error {
	if err := w.lookup(hashes); err != nil {
		return err
	}
	return w.allocate(hashes)
}

// prepareTrees assigns the IDs to the children of the trees. The children which are neither stored nor prepared are
// written as the external objects.
func (w *objectWriter) prepareTrees(contents [][]byte) error {
	children := make(map[plumbing.Hash]plumbing.ObjectType)
	var hashes []plumbing.Hash
	for _, content := range contents {
		entries, ok := parseTree(content)
		if !ok {
			continue
		}
		for _, e := range entries {
			if _, ok := w.ids[e.hash]; ok {
				continue
			}
			if _, ok := children[e.hash]; ok {
				continue
			}
			children[e.hash] = e.objectType()
			hashes = append(hashes, e.hash)
		}
	}
	if err := w.lookup(hashes); err != nil {
		return err
	}
	var missing []plumbing.Hash
	for _, h := range hashes {
		if _, ok := w.ids[h]; !ok {
			missing = append(missing, h)
		}
	}
	if err := w.allocate(missing); err != nil {
		return err
	}
	for _, h := range missing {
		if err := w.write(&database.Object{Hash: bytes.Clone(h[:]), Type: database.ObjectType(children[h]), Encoding: database.ObjectEncodingExternal}); err != nil {
			return err
		}
	}
	return nil
}

// add writes the object of the content. The tree is written as the rows of the entries if the content can be built
// from the rows. The ID of the object and the children of the tree must be prepared.
func (w *objectWriter) add(h plumbing.Hash, typ plumbing.ObjectType, content []byte) error {
	if typ == plumbing.TreeObject {
		if entries, ok := parseTree(content); ok {
			id := w.ids[h]
			for i, e := range entries {
				w.entries = append(w.entries, &database.TreeEntry{
					RepositoryId: w.storer.repositoryID,
					TreeId:       id,
					Position:     int32(i),
					Name:         e.name,
					Mode:         e.mode,
					ChildId:      w.ids[e.hash],
				})
			}
			return w.write(&database.Object{
				Hash:     bytes.Clone(h[:]),
				Type:     database.ObjectType(typ),
				Encoding: database.ObjectEncodingTree,
				Size:     int64(len(content)),
			})
		}
	}
	return w.write(&database.Object{
		Hash:     bytes.Clone(h[:]),
		Type:     database.ObjectType(typ),
		Encoding: database.ObjectEncodingFull,
		Size:     int64(len(content)),
		Payload:  zstdEncoder.EncodeAll(content, nil),
	})
}

func (w *objectWriter) addDelta(h plumbing.Hash, typ plumbing.ObjectType, size, baseID int64, depth int32, delta []byte) error {
	return w.write(&database.Object{
		Hash:     bytes.Clone(h[:]),
		Type:     database.ObjectType(typ),
		Encoding: database.ObjectEncodingDelta,
		Size:     size,
		BaseId:   &baseID,
		Depth:    depth,
		Payload:  zstdEncoder.EncodeAll(delta, nil),
	})
}

// write buffers the new object. The object which is stored as the external object is replaced at once.
func (w *objectWriter) write(obj *database.Object) error {
	var h plumbing.Hash
	copy(h[:], obj.Hash)
	obj.RepositoryId, obj.Id = w.storer.repositoryID, w.ids[h]
	if obj.Payload == nil {
		obj.Payload = []byte{}
	}
	if w.external[h] {
		current, err := w.storer.dao.Object.Select(w.ctx, w.storer.repositoryID, obj.Id)
		if err != nil {
			return xerrors.WithStack(err)
		}
		current.Type, current.Encoding, current.Size, current.BaseId, current.Depth, current.Payload = obj.Type, obj.Encoding, obj.Size, obj.BaseId, obj.Depth, obj.Payload
		if err := w.storer.dao.Object.Update(w.ctx, current); err != nil {
			return xerrors.WithStack(err)
		}
		delete(w.external, h)
		return nil
	}

	if len(w.objects) >= maxBufferedRows || w.objectBytes+len(obj.Payload) > maxBufferedBytes {
		if err := w.flush(); err != nil {
			return err
		}
	}
	w.objects = append(w.objects, obj)
	w.objectBytes += len(obj.Payload)
	return nil
}

// flush writes the buffered rows. The entries are written before the objects so that the tree is readable when the
// object is written.
func (w *objectWriter) flush() error {
	if len(w.entries) > 0 {
		if err := w.storer.dao.TreeEntry.CreateMulti(w.ctx, w.entries); err != nil {
			return xerrors.WithStack(err)
		}
		w.entries = w.entries[:0]
	}
	if len(w.objects) > 0 {
		if err := w.storer.dao.Object.CreateMulti(w.ctx, w.objects); err != nil {
			return xerrors.WithStack(err)
		}
		w.objects, w.objectBytes = w.objects[:0], 0
	}
	return nil
}

// lookup reads the IDs of the stored objects.
func (w *objectWriter) lookup(hashes []plumbing.Hash) error {
	var targets [][]byte
	for _, h := range hashes {
		if _, ok := w.ids[h]; !ok {
			targets = append(targets, h[:])
		}
	}
	for len(targets) > 0 {
		n := min(len(targets), maxLookupHashes)
		rows, err := w.storer.dao.Object.ListIdsByHashes(w.ctx, w.storer.repositoryID, targets[:n])
		if err != nil {
			return xerrors.WithStack(err)
		}
		for _, v := range rows {
			var h plumbing.Hash
			copy(h[:], v.Hash)
			w.ids[h] = v.Id
			if v.Encoding == database.ObjectEncodingExternal {
				w.external[h] = true
			} else {
				w.stored[h] = true
			}
		}
		targets = targets[n:]
	}
	return nil
}

// allocate assigns the new IDs to the objects which don't have the ID. The IDs are reserved by advancing
// repository.next_object_id. The row is not locked because the storer has only one writer. If the IDs are reserved by
// two writers at the same time, writing the objects fails by the duplicated primary key.
func (w *objectWriter) allocate(hashes []plumbing.Hash) error {
	var targets []plumbing.Hash
	seen := make(map[plumbing.Hash]struct{})
	for _, h := range hashes {
		if _, ok := w.ids[h]; ok {
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		targets = append(targets, h)
	}
	if len(targets) == 0 {
		return nil
	}

	repo, err := w.storer.dao.Repository.Select(w.ctx, w.storer.repositoryID)
	if err != nil {
		return xerrors.WithStack(err)
	}
	first := repo.NextObjectId
	repo.NextObjectId += int64(len(targets))
	if err := w.storer.dao.Repository.Update(w.ctx, repo); err != nil {
		return xerrors.WithStack(err)
	}
	for i, h := range targets {
		w.ids[h] = first + int64(i)
	}
	return nil
}

type treeEntry struct {
	mode int32
	name []byte
	hash plumbing.Hash
}

func (e *treeEntry) objectType() plumbing.ObjectType {
	switch filemode.FileMode(e.mode) {
	case filemode.Dir:
		return plumbing.TreeObject
	case filemode.Submodule:
		return plumbing.CommitObject
	default:
		return plumbing.BlobObject
	}
}

func (e *treeEntry) encode(buf *bytes.Buffer) {
	buf.WriteString(strconv.FormatInt(int64(e.mode), 8))
	buf.WriteByte(' ')
	buf.Write(e.name)
	buf.WriteByte(0)
	buf.Write(e.hash[:])
}

// parseTree parses the content of the tree. The second return value is false if the content is malformed or is not
// the same as the content which is built from the entries (e.g. the mode which has the leading zero).
func parseTree(content []byte) ([]treeEntry, bool) {
	var entries []treeEntry
	rest := content
	for len(rest) > 0 {
		sp := bytes.IndexByte(rest, ' ')
		if sp <= 0 {
			return nil, false
		}
		mode, err := strconv.ParseInt(string(rest[:sp]), 8, 32)
		if err != nil || strconv.FormatInt(mode, 8) != string(rest[:sp]) {
			return nil, false
		}
		rest = rest[sp+1:]
		nul := bytes.IndexByte(rest, 0)
		if nul < 0 || len(rest) < nul+1+len(plumbing.ZeroHash) {
			return nil, false
		}
		e := treeEntry{mode: int32(mode), name: rest[:nul]}
		copy(e.hash[:], rest[nul+1:])
		entries = append(entries, e)
		rest = rest[nul+1+len(plumbing.ZeroHash):]
	}
	return entries, true
}

func readContent(obj plumbing.EncodedObject) ([]byte, error) {
	r, err := obj.Reader()
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	defer r.Close()
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return content, nil
}
