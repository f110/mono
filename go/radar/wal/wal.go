// Package wal provides the local intermediate storage of the collector.
//
// The collector appends the parsed values to the WAL and the archiver reads the whole WAL of the
// day to build the daily container. The WAL is neither compressed nor differential so that the
// collecting path stays as simple as possible and has fewer points of failure.
package wal

import (
	"encoding/binary"
	"hash/crc32"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/radar"
)

const (
	fileExt = ".wal"
	// headerSize is the size of the slot and the length field of a record.
	recordHeaderSize = 8
	// checksumSize is the size of the CRC32 that follows the values.
	checksumSize = 4
)

var ErrInvalidPath = xerrors.Define("wal: invalid path")

// WAL is the local intermediate storage rooted at a directory.
type WAL struct {
	dir      string
	interval time.Duration
}

// New returns a WAL that stores the frames of the interval under dir.
func New(dir string, interval time.Duration) *WAL {
	return &WAL{dir: dir, interval: interval}
}

// Entry identifies a WAL file.
type Entry struct {
	Source radar.SourceID
	Tile   radar.Tile
	Date   time.Time
}

// Path returns the path of the WAL file of the frame.
func (w *WAL) Path(source radar.SourceID, tile radar.Tile, date time.Time) string {
	d := radar.DateOf(date)
	return filepath.Join(
		w.dir,
		source.String(),
		strconv.Itoa(tile.Z),
		strconv.Itoa(tile.X),
		strconv.Itoa(tile.Y),
		d.Format("20060102")+fileExt,
	)
}

// Append writes the values of the frame to the tail of the WAL file and calls fsync.
//
// A record of the same slot may appear more than once. The last one wins when it is loaded so
// that a retry does not have to care about the duplication.
func (w *WAL) Append(f *radar.Frame) error {
	slot := radar.Slot(f.ValidTime, w.interval)
	if slot < 0 || slot >= radar.FrameCount(w.interval) {
		return xerrors.NewfWithStack("wal: slot is out of range: %d", slot)
	}

	p := w.Path(f.Source, f.Tile, f.ValidTime)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return xerrors.WithStack(err)
	}
	file, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer file.Close()

	buf := make([]byte, recordHeaderSize+len(f.Values)+checksumSize)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(slot))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(f.Values)))
	copy(buf[recordHeaderSize:], f.Values)
	binary.LittleEndian.PutUint32(buf[recordHeaderSize+len(f.Values):], crc32.ChecksumIEEE(f.Values))

	if _, err := file.Write(buf); err != nil {
		return xerrors.WithStack(err)
	}
	if err := file.Sync(); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

// Load reads the WAL file and returns the values of each slot.
//
// A record whose checksum does not match and everything after it are discarded. A crash can leave
// a partial record at the tail and it is not distinguishable from the corruption, so the reading
// stops at the first broken record instead of trying to recover the rest.
func (w *WAL) Load(source radar.SourceID, tile radar.Tile, date time.Time) (map[int][]byte, error) {
	buf, err := os.ReadFile(w.Path(source, tile, date))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, xerrors.WithStack(err)
	}

	frames := make(map[int][]byte)
	var offset int
	for offset < len(buf) {
		if offset+recordHeaderSize > len(buf) {
			w.logTruncated(source, tile, offset)
			break
		}
		slot := int(binary.LittleEndian.Uint32(buf[offset : offset+4]))
		length := binary.LittleEndian.Uint32(buf[offset+4 : offset+8])
		// The arithmetic is done in uint64 so that a bogus length cannot overflow into a value that
		// passes the bound check.
		if uint64(offset)+recordHeaderSize+uint64(length)+checksumSize > uint64(len(buf)) {
			w.logTruncated(source, tile, offset)
			break
		}
		end := offset + recordHeaderSize + int(length) + checksumSize
		values := buf[offset+recordHeaderSize : offset+recordHeaderSize+int(length)]
		sum := binary.LittleEndian.Uint32(buf[end-checksumSize : end])
		if crc32.ChecksumIEEE(values) != sum {
			w.logTruncated(source, tile, offset)
			break
		}
		// The last record of the slot wins.
		frames[slot] = values
		offset = end
	}
	return frames, nil
}

func (w *WAL) logTruncated(source radar.SourceID, tile radar.Tile, offset int) {
	if !slogger.Enabled() {
		return
	}
	slogger.Log.Warn("wal: stopped reading at a broken record",
		slog.String("source", source.String()), slog.String("tile", tile.String()), slog.Int("offset", offset))
}

// Remove deletes the WAL file. It is called after the daily container is verified.
func (w *WAL) Remove(source radar.SourceID, tile radar.Tile, date time.Time) error {
	if err := os.Remove(w.Path(source, tile, date)); err != nil && !os.IsNotExist(err) {
		return xerrors.WithStack(err)
	}
	return nil
}

// List enumerates the WAL files of the date.
func (w *WAL) List(date time.Time) ([]Entry, error) {
	name := radar.DateOf(date).Format("20060102") + fileExt

	var entries []Entry
	err := filepath.WalkDir(w.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || d.Name() != name {
			return nil
		}
		e, err := w.parsePath(p)
		if err != nil {
			if slogger.Enabled() {
				slogger.Log.Warn("wal: skipping a file of an unexpected path", slog.String("path", p), slogger.E(err))
			}
			return nil
		}
		entries = append(entries, e)
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, xerrors.WithStack(err)
	}
	return entries, nil
}

// parsePath extracts the source and the tile from the path of a WAL file.
func (w *WAL) parsePath(p string) (Entry, error) {
	rel, err := filepath.Rel(w.dir, p)
	if err != nil {
		return Entry{}, xerrors.WithStack(err)
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) != 5 {
		return Entry{}, xerrors.WithMessagef(ErrInvalidPath, "%s", rel)
	}
	source, err := radar.ParseSourceID(parts[0])
	if err != nil {
		return Entry{}, err
	}
	tile, err := radar.ParseTile(parts[1] + "/" + parts[2] + "/" + parts[3])
	if err != nil {
		return Entry{}, err
	}
	date, err := time.ParseInLocation("20060102", parts[4][:len(parts[4])-len(fileExt)], radar.JST)
	if err != nil {
		return Entry{}, xerrors.WithStack(err)
	}
	return Entry{Source: source, Tile: tile, Date: date}, nil
}
