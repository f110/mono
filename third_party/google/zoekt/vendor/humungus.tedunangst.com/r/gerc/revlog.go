/*
 * Copyright (c) 2019 Ted Unangst <tedu@tedunangst.com>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package gerc

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
)

// One revision.
// This is mostly an internal data structure.
type Rev struct {
	Offset    uint64
	Flags     uint16
	Clen      uint32
	Ulen      uint32
	Baserev   int // uint32
	Linkrev   int // uint32
	P1rev     int // uint32
	P2rev     int // uint32
	NodeID    [32]byte
	data      []byte
	datavalid bool
}

// Get the data for this chunk. It may be a delta.
func (rev *Rev) ChunkData(repo *Repo, datafile string) ([]byte, error) {
	if rev.datavalid {
		return rev.data, nil
	}
	if rev.data == nil {
		if rev.Clen == 0 {
			return nil, nil
		}
		fd, err := repo.fds.open(datafile)
		if err != nil {
			return nil, err
		}
		defer repo.fds.put(datafile, fd)
		fd.Seek(int64(rev.Offset), 0)
		rev.data = make([]byte, rev.Clen)
		_, err = io.ReadFull(fd, rev.data)
		if err != nil {
			return nil, err
		}
		switch rev.data[0] {
		case 0:
			rev.datavalid = true
		case 'u':
			rev.data = rev.data[1:]
			rev.datavalid = true
		}
		if rev.datavalid {
			return rev.data, nil
		}
	}
	switch rev.data[0] {
	case 'x':
		var buf bytes.Buffer
		gzr, err := zlib.NewReader(bytes.NewReader(rev.data))
		if err != nil {
			return nil, err
		}
		defer gzr.Close()
		io.Copy(&buf, gzr)
		rev.data = buf.Bytes()
		rev.datavalid = true
		return rev.data, nil
	}
	return nil, fmt.Errorf("unknown compression")
}

// A revlog structure
type Revlog struct {
	version  uint16
	flags    uint16
	datafile string
	revs     []*Rev
	links    map[int]int
	repo     *Repo
}

const flagInlineDeltas = 0x1
const flagSkipDeltas = 0x2

// Returns the length.
func (revlog *Revlog) Len() int {
	return len(revlog.revs)
}

// A revision could not be found
type RevisionNotFoundError struct{}

func (e RevisionNotFoundError) Error() string {
	return "revision not found"
}

// Get one revision from the revlog, by index.
func (revlog *Revlog) Get(i int) (*Rev, error) {
	if i < 0 || i >= len(revlog.revs) {
		return nil, RevisionNotFoundError{}
	}
	return revlog.revs[i], nil
}

// Return the index in this revlog for a link.
func (revlog *Revlog) FindLink(linkrev int) int {
	idx, ok := revlog.links[linkrev]
	if !ok {
		return -1
	}
	return idx
}

// Get the data for an index.
// This will apply deltas as necessary to reconstruct the data.
// Returns data, metadata, error.
func (revlog *Revlog) GetData(idx int) ([]byte, []byte, error) {
	baserev, err := revlog.Get(idx)
	if err != nil {
		return nil, nil, err
	}
	var prevrevs []*Rev
	for idx != baserev.Baserev {
		prevrevs = append(prevrevs, baserev)
		if revlog.flags&flagSkipDeltas != 0 {
			idx = baserev.Baserev
		} else {
			idx--
		}
		baserev, err = revlog.Get(idx)
		if err != nil {
			return nil, nil, err
		}
		if baserev.Baserev > idx {
			return nil, nil, fmt.Errorf("revlog goes backwards")
		}
	}

	base, err := baserev.ChunkData(revlog.repo, revlog.datafile)
	if err != nil {
		return nil, nil, err
	}
	var scratch []byte
	for i := len(prevrevs) - 1; i >= 0; i-- {
		if scratch == nil {
			// create another copy to not modify base
			scratch = make([]byte, len(base))
			copy(scratch, base)
			base = make([]byte, len(base))
			base, scratch = scratch, base
		}
		rev := prevrevs[i]
		deltadata, err := rev.ChunkData(revlog.repo, revlog.datafile)
		deltas, err := parsedeltas(deltadata, err)
		oldbase := base
		base, err = applydeltas(scratch, base, deltas, err)
		if err != nil {
			return nil, nil, err
		}
		scratch = oldbase
	}
	data, meta := splitmeta(base)
	return data, meta, nil
}

func splitmeta(data []byte) ([]byte, []byte) {
	metamarker := []byte{'\x01', '\n'}
	if bytes.HasPrefix(data, metamarker) {
		idx := bytes.Index(data[2:], metamarker)
		if idx != -1 {
			return data[2+idx+2:], data[2:idx]
		}
	}
	return data, nil
}

func revnumfrombytes(b []byte) int {
	BE := binary.BigEndian
	x := BE.Uint32(b)
	if x == 4294967295 {
		return -1
	}
	return int(x)
}

func (repo *Repo) readrevlog(filename string) (*Revlog, error) {
	revlog := repo.cache[filename]
	if revlog != nil {
		// notyet clear the cache at some point
		return revlog, nil
	}
	filefd, err := repo.hgopen(filename)
	if err != nil {
		return nil, err
	}
	defer filefd.Close()
	fd := bufio.NewReader(filefd)
	BE := binary.BigEndian
	first := true
	revlog = new(Revlog)
	revlog.links = make(map[int]int)
	revlog.repo = repo
	for {
		rev := &Rev{}
		var offset [8]byte
		_, err := io.ReadFull(fd, offset[2:8])
		if err == io.EOF {
			repo.cache[filename] = revlog
			return revlog, nil
		}
		if err != nil {
			return nil, err
		}
		if first {
			revlog.flags = BE.Uint16(offset[2:4])
			revlog.version = BE.Uint16(offset[4:6])
			if revlog.flags&flagInlineDeltas == 0 {
				revlog.datafile = repo.hgfilename(filename[:len(filename)-1] + "d")
			}
			rev.Offset = 0
		} else {
			rev.Offset = BE.Uint64(offset[:])
		}
		var buf [58]byte
		_, err = io.ReadFull(fd, buf[:])
		if err != nil {
			return nil, err
		}

		rev.Flags = BE.Uint16(buf[0:2])
		rev.Clen = BE.Uint32(buf[2:6])
		rev.Ulen = BE.Uint32(buf[6:10])
		rev.Baserev = revnumfrombytes(buf[10:14])
		rev.Linkrev = revnumfrombytes(buf[14:18])
		rev.P1rev = revnumfrombytes(buf[18:22])
		rev.P2rev = revnumfrombytes(buf[22:26])
		copy(rev.NodeID[:], buf[26:58])

		if revlog.flags&flagInlineDeltas != 0 && rev.Clen > 0 {
			rev.data = make([]byte, rev.Clen)
			_, err = io.ReadFull(fd, rev.data)
			if err != nil {
				return nil, err
			}
			switch rev.data[0] {
			case 0:
				rev.datavalid = true
			case 'u':
				rev.data = rev.data[1:]
				rev.datavalid = true
			}
		}
		revlog.links[rev.Linkrev] = len(revlog.revs)
		revlog.revs = append(revlog.revs, rev)
		first = false
	}
}

// A delta. Internal.
type Delta struct {
	Start uint32
	Skip  uint32
	Add   uint32
	Plus  []byte
}

func parsedeltas(data []byte, err error) ([]*Delta, error) {
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	if data[0] != 0 {
		return nil, fmt.Errorf("not a diff")
	}
	BE := binary.BigEndian
	var deltas []*Delta
	for pos := 0; pos < len(data); {
		if pos+12 > len(data) {
			return nil, fmt.Errorf("corrupted delta")
		}
		delta := new(Delta)
		delta.Start = BE.Uint32(data[pos : pos+4])
		delta.Skip = BE.Uint32(data[pos+4 : pos+8])
		delta.Add = BE.Uint32(data[pos+8 : pos+12])
		pos += 12
		if delta.Add > 0 {
			amt := int(delta.Add)
			if pos+amt > len(data) {
				return nil, fmt.Errorf("corrupted delta")
			}
			delta.Plus = data[pos : pos+amt]
			pos += amt
		}
		deltas = append(deltas, delta)
	}
	return deltas, nil
}

func applydeltas(res []byte, base []byte, diff []*Delta, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	var n uint32 = 0
	var pos uint32 = 0
	for _, delta := range diff {
		amt := delta.Start - pos
		if amt > 0 {
			n += amt
		}
		pos = delta.Skip
		if delta.Add > 0 {
			n += delta.Add
		}
	}
	if int(pos) < len(base) {
		n += uint32(len(base)) - pos
	}

	if len(res) < int(n) {
		res = append(res, make([]byte, int(n)-len(res))...)
	}

	n = 0
	pos = 0
	for _, delta := range diff {
		amt := delta.Start - pos
		if amt > 0 {
			copy(res[n:], base[pos:delta.Start])
			n += amt
		}
		pos = delta.Skip
		if delta.Add > 0 {
			copy(res[n:], delta.Plus)
			n += delta.Add
		}
	}
	if int(pos) < len(base) {
		copy(res[n:], base[pos:])
		n += uint32(len(base)) - pos
	}
	return res[:n], nil
}
