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
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/idxfile"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/format/objfile"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gitStorage "github.com/go-git/go-git/v5/storage"
	"go.f110.dev/go-memcached/client"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/collections/set"
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

	s := NewObjectStorageStorer(b, prefix, nil)
	repo, err := git.Open(s, nil)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to open the repository")
	}
	if err := s.InflatePackFile(ctx); err != nil {
		return nil, xerrors.WithMessage(err, "failed to inflate pack file")
	}

	return repo, nil
}

type ObjectStorageStorer struct {
	backend   ObjectStorageInterface
	rootPath  string
	cachePool *client.SinglePool
}

var _ gitStorage.Storer = &ObjectStorageStorer{}

func NewObjectStorageStorer(b ObjectStorageInterface, rootPath string, cachePool *client.SinglePool) *ObjectStorageStorer {
	return &ObjectStorageStorer{backend: b, rootPath: rootPath, cachePool: cachePool}
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
	return NewObjectStorageStorer(b.backend, path.Join(b.rootPath, name), b.cachePool), nil
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

func (b *ObjectStorageStorer) InflatePackFile(ctx context.Context) error {
	packFiles, err := b.backend.List(ctx, path.Join(b.rootPath, "objects/pack"))
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
		file, err := b.backend.Get(ctx, filepath.Join(b.rootPath, fmt.Sprintf("objects/pack/pack-%s.idx", v.String())))
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

		file, err = b.backend.Get(ctx, filepath.Join(b.rootPath, fmt.Sprintf("objects/pack/pack-%s.pack", v.String())))
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
		packfile, err := NewPackfile(idx, nopCloser(bytes.NewReader(buf)))
		if err != nil {
			return err
		}

		objs, err := packfile.All()
		if err != nil {
			return err
		}

		for _, obj := range objs {
			if _, err := b.SetEncodedObject(obj); err != nil {
				return err
			}
		}

		if err := b.backend.Delete(ctx, path.Join(b.rootPath, fmt.Sprintf("objects/pack/pack-%s.pack", v.String()))); err != nil {
			return err
		}
		if err := b.backend.Delete(ctx, path.Join(b.rootPath, fmt.Sprintf("objects/pack/pack-%s.pack", v.String()))); err != nil {
			return err
		}
	}

	return nil
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
			return xerrors.New("reference has changed concurrently")
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
	refSet := set.New()
	for _, v := range r {
		refSet.Add(v)
	}

	packedRefs, err := b.readPackedRefs()
	if err != nil {
		return xerrors.WithStack(err)
	}
	packedRefsSet := set.New()
	for _, v := range packedRefs {
		refSet.Add(v)
		packedRefsSet.Add(v)
	}

	buf := new(bytes.Buffer)
	for _, v := range refSet.ToSlice() {
		ref := v.(*plumbing.Reference)
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
	for _, v := range looseRefs.ToSlice() {
		ref := v.(*plumbing.Reference)
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

	var packIndex *idxfile.MemoryIndex
	var packHash plumbing.Hash
	// Find object in the index file
	for _, v := range packs {
		file, err := b.backend.Get(context.Background(), filepath.Join(b.rootPath, fmt.Sprintf("objects/pack/pack-%s.idx", v.String())))
		if err != nil {
			return nil, err
		}
		idx := idxfile.NewMemoryIndex()
		if err := idxfile.NewDecoder(file.Body).Decode(idx); err != nil {
			return nil, err
		}
		if err := file.Body.Close(); err != nil {
			return nil, err
		}
		if _, err := idx.FindOffset(h); err == nil {
			packIndex = idx
			packHash = v
			break
		}
	}
	if packIndex == nil {
		return nil, plumbing.ErrObjectNotFound
	}

	// Fetch the object from packfile
	file, err := b.backend.Get(context.Background(), filepath.Join(b.rootPath, fmt.Sprintf("objects/pack/pack-%s.pack", packHash.String())))
	if err != nil {
		return nil, err
	}
	buf, err := io.ReadAll(file.Body)
	if err != nil {
		return nil, err
	}
	if err := file.Body.Close(); err != nil {
		return nil, err
	}
	packfile, err := NewPackfile(packIndex, nopCloser(bytes.NewReader(buf)))
	if err != nil {
		return nil, err
	}
	obj, err := packfile.Get(h)
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

type nopSeekCloser struct {
	io.ReadSeeker
}

func nopCloser(r io.ReadSeeker) io.ReadSeekCloser {
	return nopSeekCloser{ReadSeeker: r}
}

func (nopSeekCloser) Close() error { return nil }

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

	if err := f.Close(); err != nil {
		return nil, xerrors.WithStack(err)
	}
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
