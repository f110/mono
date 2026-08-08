package sidecar

import (
	"errors"
	"io"
	"os"
	"time"

	"go.f110.dev/xerrors"
	"google.golang.org/protobuf/encoding/protodelim"

	"go.f110.dev/mono/go/bazel/buildeventstream"
	"go.f110.dev/mono/go/file"
)

// waitAndOpenBEP waits for the Build Event Protocol file at path to appear (up to
// timeout), then opens it wrapped in a tailing reader. The caller must close the
// returned file. Both the report and collect sidecars read the same BEP file
// this way.
func waitAndOpenBEP(path string, timeout time.Duration) (*os.File, protodelim.Reader, error) {
	ch := make(chan struct{})
	go func() {
		for {
			if _, err := os.Lstat(path); err == nil {
				close(ch)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
	select {
	case <-ch:
	case <-time.After(timeout):
		return nil, nil, xerrors.Definef("could not find event binary file in %v", timeout).WithStack()
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, nil, xerrors.WithStack(err)
	}
	r, err := file.NewTailReader(f)
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return f, r, nil
}

// iterateBuildEvents decodes the length-delimited BuildEvent stream and calls fn
// for each event until the stream ends. Termination keys on the last_message flag
// because a tailing reader does not return io.EOF on its own while the writer is
// open; io.EOF is still handled for non-tailing readers (e.g. tests).
func iterateBuildEvents(r protodelim.Reader, fn func(*buildeventstream.BuildEvent) error) error {
	for {
		msg := &buildeventstream.BuildEvent{}
		if err := protodelim.UnmarshalFrom(r, msg); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return xerrors.WithStack(err)
		}
		if err := fn(msg); err != nil {
			return err
		}
		if msg.GetLastMessage() {
			return nil
		}
	}
}
