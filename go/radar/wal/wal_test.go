package wal

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.f110.dev/mono/go/radar"
	"go.f110.dev/mono/go/testing/assertion"
)

var (
	testDate = time.Date(2026, 7, 16, 0, 0, 0, 0, radar.JST)
	testTile = radar.Tile{Z: 10, X: 909, Y: 403}
)

func testFrame(slotTime time.Time, v byte) *radar.Frame {
	values := make([]byte, 16)
	for i := range values {
		values[i] = v
	}
	return &radar.Frame{
		Source:       radar.SourceAmesh,
		Tile:         testTile,
		ValidTime:    slotTime,
		ScaleVersion: 1,
		Values:       values,
	}
}

func TestAppendAndLoad(t *testing.T) {
	t.Run("keeps every slot", func(t *testing.T) {
		w := New(t.TempDir(), time.Minute)
		f0 := testFrame(testDate, 1)
		f5 := testFrame(testDate.Add(5*time.Minute), 2)
		assertion.MustNoError(t, w.Append(f0))
		assertion.MustNoError(t, w.Append(f5))

		frames, err := w.Load(f0.Source, f0.Tile, testDate)
		assertion.MustNoError(t, err)
		assertion.Len(t, frames, 2)
		assertion.Equal(t, frames[0], f0.Values)
		assertion.Equal(t, frames[5], f5.Values)
	})

	t.Run("the last record of the slot wins", func(t *testing.T) {
		w := New(t.TempDir(), time.Minute)
		assertion.MustNoError(t, w.Append(testFrame(testDate, 1)))
		assertion.MustNoError(t, w.Append(testFrame(testDate, 9)))

		frames, err := w.Load(radar.SourceAmesh, testTile, testDate)
		assertion.MustNoError(t, err)
		assertion.Len(t, frames, 1)
		assertion.Equal(t, frames[0], testFrame(testDate, 9).Values)
	})

	t.Run("the real size", func(t *testing.T) {
		w := New(t.TempDir(), time.Minute)
		values := bytes.Repeat([]byte{7}, radar.TileSize*radar.TileSize)
		f := &radar.Frame{
			Source:    radar.SourceXRAIN,
			Tile:      radar.Tile{Z: 10, X: 905, Y: 402},
			ValidTime: testDate.Add(123 * time.Minute),
			Values:    values,
		}
		assertion.MustNoError(t, w.Append(f))

		frames, err := w.Load(f.Source, f.Tile, testDate)
		assertion.MustNoError(t, err)
		assertion.Equal(t, frames[123], values)
	})

	t.Run("a missing file is not an error", func(t *testing.T) {
		w := New(t.TempDir(), time.Minute)
		frames, err := w.Load(radar.SourceAmesh, testTile, testDate)
		assertion.MustNoError(t, err)
		assertion.Empty(t, frames)
	})

	t.Run("an out of range slot is rejected", func(t *testing.T) {
		w := New(t.TempDir(), 0)
		assertion.Error(t, w.Append(testFrame(testDate, 1)))
	})
}

func TestLoadStopsAtABrokenRecord(t *testing.T) {
	cases := []struct {
		name   string
		break_ func(buf []byte) []byte
	}{
		{
			name:   "truncated",
			break_: func(buf []byte) []byte { return buf[:len(buf)-5] },
		},
		{
			name: "checksum does not match",
			break_: func(buf []byte) []byte {
				recordSize := recordHeaderSize + 16 + checksumSize
				buf[recordSize+recordHeaderSize] ^= 0xff
				return buf
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := New(t.TempDir(), time.Minute)
			assertion.MustNoError(t, w.Append(testFrame(testDate, 1)))
			assertion.MustNoError(t, w.Append(testFrame(testDate.Add(time.Minute), 2)))

			p := w.Path(radar.SourceAmesh, testTile, testDate)
			buf, err := os.ReadFile(p)
			assertion.MustNoError(t, err)
			assertion.MustNoError(t, os.WriteFile(p, tc.break_(buf), 0644))

			frames, err := w.Load(radar.SourceAmesh, testTile, testDate)
			assertion.MustNoError(t, err)
			assertion.Len(t, frames, 1)
			assertion.Equal(t, frames[0], testFrame(testDate, 1).Values)
		})
	}
}

func TestPath(t *testing.T) {
	cases := []struct {
		name   string
		source radar.SourceID
		tile   radar.Tile
		at     time.Time
		want   string
	}{
		{
			name:   "jst",
			source: radar.SourceXRAIN,
			tile:   radar.Tile{Z: 10, X: 905, Y: 402},
			at:     time.Date(2026, 7, 16, 12, 0, 0, 0, radar.JST),
			want:   filepath.Join("/var/lib/radar", "xrain", "10", "905", "402", "20260716.wal"),
		},
		{
			name:   "utc rolls over",
			source: radar.SourceAmesh,
			tile:   radar.Tile{Z: 10, X: 1, Y: 2},
			at:     time.Date(2026, 7, 16, 16, 0, 0, 0, time.UTC),
			want:   filepath.Join("/var/lib/radar", "amesh", "10", "1", "2", "20260717.wal"),
		},
	}

	w := New("/var/lib/radar", time.Minute)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertion.Equal(t, w.Path(tc.source, tc.tile, tc.at), tc.want)
		})
	}
}

func TestList(t *testing.T) {
	t.Run("only the date", func(t *testing.T) {
		w := New(t.TempDir(), time.Minute)
		for _, tile := range []radar.Tile{{Z: 10, X: 909, Y: 403}, {Z: 10, X: 910, Y: 403}} {
			f := testFrame(testDate, 1)
			f.Tile = tile
			assertion.MustNoError(t, w.Append(f))
		}
		assertion.MustNoError(t, w.Append(testFrame(testDate.AddDate(0, 0, 1), 1)))

		entries, err := w.List(testDate)
		assertion.MustNoError(t, err)
		assertion.Len(t, entries, 2)
		for _, e := range entries {
			assertion.Equal(t, e.Source, radar.SourceAmesh)
			assertion.Equal(t, e.Date.Format("20060102"), "20260716")
		}
	})

	t.Run("a missing directory is not an error", func(t *testing.T) {
		w := New(filepath.Join(t.TempDir(), "does-not-exist"), time.Minute)
		entries, err := w.List(testDate)
		assertion.MustNoError(t, err)
		assertion.Empty(t, entries)
	})
}

func TestRemove(t *testing.T) {
	w := New(t.TempDir(), time.Minute)
	assertion.MustNoError(t, w.Append(testFrame(testDate, 1)))

	t.Run("removes the file", func(t *testing.T) {
		assertion.MustNoError(t, w.Remove(radar.SourceAmesh, testTile, testDate))
		_, err := os.Stat(w.Path(radar.SourceAmesh, testTile, testDate))
		assertion.True(t, os.IsNotExist(err))
	})

	t.Run("removing twice is not an error", func(t *testing.T) {
		assertion.MustNoError(t, w.Remove(radar.SourceAmesh, testTile, testDate))
	})
}
