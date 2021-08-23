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
	"os"
	"sync"
)

type fdcacheent struct {
	filename string
	fd       *os.File
}

type fdcache struct {
	ents []fdcacheent
	pos  int
	mtx  sync.Mutex
}

var filecache = fdcache{
	ents: make([]fdcacheent, 16),
}

func (fdc *fdcache) open(filename string) (*os.File, error) {
	fdc.mtx.Lock()
	defer fdc.mtx.Unlock()
	for i := range fdc.ents {
		ent := &fdc.ents[i]
		if ent.filename == filename {
			fd := ent.fd
			ent.filename = ""
			ent.fd = nil
			return fd, nil
		}
	}
	return os.Open(filename)
}

func (fdc *fdcache) put(filename string, fd *os.File) {
	fdc.mtx.Lock()
	defer fdc.mtx.Unlock()
	for i := range fdc.ents {
		ent := &fdc.ents[i]
		if ent.filename == "" {
			ent.filename = filename
			ent.fd = fd
			return
		}
	}
	fdc.pos++
	if fdc.pos == len(fdc.ents) {
		fdc.pos = 0
	}
	ent := &fdc.ents[fdc.pos]
	ent.fd.Close()
	ent.filename = filename
	ent.fd = fd
}

func (fdc *fdcache) close() {
	fdc.mtx.Lock()
	defer fdc.mtx.Unlock()
	for i := range fdc.ents {
		ent := &fdc.ents[i]
		if ent.filename != "" {
			ent.filename = ""
			ent.fd.Close()
		}
	}
}
