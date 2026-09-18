package cmd

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/radar"
	"go.f110.dev/mono/go/radar/amesh"
	"go.f110.dev/mono/go/radar/dayfile"
	"go.f110.dev/mono/go/radar/wal"
	"go.f110.dev/mono/go/storage"
	"go.f110.dev/mono/go/testing/assertion"
)

type memoryStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	putErr  error
}

func newMemoryStore() *memoryStore {
	return &memoryStore{objects: make(map[string][]byte)}
}

func (s *memoryStore) Name() string { return "memory" }

func (s *memoryStore) PutReader(_ context.Context, name string, r io.Reader) error {
	if s.putErr != nil {
		return s.putErr
	}
	buf, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[name] = buf
	return nil
}

func (s *memoryStore) Get(_ context.Context, name string) (*storage.Object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	buf, ok := s.objects[name]
	if !ok {
		return nil, xerrors.WithMessagef(storage.ErrObjectNotFound, "%s", name)
	}
	return &storage.Object{
		Name: name,
		Size: int64(len(buf)),
		Body: io.NopCloser(bytes.NewReader(buf)),
	}, nil
}

func testDate() time.Time {
	return time.Date(2026, 7, 16, 0, 0, 0, 0, radar.JST)
}

func testTile() radar.Tile {
	return radar.Tile{Z: 10, X: 909, Y: 403}
}

func seedWAL(t *testing.T, slots map[int]byte) string {
	t.Helper()
	workDir := t.TempDir()
	w := wal.New(filepath.Join(workDir, "wal"), time.Minute)
	for slot, v := range slots {
		values := bytes.Repeat([]byte{v}, radar.TileSize*radar.TileSize)
		assertion.MustNoError(t, w.Append(&radar.Frame{
			Source:       radar.SourceAmesh,
			Tile:         testTile(),
			ValidTime:    radar.SlotTime(testDate(), slot, time.Minute),
			ScaleVersion: amesh.ScaleVersion1,
			Values:       values,
		}))
	}
	return workDir
}

func TestObjectKey(t *testing.T) {
	cases := []struct {
		name   string
		source radar.SourceID
		tile   radar.Tile
		at     time.Time
		want   string
	}{
		{
			name:   "jst",
			source: radar.SourceAmesh,
			tile:   radar.Tile{Z: 10, X: 909, Y: 403},
			at:     testDate(),
			want:   "radar/v1/amesh/10/909/403/2026/07/16.rac.zst",
		},
		{
			name:   "utc rolls over",
			source: radar.SourceXRAIN,
			tile:   radar.Tile{Z: 10, X: 905, Y: 402},
			at:     time.Date(2026, 7, 16, 16, 0, 0, 0, time.UTC),
			want:   "radar/v1/xrain/10/905/402/2026/07/17.rac.zst",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertion.Equal(t, objectKey(tc.source, tc.tile, tc.at), tc.want)
		})
	}
}

func TestArchive(t *testing.T) {
	workDir := seedWAL(t, map[int]byte{0: 1, 1: 1, 5: 2, 1439: 3})
	store := newMemoryStore()

	assertion.MustNoError(t, archive(context.Background(), workDir, testDate(), store))

	key := objectKey(radar.SourceAmesh, testTile(), testDate())
	obj, err := store.Get(context.Background(), key)
	assertion.MustNoError(t, err)

	r, err := dayfile.NewReader(obj.Body)
	assertion.MustNoError(t, err)
	h := r.Header()
	assertion.Equal(t, h.Source, radar.SourceAmesh)
	assertion.Equal(t, h.Tile, testTile())
	assertion.Equal(t, h.FrameCount, 1440)
	assertion.Equal(t, h.Interval, time.Minute)
	assertion.True(t, h.Finalized)

	kinds := r.Kinds()
	for _, tc := range []struct {
		name string
		slot int
		kind dayfile.Kind
	}{
		{name: "keyframe", slot: 0, kind: dayfile.KindKeyframe},
		{name: "unchanged", slot: 1, kind: dayfile.KindUnchanged},
		{name: "missing", slot: 2, kind: dayfile.KindMissing},
		{name: "delta", slot: 5, kind: dayfile.KindDelta},
		{name: "last", slot: 1439, kind: dayfile.KindDelta},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertion.Equal(t, kinds[tc.slot], tc.kind)
		})
	}

	got, err := r.Frame(1439)
	assertion.MustNoError(t, err)
	assertion.Equal(t, got, bytes.Repeat([]byte{3}, radar.TileSize*radar.TileSize))

	entries, err := wal.New(filepath.Join(workDir, "wal"), time.Minute).List(testDate())
	assertion.MustNoError(t, err)
	assertion.Empty(t, entries)
}

func TestArchiveIsIdempotent(t *testing.T) {
	workDir := seedWAL(t, map[int]byte{0: 1, 10: 2})
	store := newMemoryStore()

	assertion.MustNoError(t, archive(context.Background(), workDir, testDate(), store))
	first := store.objects[objectKey(radar.SourceAmesh, testTile(), testDate())]

	assertion.MustNoError(t, archive(context.Background(), workDir, testDate(), store))
	assertion.Equal(t, store.objects[objectKey(radar.SourceAmesh, testTile(), testDate())], first)
}

func TestArchiveKeepsTheWALWhenTheUploadFails(t *testing.T) {
	workDir := seedWAL(t, map[int]byte{0: 1})
	store := newMemoryStore()
	store.putErr = xerrors.NewWithStack("upload failed")

	assertion.Error(t, archive(context.Background(), workDir, testDate(), store))

	entries, err := wal.New(filepath.Join(workDir, "wal"), time.Minute).List(testDate())
	assertion.MustNoError(t, err)
	assertion.Len(t, entries, 1)
}

func TestArchiveWithoutWAL(t *testing.T) {
	assertion.MustNoError(t, archive(context.Background(), t.TempDir(), testDate(), newMemoryStore()))
}

func TestParseDate(t *testing.T) {
	t.Run("explicit", func(t *testing.T) {
		d, err := parseDate("2026-07-16")
		assertion.MustNoError(t, err)
		assertion.Equal(t, d.Format(time.RFC3339), "2026-07-16T00:00:00+09:00")
	})

	t.Run("empty means the previous day", func(t *testing.T) {
		d, err := parseDate("")
		assertion.MustNoError(t, err)
		assertion.Equal(t, d, radar.DateOf(time.Now()).AddDate(0, 0, -1))
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := parseDate("not a date")
		assertion.Error(t, err)
	})
}

func TestIntervalOf(t *testing.T) {
	cases := []struct {
		name   string
		source radar.SourceID
		want   time.Duration
	}{
		{name: "amesh", source: radar.SourceAmesh, want: time.Minute},
		{name: "xrain", source: radar.SourceXRAIN, want: time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertion.Equal(t, intervalOf(tc.source), tc.want)
		})
	}
}
