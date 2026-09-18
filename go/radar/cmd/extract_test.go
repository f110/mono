package cmd

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.f110.dev/mono/go/radar"
	"go.f110.dev/mono/go/radar/amesh"
	"go.f110.dev/mono/go/radar/dayfile"
	"go.f110.dev/mono/go/testing/assertion"
)

func seedArchive(t *testing.T, store *memoryStore, slots map[int]byte) {
	t.Helper()
	h := dayfile.Header{
		Source:       radar.SourceAmesh,
		Tile:         testTile(),
		ScaleVersion: amesh.ScaleVersion1,
		DateJST:      testDate(),
		TileSize:     radar.TileSize,
		Interval:     time.Minute,
		FrameCount:   radar.FrameCount(time.Minute),
		Finalized:    true,
	}
	w := dayfile.NewWriter(h)
	for slot, v := range slots {
		assertion.MustNoError(t, w.SetFrame(slot, bytes.Repeat([]byte{v}, radar.TileSize*radar.TileSize)))
	}
	buf := &bytes.Buffer{}
	_, err := w.WriteTo(buf)
	assertion.MustNoError(t, err)
	assertion.MustNoError(t, store.PutReader(context.Background(), objectKey(h.Source, h.Tile, h.DateJST), buf))
}

func runExtract(t *testing.T, f *extractFlags, store *memoryStore) (string, error) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "out")
	file, err := os.Create(p)
	assertion.MustNoError(t, err)
	err = extract(context.Background(), f, store, file)
	assertion.MustNoError(t, file.Close())
	buf, readErr := os.ReadFile(p)
	assertion.MustNoError(t, readErr)
	return string(buf), err
}

func TestExtractPoint(t *testing.T) {
	store := newMemoryStore()
	seedArchive(t, store, map[int]byte{0: 5, 1: radar.BandOutOfRange})

	lat, lon := testTile().Cell(128, 128)
	out, err := runExtract(t, &extractFlags{
		source:   "amesh",
		zoom:     10,
		lat:      lat,
		lon:      lon,
		date:     "2026-07-16",
		timeSpan: "00:00-00:02",
	}, store)
	assertion.MustNoError(t, err)

	lines := strings.Split(strings.TrimSpace(out), "\n")
	assertion.Len(t, lines, 3)
	assertion.Equal(t, lines[0], "2026-07-16T00:00:00+09:00\t10-20")
	assertion.Equal(t, lines[1], "2026-07-16T00:01:00+09:00\tout of range")
	assertion.Equal(t, lines[2], "2026-07-16T00:02:00+09:00\tmissing")
}

func TestExtractPointAtASingleTime(t *testing.T) {
	store := newMemoryStore()
	seedArchive(t, store, map[int]byte{720: 11})

	lat, lon := testTile().Cell(0, 0)
	out, err := runExtract(t, &extractFlags{
		source: "amesh",
		zoom:   10,
		lat:    lat,
		lon:    lon,
		at:     "2026-07-16T12:00:00+09:00",
	}, store)
	assertion.MustNoError(t, err)
	assertion.Equal(t, strings.TrimSpace(out), "2026-07-16T12:00:00+09:00\t75-")
}

func TestExtractTileAsImage(t *testing.T) {
	store := newMemoryStore()
	seedArchive(t, store, map[int]byte{0: 5})

	out := filepath.Join(t.TempDir(), "frame.png")
	_, err := runExtract(t, &extractFlags{
		source: "amesh",
		tile:   testTile().String(),
		at:     "2026-07-16T00:00:00+09:00",
		output: out,
	}, store)
	assertion.MustNoError(t, err)

	file, err := os.Open(out)
	assertion.MustNoError(t, err)
	defer file.Close()
	img, err := png.Decode(file)
	assertion.MustNoError(t, err)
	assertion.Equal(t, img.Bounds().Dx(), radar.TileSize)

	r, g, b, a := img.At(0, 0).RGBA()
	assertion.Equal(t, [4]uint32{r >> 8, g >> 8, b >> 8, a >> 8}, [4]uint32{51, 51, 255, 255})
}

func TestExtractRangeWritesMultipleImages(t *testing.T) {
	store := newMemoryStore()
	seedArchive(t, store, map[int]byte{0: 5, 1: 6, 2: 7})

	dir := filepath.Join(t.TempDir(), "frames")
	_, err := runExtract(t, &extractFlags{
		source:   "amesh",
		tile:     testTile().String(),
		date:     "2026-07-16",
		timeSpan: "00:00-00:02",
		output:   dir,
	}, store)
	assertion.MustNoError(t, err)

	entries, err := os.ReadDir(dir)
	assertion.MustNoError(t, err)
	assertion.Len(t, entries, 3)
	assertion.Equal(t, entries[0].Name(), "0000.png")
	assertion.Equal(t, entries[2].Name(), "0002.png")
}

func TestExtractRejectsBadInput(t *testing.T) {
	seeded := newMemoryStore()
	seedArchive(t, seeded, map[int]byte{0: 5})

	cases := []struct {
		name  string
		flags *extractFlags
		store *memoryStore
	}{
		{
			name:  "neither a tile nor a coordinate",
			flags: &extractFlags{source: "amesh", zoom: 10, date: "2026-07-16"},
			store: seeded,
		},
		{
			name:  "the zoom does not match the collector",
			flags: &extractFlags{source: "amesh", zoom: 11, lat: 35.68, lon: 139.76, at: "2026-07-16T00:00:00+09:00"},
			store: seeded,
		},
		{
			name:  "no time",
			flags: &extractFlags{source: "amesh", tile: testTile().String()},
			store: seeded,
		},
		{
			name:  "the object does not exist",
			flags: &extractFlags{source: "amesh", tile: testTile().String(), at: "2026-07-16T00:00:00+09:00"},
			store: newMemoryStore(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runExtract(t, tc.flags, tc.store)
			assertion.Error(t, err)
		})
	}
}

func TestDrawingPalette(t *testing.T) {
	cases := []struct {
		name    string
		source  radar.SourceID
		wantErr bool
	}{
		{name: "amesh", source: radar.SourceAmesh},
		{name: "xrain", source: radar.SourceXRAIN},
		{name: "no palette in the code", source: radar.SourceID(9999), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := drawingPalette(tc.source, "")
			if tc.wantErr {
				assertion.Error(t, err)
				return
			}
			assertion.MustNoError(t, err)
			assertion.NotNil(t, p)
		})
	}
}

func TestLoadPalette(t *testing.T) {
	p := filepath.Join(t.TempDir(), "palette.json")
	assertion.MustNoError(t, os.WriteFile(p, []byte(`[
		{"color": "#ccffff", "min_mmh": 1, "max_mmh": 5},
		{"color": "#7030a0", "min_mmh": 75}
	]`), 0644))

	entries, err := loadPalette(p)
	assertion.MustNoError(t, err)
	assertion.Len(t, entries, 2)
	assertion.Equal(t, entries[0].MinMMH, float64(1))
	assertion.Equal(t, entries[0].MaxMMH, float64(5))
	assertion.True(t, entries[1].MaxMMH > 1e300)
	assertion.Equal(t, entries[1].Color.R, uint8(0x70))
}

func TestLoadPaletteRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "no entry", body: `[]`},
		{name: "invalid color", body: `[{"color": "zzz", "min_mmh": 1}]`},
		{name: "not json", body: `not json`},
	}

	dir := t.TempDir()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(dir, "p.json")
			assertion.MustNoError(t, os.WriteFile(p, []byte(tc.body), 0644))
			_, err := loadPalette(p)
			assertion.Error(t, err)
		})
	}

	t.Run("the file does not exist", func(t *testing.T) {
		_, err := loadPalette(filepath.Join(dir, "does-not-exist.json"))
		assertion.Error(t, err)
	})
}
