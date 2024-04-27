package git

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"io"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/idxfile"
	gitBinary "github.com/go-git/go-git/v5/utils/binary"
	"go.f110.dev/xerrors"
)

// Packfile is a decoder for git's packfile.
// Ref: https://git-scm.com/docs/pack-format
// Ref: http://shafiul.github.io/gitbook/7_the_packfile.html
type Packfile struct {
	index           idxfile.Index
	offsets         []uint64
	file            io.ReadSeekCloser
	fileLen         int64
	version         uint32
	numberOfObjects uint32
}

func NewPackfile(idx idxfile.Index, f io.ReadSeekCloser) (*Packfile, error) {
	buf := make([]byte, 4)
	if _, err := io.ReadFull(f, buf); err != nil {
		return nil, xerrors.WithStack(err)
	}
	if buf[0] != 'P' || buf[1] != 'A' || buf[2] != 'C' || buf[3] != 'K' {
		return nil, xerrors.Define("invalid packfile format. The signature is mismatch").WithStack()
	}

	if _, err := io.ReadFull(f, buf); err != nil {
		return nil, xerrors.WithStack(err)
	}
	var version uint32
	if v := binary.BigEndian.Uint32(buf); v != 2 {
		return nil, xerrors.Define("invalid packfile format. The version is not 2").WithStack()
	} else {
		version = v
	}

	var numOfObj uint32
	if _, err := io.ReadFull(f, buf); err != nil {
		return nil, xerrors.WithStack(err)
	}
	numOfObj = binary.BigEndian.Uint32(buf)
	if n, err := idx.Count(); err != nil {
		return nil, xerrors.WithStack(err)
	} else if n != int64(numOfObj) {
		return nil, xerrors.Define("invalid packfile format. mismatch the number of object count with the index").WithStack()
	}

	iter, err := idx.EntriesByOffset()
	if err != nil {
		return nil, err
	}
	count, err := idx.Count()
	if err != nil {
		return nil, err
	}
	offsets := make([]uint64, 0, count)
	for {
		e, err := iter.Next()
		if err == io.EOF {
			break
		}
		offsets = append(offsets, e.Offset)
	}
	fileLen, _ := f.Seek(0, io.SeekEnd)
	return &Packfile{index: idx, offsets: offsets, file: f, fileLen: fileLen, version: version, numberOfObjects: numOfObj}, nil
}

func (p *Packfile) Get(hash plumbing.Hash) (plumbing.EncodedObject, error) {
	offset, err := p.index.FindOffset(hash)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return p.readObject(offset, hash)
}

func (p *Packfile) All() ([]plumbing.EncodedObject, error) {
	iter, err := p.index.Entries()
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	var objs []plumbing.EncodedObject
	for {
		e, err := iter.Next()
		if err == io.EOF {
			break
		}
		obj, err := p.readObject(int64(e.Offset), e.Hash)
		if err != nil {
			return nil, err
		}
		objs = append(objs, obj)
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}

	return objs, nil
}

func (p *Packfile) readObject(offset int64, hash plumbing.Hash) (plumbing.EncodedObject, error) {
	if _, err := p.file.Seek(offset, io.SeekStart); err != nil {
		return nil, xerrors.WithStack(err)
	}
	buf := make([]byte, 1)
	if _, err := io.ReadFull(p.file, buf); err != nil {
		return nil, xerrors.WithStack(err)
	}

	var objectType = buf[0] >> 4 & 0x7
	var length = int64(buf[0] & 0x0F)
	sizeByte := buf[0]
	shift := 4
	for sizeByte>>7 == 1 {
		if _, err := io.ReadFull(p.file, buf); err != nil {
			return nil, xerrors.WithStack(err)
		}

		sizeByte = buf[0]
		length += int64(int64(sizeByte) & 0x7F << shift)
		shift += 7
	}

	var offsetReference int64
	switch plumbing.ObjectType(objectType) {
	case plumbing.OFSDeltaObject:
		no, err := gitBinary.ReadVariableWidthInt(p.file)
		if err != nil {
			return nil, err
		}

		offsetReference = offset - no
	}

	var bufSize int64
	for i, v := range p.offsets {
		if int64(v) == offset {
			if len(p.offsets) == i+1 {
				bufSize = p.fileLen - offset
			} else {
				bufSize = int64(p.offsets[i+1]) - offset
			}
			break
		}
	}
	buf = make([]byte, bufSize)
	if _, err := p.file.Read(buf); err != nil {
		return nil, xerrors.WithStack(err)
	}

	obj := &EncodedObject{hash: hash, typ: plumbing.ObjectType(objectType), size: length}
	switch obj.typ {
	case plumbing.CommitObject, plumbing.TreeObject, plumbing.BlobObject, plumbing.TagObject:
		r, err := zlib.NewReader(bytes.NewReader(buf))
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		obj.SetReader(r)
	case plumbing.OFSDeltaObject:
		r, err := zlib.NewReader(bytes.NewReader(buf))
		if err != nil {
			return nil, xerrors.WithStack(err)
		}

		_, err = readInt64LittleEndian(r) // src size
		if err != nil {
			return nil, err
		}
		size, err := readInt64LittleEndian(r) // actual size
		if err != nil {
			return nil, err
		}
		obj.SetSize(size)
		actualHash, err := p.index.FindHash(offsetReference)
		if err != nil {
			return nil, err
		}
		deltaObj, err := p.readObject(offsetReference, actualHash)
		if err != nil {
			return nil, err
		}

		obj.SetType(deltaObj.Type())
		rc, err := deltaObj.Reader()
		if err != nil {
			return nil, err
		}
		obj.SetReader(rc)
	default:
		obj.SetReader(io.NopCloser(bytes.NewReader(buf)))
	}
	return obj, nil
}

func readInt64LittleEndian(r io.Reader) (int64, error) {
	c := make([]byte, 1)
	if _, err := r.Read(c); err != nil {
		return 0, xerrors.WithStack(err)
	}

	v := int64(c[0] & 0x7F)
	shift := 7
	for c[0]>>7 == 1 {
		if _, err := r.Read(c); err != nil {
			return 0, xerrors.WithStack(err)
		}

		v += int64(int64(c[0]) & 0x7F << shift)
		shift += 7
	}

	return v, nil
}
