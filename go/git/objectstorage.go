package git

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/idxfile"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/format/objfile"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gitStorage "github.com/go-git/go-git/v5/storage"
	"go.f110.dev/go-memcached/client"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/collections/dict"
	"go.f110.dev/mono/go/collections/set"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/storage"
)

type ObjectStorageInterface interface {
	PutReader(ctx context.Context, name string, data io.Reader) error
	Delete(ctx context.Context, name string) error
	Get(ctx context.Context, name string) (*storage.Object, error)
	List(ctx context.Context, prefix string) ([]*storage.Object, error)
}

func InitObjectStorageRepository(ctx context.Context, b ObjectStorageInterface, url, prefix string, auth *http.BasicAuth) (*git.Repository, error) {
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, err
	}
	defer func() {
		os.RemoveAll(tmpDir)
	}()

	_, err = git.PlainClone(tmpDir, false, &git.CloneOptions{
		URL:        url,
		NoCheckout: true,
		Auth:       auth,
	})
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to clone the repository")
	}
	err = filepath.Walk(filepath.Join(tmpDir, ".git"), func(p string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		name := strings.TrimPrefix(p, filepath.Join(tmpDir, ".git")+"/")
		file, err := os.Open(p)
		if err != nil {
			return err
		}
		err = b.PutReader(ctx, prefix+"/"+name, file)
		if err != nil {
			return err
		}
		file.Close()
		return nil
	})
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to walk .git dir")
	}

	s := NewObjectStorageStorer(b, prefix, nil, nil)
	repo, err := git.Open(s, nil)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to open the repository")
	}
	slogger.Log.Debug("Start inflate packfile", slog.String("path", prefix))
	if err := InflatePackFile(ctx, b, prefix, repo); err != nil {
		return nil, xerrors.WithMessage(err, "failed to inflate pack file")
	}

	return repo, nil
}

func InflatePackFile(ctx context.Context, st ObjectStorageInterface, rootPath string, repo *git.Repository) error {
	packFiles, err := st.List(ctx, path.Join(rootPath, "objects/pack"))
	if err != nil {
		return err
	}
	var packs []plumbing.Hash
	for _, v := range packFiles {
		n := filepath.Base(v.Name)
		if filepath.Ext(n) == ".pack" {
			packs = append(packs, plumbing.NewHash(n[5:len(n)-5]))
		}
	}

	for _, v := range packs {
		slogger.Log.Debug("Inflate packfile", slog.String("file", fmt.Sprintf("pack-%s.idx", v.String())))
		file, err := st.Get(ctx, filepath.Join(rootPath, fmt.Sprintf("objects/pack/pack-%s.idx", v.String())))
		if err != nil {
			return err
		}
		idx := idxfile.NewMemoryIndex()
		if err := idxfile.NewDecoder(file.Body).Decode(idx); err != nil {
			return err
		}
		if err := file.Body.Close(); err != nil {
			return err
		}

		file, err = st.Get(ctx, filepath.Join(rootPath, fmt.Sprintf("objects/pack/pack-%s.pack", v.String())))
		if err != nil {
			return err
		}
		buf, err := io.ReadAll(file.Body)
		if err != nil {
			return err
		}
		if err := file.Body.Close(); err != nil {
			return err
		}
		packfileReader := packfile.NewPackfile(idx, nil, newBufferedFile(fmt.Sprintf("objects/pack/pack-%s.pack", v.String()), buf), 0)

		objs, err := packfileReader.GetAll()
		if err != nil {
			return xerrors.WithStack(err)
		}

		for {
			obj, err := objs.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return xerrors.WithStack(err)
			}
			if _, err := repo.Storer.SetEncodedObject(obj); err != nil {
				return err
			}
		}

		slogger.Log.Debug("Delete idx file", slog.String("file", fmt.Sprintf("pack-%s.idx", v.String())))
		if err := st.Delete(ctx, path.Join(rootPath, fmt.Sprintf("objects/pack/pack-%s.idx", v.String()))); err != nil {
			return err
		}
		slogger.Log.Debug("Delete pack file", slog.String("file", fmt.Sprintf("pack-%s.pack", v.String())))
		if err := st.Delete(ctx, path.Join(rootPath, fmt.Sprintf("objects/pack/pack-%s.pack", v.String()))); err != nil {
			return err
		}
	}

	return nil
}

type ObjectStorageStorer struct {
	backend   ObjectStorageInterface
	rootPath  string
	cachePool *client.SinglePool
	// packCache keeps fetched packfiles (idx + raw pack bytes) in process memory so
	// that resolving many objects out of the same pack does not re-download it from
	// the backend for every single object. It is shared across the storers created
	// for sub-modules. It may be nil, in which case packs are fetched every time.
	packCache *PackfileCache
}

// packEntry is a fetched packfile held in a PackfileCache. pack is the raw .pack
// bytes and bounds the cached size; idx is its decoded index.
type packEntry struct {
	idx  *idxfile.MemoryIndex
	pack []byte
}

func packEntrySize(e *packEntry) int64 { return int64(len(e.pack)) }

// PackfileCache is the process-level cache of fetched packfiles shared across the
// ObjectStorageStorer instances of one process.
type PackfileCache = dict.TTLCache[string, *packEntry]

// NewPackfileCache creates a PackfileCache. maxBytes bounds the total size of the
// cached pack bytes, which is what lets the caller cap the memory it uses. A
// positive sweepInterval starts a background goroutine that reclaims expired
// packs; call Close to stop it.
func NewPackfileCache(ttl, sweepInterval time.Duration, maxBytes int64) *PackfileCache {
	return dict.NewTTLCache[string, *packEntry](ttl, sweepInterval, maxBytes, packEntrySize)
}

var _ gitStorage.Storer = &ObjectStorageStorer{}

func NewObjectStorageStorer(b ObjectStorageInterface, rootPath string, cachePool *client.SinglePool, packCache *PackfileCache) *ObjectStorageStorer {
	return &ObjectStorageStorer{backend: b, rootPath: rootPath, cachePool: cachePool, packCache: packCache}
}

func (b *ObjectStorageStorer) EnabledCache() bool {
	return b.cachePool != nil
}

func (b *ObjectStorageStorer) Exist() (bool, error) {
	_, err := b.backend.Get(context.Background(), path.Join(b.rootPath, "config"))
	if err != nil && errors.Is(err, storage.ErrObjectNotFound) {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (b *ObjectStorageStorer) Module(name string) (gitStorage.Storer, error) {
	return NewObjectStorageStorer(b.backend, path.Join(b.rootPath, name), b.cachePool, b.packCache), nil
}

func (b *ObjectStorageStorer) IncludePackFile(ctx context.Context) bool {
	packFiles, err := b.backend.List(ctx, path.Join(b.rootPath, "objects/pack-"))
	if err != nil {
		return false
	}

	if len(packFiles) > 0 {
		return true
	}
	return false
}

func (b *ObjectStorageStorer) Config() (*config.Config, error) {
	file, err := b.backend.Get(context.Background(), path.Join(b.rootPath, "config"))
	if errors.Is(err, storage.ErrObjectNotFound) {
		return config.NewConfig(), nil
	}
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	conf, err := config.ReadConfig(file.Body)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	if err := file.Body.Close(); err != nil {
		return nil, xerrors.WithStack(err)
	}
	return conf, nil
}

func (b *ObjectStorageStorer) SetConfig(conf *config.Config) error {
	buf, err := conf.Marshal()
	if err != nil {
		return xerrors.WithStack(err)
	}

	if err := b.backend.PutReader(context.Background(), path.Join(b.rootPath, "config"), bytes.NewReader(buf)); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (b *ObjectStorageStorer) SetIndex(idx *index.Index) error {
	buf := new(bytes.Buffer)
	if err := index.NewEncoder(buf).Encode(idx); err != nil {
		return xerrors.WithStack(err)
	}

	if err := b.backend.PutReader(context.Background(), path.Join(b.rootPath, "index"), buf); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (b *ObjectStorageStorer) Index() (*index.Index, error) {
	file, err := b.backend.Get(context.Background(), path.Join(b.rootPath, "index"))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	idx := &index.Index{Version: 2}
	if err := index.NewDecoder(file.Body).Decode(idx); err != nil {
		return nil, xerrors.WithStack(err)
	}
	if err := file.Body.Close(); err != nil {
		return nil, xerrors.WithStack(err)
	}
	return idx, nil
}

func (b *ObjectStorageStorer) SetShallow(commits []plumbing.Hash) error {
	buf := new(bytes.Buffer)
	for _, h := range commits {
		if _, err := fmt.Fprintf(buf, "%s\n", h); err != nil {
			return xerrors.WithStack(err)
		}
	}

	if err := b.backend.PutReader(context.Background(), path.Join(b.rootPath, "shallow"), buf); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (b *ObjectStorageStorer) Shallow() ([]plumbing.Hash, error) {
	file, err := b.backend.Get(context.Background(), path.Join(b.rootPath, "shallow"))
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return nil, nil
		}
		return nil, xerrors.WithStack(err)
	}

	var hash []plumbing.Hash
	s := bufio.NewScanner(file.Body)
	for s.Scan() {
		hash = append(hash, plumbing.NewHash(s.Text()))
	}
	if err := file.Body.Close(); err != nil {
		return nil, xerrors.WithStack(err)
	}
	if err := s.Err(); err != nil {
		return nil, xerrors.WithStack(err)
	}
	return hash, nil
}

func (b *ObjectStorageStorer) SetReference(ref *plumbing.Reference) error {
	buf := new(bytes.Buffer)
	switch ref.Type() {
	case plumbing.SymbolicReference:
		if _, err := fmt.Fprintf(buf, "ref: %s\n", ref.Target()); err != nil {
			return xerrors.WithStack(err)
		}
	case plumbing.HashReference:
		if _, err := fmt.Fprintln(buf, ref.Hash().String()); err != nil {
			return xerrors.WithStack(err)
		}
	}

	if err := b.backend.PutReader(context.Background(), path.Join(b.rootPath, ref.Name().String()), buf); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (b *ObjectStorageStorer) CheckAndSetReference(new, old *plumbing.Reference) error {
	if old != nil {
		file, err := b.backend.Get(context.Background(), path.Join(b.rootPath, old.Name().String()))
		if err != nil {
			return xerrors.WithStack(err)
		}

		oldRef, err := b.readReference(file.Body, old.Name().String())
		if err != nil {
			return xerrors.WithStack(err)
		}
		if oldRef.Hash() != old.Hash() {
			return xerrors.Define("reference has changed concurrently").WithStack()
		}
	}

	if err := b.SetReference(new); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (b *ObjectStorageStorer) Reference(name plumbing.ReferenceName) (*plumbing.Reference, error) {
	file, err := b.backend.Get(context.Background(), path.Join(b.rootPath, name.String()))
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return nil, plumbing.ErrReferenceNotFound
		}
		return nil, xerrors.WithStack(err)
	}
	ref, err := b.readReference(file.Body, name.String())
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return ref, nil
}

func (b *ObjectStorageStorer) readReference(f io.ReadCloser, name string) (*plumbing.Reference, error) {
	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	if err := f.Close(); err != nil {
		return nil, xerrors.WithStack(err)
	}
	ref := plumbing.NewReferenceFromStrings(name, strings.TrimSpace(string(buf)))
	return ref, nil
}

func (b *ObjectStorageStorer) IterReferences() (storer.ReferenceIter, error) {
	var refs []*plumbing.Reference
	mark := make(map[plumbing.ReferenceName]struct{})

	// Find refs
	r, err := b.readRefs()
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	for _, v := range r {
		if _, ok := mark[v.Name()]; !ok {
			refs = append(refs, v)
			mark[v.Name()] = struct{}{}
		}
	}

	// Find packed-refs
	packedRefs, err := b.readPackedRefs()
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	for _, v := range packedRefs {
		if _, ok := mark[v.Name()]; !ok {
			refs = append(refs, v)
			mark[v.Name()] = struct{}{}
		}
	}

	// Read HEAD
	ref, err := b.readHEAD()
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	refs = append(refs, ref)

	return storer.NewReferenceSliceIter(refs), nil
}

func (b *ObjectStorageStorer) readRefs() ([]*plumbing.Reference, error) {
	var refs []*plumbing.Reference

	objs, err := b.backend.List(context.Background(), path.Join(b.rootPath, "refs"))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	for _, v := range objs {
		file, err := b.backend.Get(context.Background(), v.Name)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		ref, err := b.readReference(file.Body, strings.TrimPrefix(v.Name, b.rootPath+"/"))
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		refs = append(refs, ref)
	}

	return refs, nil
}

func (b *ObjectStorageStorer) readPackedRefs() ([]*plumbing.Reference, error) {
	var refs []*plumbing.Reference

	file, err := b.backend.Get(context.Background(), path.Join(b.rootPath, "packed-refs"))
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return refs, nil
		}
		return nil, xerrors.WithStack(err)
	}
	s := bufio.NewScanner(file.Body)
	for s.Scan() {
		ref, err := b.parsePackedRefsLine(s.Text())
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		if refs != nil {
			refs = append(refs, ref)
		}
	}
	if err := file.Body.Close(); err != nil {
		return nil, xerrors.WithStack(err)
	}

	return refs, nil
}

func (b *ObjectStorageStorer) parsePackedRefsLine(line string) (*plumbing.Reference, error) {
	switch line[0] {
	case '#', '^':
	default:
		v := strings.Split(line, " ")
		if len(v) != 2 {
			return nil, xerrors.New("git: malformed packed-ref")
		}
		return plumbing.NewReferenceFromStrings(v[1], v[0]), nil
	}

	return nil, nil
}

func (b *ObjectStorageStorer) readHEAD() (*plumbing.Reference, error) {
	file, err := b.backend.Get(context.Background(), path.Join(b.rootPath, "HEAD"))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	ref, err := b.readReference(file.Body, "HEAD")
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return ref, nil
}

func (b *ObjectStorageStorer) RemoveReference(name plumbing.ReferenceName) error {
	err := b.backend.Delete(context.Background(), path.Join(b.rootPath, name.String()))
	if err != nil {
		return xerrors.WithStack(err)
	}

	file, err := b.backend.Get(context.Background(), path.Join(b.rootPath, "packed-refs"))
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return nil
		}
		return xerrors.WithStack(err)
	}
	s := bufio.NewScanner(file.Body)
	found := false
	newPackedRefs := new(bytes.Buffer)
	for s.Scan() {
		line := s.Text()
		ref, err := b.parsePackedRefsLine(line)
		if err != nil {
			return xerrors.WithStack(err)
		}
		if ref != nil {
			if ref.Name() == name {
				found = true
				continue
			}
		}
		if _, err := newPackedRefs.WriteString(line); err != nil {
			return xerrors.WithStack(err)
		}
	}
	if err := file.Body.Close(); err != nil {
		return xerrors.WithStack(err)
	}

	if !found {
		// No need to update packed-refs
		return nil
	}

	return b.backend.PutReader(context.Background(), path.Join(b.rootPath, "packed-refs"), newPackedRefs)
}

func (b *ObjectStorageStorer) CountLooseRefs() (int, error) {
	objs, err := b.backend.List(context.Background(), path.Join(b.rootPath, "refs"))
	if err != nil {
		return -1, xerrors.WithStack(err)
	}
	var count int
	mark := make(map[plumbing.ReferenceName]struct{})
	for _, v := range objs {
		file, err := b.backend.Get(context.Background(), v.Name)
		if err != nil {
			return -1, xerrors.WithStack(err)
		}
		ref, err := b.readReference(file.Body, strings.TrimPrefix(v.Name, path.Join(b.rootPath, "refs")))
		if err != nil {
			return -1, xerrors.WithStack(err)
		}
		if _, ok := mark[ref.Name()]; !ok {
			count++
			mark[ref.Name()] = struct{}{}
		}
	}
	return count, nil
}

func (b *ObjectStorageStorer) PackRefs() error {
	r, err := b.readRefs()
	if err != nil {
		return xerrors.WithStack(err)
	}
	if len(r) == 0 {
		return nil
	}
	refSet := set.New[*plumbing.Reference]()
	for _, v := range r {
		refSet.Add(v)
	}

	packedRefs, err := b.readPackedRefs()
	if err != nil {
		return xerrors.WithStack(err)
	}
	packedRefsSet := set.New[*plumbing.Reference]()
	for _, v := range packedRefs {
		refSet.Add(v)
		packedRefsSet.Add(v)
	}

	buf := new(bytes.Buffer)
	for _, ref := range refSet.ToSlice() {
		if _, err := fmt.Fprintln(buf, ref.String()); err != nil {
			return xerrors.WithStack(err)
		}
	}
	err = b.backend.PutReader(context.Background(), path.Join(b.rootPath, "packed-refs"), buf)
	if err != nil {
		return xerrors.WithStack(err)
	}

	// Delete all loose refs.
	looseRefs := refSet.RightOuter(packedRefsSet)
	for _, ref := range looseRefs.ToSlice() {
		err := b.backend.Delete(context.Background(), path.Join(b.rootPath, ref.Name().String()))
		if err != nil {
			return xerrors.WithStack(err)
		}
	}
	return nil
}

func (b *ObjectStorageStorer) NewEncodedObject() plumbing.EncodedObject {
	return &EncodedObject{}
}

func (b *ObjectStorageStorer) SetEncodedObject(e plumbing.EncodedObject) (plumbing.Hash, error) {
	switch e.Type() {
	case plumbing.OFSDeltaObject, plumbing.REFDeltaObject:
		return plumbing.ZeroHash, plumbing.ErrInvalidType
	}

	buf := new(bytes.Buffer)
	w := objfile.NewWriter(buf)
	if err := w.WriteHeader(e.Type(), e.Size()); err != nil {
		return plumbing.ZeroHash, xerrors.WithStack(err)
	}
	r, err := e.Reader()
	if err != nil {
		return plumbing.ZeroHash, xerrors.WithStack(err)
	}
	if _, err := io.Copy(w, r); err != nil {
		return plumbing.ZeroHash, xerrors.WithStack(err)
	}
	if err := w.Close(); err != nil {
		return plumbing.ZeroHash, xerrors.WithStack(err)
	}

	hash := w.Hash().String()
	err = b.backend.PutReader(context.Background(), path.Join(b.rootPath, "objects", hash[0:2], hash[2:40]), buf)
	if err != nil {
		return plumbing.ZeroHash, xerrors.WithStack(err)
	}

	return e.Hash(), nil
}

// PackfileWriter implements storer.PackfileWriter. When this is present go-git
// streams a received packfile straight into the returned writer instead of
// exploding it into individual loose objects. That turns a fetch of N objects
// from N backend writes (one PutReader per object) into a single pack upload,
// which is what keeps a large fetch within the updater's fetch timeout.
func (b *ObjectStorageStorer) PackfileWriter() (io.WriteCloser, error) {
	tmp, err := os.CreateTemp("", "objectstorage-pack-")
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return &packfileWriter{backend: b.backend, rootPath: b.rootPath, tmp: tmp}, nil
}

// packfileWriter buffers the incoming packfile on local disk, then on Close
// builds its index and uploads the pack/idx pair to the backend.
type packfileWriter struct {
	backend  ObjectStorageInterface
	rootPath string
	tmp      *os.File
}

func (w *packfileWriter) Write(p []byte) (int, error) {
	return w.tmp.Write(p)
}

func (w *packfileWriter) Close() error {
	name := w.tmp.Name()
	defer func() {
		_ = w.tmp.Close()
		_ = os.Remove(name)
	}()

	info, err := w.tmp.Stat()
	if err != nil {
		return xerrors.WithStack(err)
	}
	// go-git opens a packfile writer even when the fetch transfers nothing.
	if info.Size() == 0 {
		return nil
	}

	if _, err := w.tmp.Seek(0, io.SeekStart); err != nil {
		return xerrors.WithStack(err)
	}
	idxWriter := new(idxfile.Writer)
	parser, err := packfile.NewParser(packfile.NewScanner(w.tmp), idxWriter)
	if err != nil {
		return xerrors.WithStack(err)
	}
	checksum, err := parser.Parse()
	if err != nil {
		return xerrors.WithStack(err)
	}
	idx, err := idxWriter.Index()
	if err != nil {
		return xerrors.WithStack(err)
	}
	if count, err := idx.Count(); err != nil {
		return xerrors.WithStack(err)
	} else if count == 0 {
		return nil
	}

	var idxBuf bytes.Buffer
	if _, err := idxfile.NewEncoder(&idxBuf).Encode(idx); err != nil {
		return xerrors.WithStack(err)
	}

	base := path.Join(w.rootPath, "objects/pack", fmt.Sprintf("pack-%s", checksum.String()))
	// Store the pack before the idx: getEncodedObjectFromPackFile keys off the
	// idx, so a crash between the two writes leaves an orphan pack that is simply
	// ignored rather than an index pointing at a missing pack.
	if _, err := w.tmp.Seek(0, io.SeekStart); err != nil {
		return xerrors.WithStack(err)
	}
	if err := w.backend.PutReader(context.Background(), base+".pack", w.tmp); err != nil {
		return xerrors.WithStack(err)
	}
	if err := w.backend.PutReader(context.Background(), base+".idx", &idxBuf); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (b *ObjectStorageStorer) EncodedObject(objectType plumbing.ObjectType, hash plumbing.Hash) (plumbing.EncodedObject, error) {
	obj, err := b.getUnpackedEncodedObject(hash)
	if errors.Is(err, plumbing.ErrObjectNotFound) {
		obj, err = b.getEncodedObjectFromPackFile(hash)
	}
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (b *ObjectStorageStorer) getEncodedObjectFromPackFile(h plumbing.Hash) (plumbing.EncodedObject, error) {
	if b.EnabledCache() {
		item, err := b.cachePool.Get(fmt.Sprintf("objects/%s", h.String()))
		if err == nil {
			obj := b.NewEncodedObject()
			if err := json.Unmarshal(item.Value, obj); err == nil {
				return obj, nil
			}
		}
	}

	packFiles, err := b.backend.List(context.Background(), path.Join(b.rootPath, "objects/pack"))
	if err != nil {
		return nil, err
	}
	var packs []plumbing.Hash
	for _, f := range packFiles {
		n := filepath.Base(f.Name)
		if filepath.Ext(n) != ".pack" || !strings.HasPrefix(n, "pack-") {
			continue
		}
		h := plumbing.NewHash(n[5 : len(n)-5])
		if h.IsZero() {
			continue
		}
		packs = append(packs, h)
	}

	var entry *packEntry
	var packHash plumbing.Hash
	// Find the pack that contains the object via its index.
	for _, v := range packs {
		e, err := b.loadPack(v)
		if err != nil {
			return nil, err
		}
		if _, err := e.idx.FindOffset(h); err == nil {
			entry = e
			packHash = v
			break
		}
	}
	if entry == nil {
		return nil, plumbing.ErrObjectNotFound
	}

	// Fetch the object from the (cached) packfile bytes.
	packfileReader := packfile.NewPackfile(entry.idx, nil, newBufferedFile(fmt.Sprintf("objects/pack/pack-%s.pack", packHash.String()), entry.pack), 0)
	obj, err := packfileReader.Get(h)
	_ = packfileReader.Close()
	if err == nil {
		if b.EnabledCache() &&
			(obj.Type() == plumbing.TreeObject ||
				obj.Type() == plumbing.CommitObject ||
				obj.Type() == plumbing.OFSDeltaObject ||
				obj.Type() == plumbing.REFDeltaObject) {
			buf, err := json.Marshal(obj)
			if err == nil {
				// No need to handle an error
				_ = b.cachePool.Set(&client.Item{
					Key:        fmt.Sprintf("objects/%s", h.String()),
					Value:      buf,
					Expiration: 60 * 60 * 24, // 1 day
				})
			}
		}
		return obj, nil
	}

	return nil, plumbing.ErrObjectNotFound
}

// loadPack fetches a packfile (its index and raw bytes) from the backend. When a
// packCache is configured the result is cached in process memory and concurrent
// loads of the same pack are coalesced into a single fetch.
func (b *ObjectStorageStorer) loadPack(v plumbing.Hash) (*packEntry, error) {
	load := func() (*packEntry, error) {
		idxFile, err := b.backend.Get(context.Background(), filepath.Join(b.rootPath, fmt.Sprintf("objects/pack/pack-%s.idx", v.String())))
		if err != nil {
			return nil, err
		}
		idx := idxfile.NewMemoryIndex()
		if err := idxfile.NewDecoder(idxFile.Body).Decode(idx); err != nil {
			return nil, err
		}
		if err := idxFile.Body.Close(); err != nil {
			return nil, err
		}

		packFile, err := b.backend.Get(context.Background(), filepath.Join(b.rootPath, fmt.Sprintf("objects/pack/pack-%s.pack", v.String())))
		if err != nil {
			return nil, err
		}
		buf, err := io.ReadAll(packFile.Body)
		if err != nil {
			return nil, err
		}
		if err := packFile.Body.Close(); err != nil {
			return nil, err
		}
		return &packEntry{idx: idx, pack: buf}, nil
	}

	if b.packCache == nil {
		return load()
	}
	return b.packCache.GetOrLoad(filepath.Join(b.rootPath, fmt.Sprintf("objects/pack/pack-%s", v.String())), load)
}

func (b *ObjectStorageStorer) getUnpackedEncodedObject(h plumbing.Hash) (plumbing.EncodedObject, error) {
	if b.EnabledCache() {
		item, err := b.cachePool.Get(fmt.Sprintf("objects/%s", h.String()))
		if err == nil {
			obj := b.NewEncodedObject()
			if err := json.Unmarshal(item.Value, obj); err == nil {
				return obj, nil
			}
		}
	}

	file, err := b.backend.Get(context.Background(), path.Join(b.rootPath, "objects", h.String()[0:2], h.String()[2:40]))
	if err != nil && errors.Is(err, storage.ErrObjectNotFound) {
		return nil, plumbing.ErrObjectNotFound
	}
	if err != nil {
		return nil, err
	}

	obj, err := b.readUnpackedEncodedObject(file.Body, h)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	if b.EnabledCache() &&
		(obj.Type() == plumbing.TreeObject ||
			obj.Type() == plumbing.CommitObject ||
			obj.Type() == plumbing.OFSDeltaObject ||
			obj.Type() == plumbing.REFDeltaObject) {
		buf, err := json.Marshal(obj)
		if err == nil {
			_ = b.cachePool.Set(&client.Item{
				Key:        fmt.Sprintf("objects/%s", h.String()),
				Value:      buf,
				Expiration: 60 * 60 * 24, // 1 day
			})
		}
	}
	return obj, nil
}

func (b *ObjectStorageStorer) readUnpackedEncodedObject(f io.ReadCloser, h plumbing.Hash) (plumbing.EncodedObject, error) {
	obj := b.NewEncodedObject().(*EncodedObject)
	r, err := objfile.NewReader(f)
	if err != nil {
		return nil, err
	}
	typ, size, err := r.Header()
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	obj.hash = h
	obj.SetType(typ)
	obj.SetSize(size)
	obj.SetReader(r)

	return obj, nil
}

func (b *ObjectStorageStorer) IterEncodedObjects(objectType plumbing.ObjectType) (storer.EncodedObjectIter, error) {
	objs, err := b.backend.List(context.Background(), path.Join(b.rootPath, "objects"))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	var encodedObjs []plumbing.EncodedObject
	for _, v := range objs {
		s := strings.Split(strings.TrimPrefix(v.Name, b.rootPath), "/")
		if len(s[2]) != 2 || len(s[3]) != 38 {
			continue
		}
		file, err := b.backend.Get(context.Background(), v.Name)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}

		obj, err := b.readUnpackedEncodedObject(file.Body, plumbing.NewHash(s[2]+s[3]))
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		encodedObjs = append(encodedObjs, obj)
	}
	return storer.NewEncodedObjectSliceIter(encodedObjs), nil
}

func (b *ObjectStorageStorer) HasEncodedObject(hash plumbing.Hash) error {
	_, err := b.getUnpackedEncodedObject(hash)
	return err
}

func (b *ObjectStorageStorer) EncodedObjectSize(hash plumbing.Hash) (int64, error) {
	obj, err := b.getUnpackedEncodedObject(hash)
	if err != nil {
		return -1, xerrors.WithStack(err)
	}
	return obj.Size(), nil
}

func (b *ObjectStorageStorer) AddAlternate(remote string) error {
	return nil
}

type EncodedObject struct {
	hash plumbing.Hash
	typ  plumbing.ObjectType
	size int64
	r    io.ReadCloser
	w    io.WriteCloser
}

var _ plumbing.EncodedObject = &EncodedObject{}

func (e *EncodedObject) Hash() plumbing.Hash {
	return e.hash
}

func (e *EncodedObject) Type() plumbing.ObjectType {
	return e.typ
}

func (e *EncodedObject) SetType(objectType plumbing.ObjectType) {
	e.typ = objectType
}

func (e *EncodedObject) Size() int64 {
	return e.size
}

func (e *EncodedObject) SetSize(i int64) {
	e.size = i
}

func (e *EncodedObject) Reader() (io.ReadCloser, error) {
	if e.r == nil {
		return nil, xerrors.New("this object is not readable")
	}
	return e.r, nil
}

func (e *EncodedObject) Writer() (io.WriteCloser, error) {
	if e.w == nil {
		return nil, xerrors.New("this object is not writable")
	}
	return e.w, nil
}

func (e *EncodedObject) SetReader(r io.ReadCloser) {
	e.r = r
}

func (e *EncodedObject) SetWriter(w *objfile.Writer) {
	e.w = w
}

func (e *EncodedObject) MarshalJSON() ([]byte, error) {
	var buf []byte
	if e.r != nil {
		b, err := io.ReadAll(e.r)
		if err != nil {
			return nil, err
		}
		e.r = io.NopCloser(bytes.NewReader(b))
		buf = b
	}

	j := new(bytes.Buffer)
	j.WriteRune('{')
	j.WriteString(`"type":`)
	fmt.Fprintf(j, "%q", e.typ.String())
	j.WriteRune(',')
	j.WriteString(`"hash":`)
	fmt.Fprintf(j, "%q", e.hash.String())
	if buf != nil {
		j.WriteRune(',')
		j.WriteString(`"content":`)
		fmt.Fprintf(j, "%q", base64.StdEncoding.EncodeToString(buf))
	}
	j.WriteRune('}')

	return j.Bytes(), nil
}

func (e *EncodedObject) UnmarshalJSON(b []byte) error {
	m := make(map[string]string)
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	e.hash = plumbing.NewHash(m["hash"])
	if v, err := plumbing.ParseObjectType(m["type"]); err != nil {
		return err
	} else {
		e.typ = v
	}
	if encoded, ok := m["content"]; ok {
		buf, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return err
		}
		e.r = io.NopCloser(bytes.NewReader(buf))
		e.size = int64(len(buf))
	}
	return nil
}

type bufferedFile struct {
	*bytes.Reader
	stub *bytes.Buffer
	name string
}

var _ billy.File = (*bufferedFile)(nil)

func newBufferedFile(name string, buf []byte) *bufferedFile {
	return &bufferedFile{name: name, Reader: bytes.NewReader(buf), stub: bytes.NewBuffer(buf)}
}

func (f *bufferedFile) Name() string {
	return f.name
}

func (f *bufferedFile) Close() error {
	return nil
}

func (f *bufferedFile) Lock() error {
	return nil
}

func (f *bufferedFile) Unlock() error {
	return nil
}

func (f *bufferedFile) Write(p []byte) (int, error) {
	return f.stub.Write(p)
}

func (f *bufferedFile) Truncate(size int64) error {
	f.stub.Truncate(int(size))
	return nil
}
