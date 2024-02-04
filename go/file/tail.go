package file

import (
	"bufio"
	"errors"
	"io"
	"os"

	"github.com/fsnotify/fsnotify"
	"go.f110.dev/xerrors"
)

type TailReader struct {
	r  *bufio.Reader
	w  *fsnotify.Watcher
	ch chan struct{}
}

var _ io.Reader = (*TailReader)(nil)
var _ io.ByteReader = (*TailReader)(nil)
var _ io.Closer = (*TailReader)(nil)

func NewTailReader(f *os.File) (*TailReader, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	if err := w.Add(f.Name()); err != nil {
		return nil, xerrors.WithStack(err)
	}

	r := &TailReader{r: bufio.NewReader(f), w: w, ch: make(chan struct{})}
	go r.watch()
	return r, nil
}

func (t *TailReader) watch() {
	for e := range t.w.Events {
		if e.Op&fsnotify.Write == fsnotify.Write {
			select {
			case t.ch <- struct{}{}:
			default:
			}
		}
	}
}

func (t *TailReader) Read(b []byte) (int, error) {
	n, err := t.r.Read(b)
	if errors.Is(err, io.EOF) {
		if n > 0 {
			return n, nil
		}

		for {
			_, ok := <-t.ch
			if !ok {
				return 0, io.EOF
			}
			n, err = t.r.Read(b)
			if errors.Is(err, io.EOF) {
				if n > 0 {
					return n, nil
				}
				continue
			}
			if err != nil {
				return n, err
			}
			return n, nil
		}
	}
	if err != nil {
		return 0, xerrors.WithStack(err)
	}

	return n, nil
}

func (t *TailReader) ReadByte() (byte, error) {
	b, err := t.r.ReadByte()
	if errors.Is(err, io.EOF) {
		for {
			_, ok := <-t.ch
			if !ok {
				return 0, io.EOF
			}
			b, err = t.r.ReadByte()
			if errors.Is(err, io.EOF) {
				continue
			}
			if err != nil {
				return 0, xerrors.WithStack(err)
			}
			return b, nil
		}
	}
	if err != nil {
		return 0, xerrors.WithStack(err)
	}
	return b, nil
}

func (t *TailReader) Close() error {
	if err := t.w.Close(); err != nil {
		return xerrors.WithStack(err)
	}
	close(t.ch)
	return nil
}
