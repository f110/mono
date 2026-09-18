package dayfile

import (
	"bytes"
	"math/rand/v2"
	"testing"
	"time"

	"go.f110.dev/mono/go/radar"
	"go.f110.dev/mono/go/testing/assertion"
)

func testHeader() Header {
	return Header{
		Source:       radar.SourceAmesh,
		Tile:         radar.Tile{Z: 10, X: 909, Y: 403},
		ScaleVersion: 1,
		DateJST:      time.Date(2026, 7, 16, 0, 0, 0, 0, radar.JST),
		TileSize:     radar.TileSize,
		Interval:     time.Minute,
		FrameCount:   radar.FrameCount(time.Minute),
	}
}

func smallHeader() Header {
	h := testHeader()
	h.TileSize = 4
	h.FrameCount = 8
	return h
}

func fillFrame(size int, v byte) []byte {
	b := make([]byte, size)
	for i := range b {
		b[i] = v
	}
	return b
}

func TestHeaderRoundTrip(t *testing.T) {
	h := testHeader()
	h.FormatVersion = FormatVersion
	h.Finalized = true

	got, err := unmarshalHeader(h.marshal())
	assertion.MustNoError(t, err)
	assertion.Equal(t, got, h)
}

func TestHeaderRejectsBadInput(t *testing.T) {
	h := testHeader()
	h.FormatVersion = FormatVersion
	b := h.marshal()

	cases := []struct {
		name string
		in   []byte
	}{
		{name: "bad magic", in: append([]byte{'X'}, bytes.Clone(b)[1:]...)},
		{name: "shorter than the header", in: b[:10]},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := unmarshalHeader(tc.in)
			assertion.Error(t, err)
		})
	}
}

func TestWriteAndRead(t *testing.T) {
	h := smallHeader()
	size := h.frameSize()

	frames := map[int][]byte{
		0: fillFrame(size, 1),
		1: fillFrame(size, 1),
		2: fillFrame(size, 2),
		4: fillFrame(size, 3),
		7: fillFrame(size, 4),
	}

	w := NewWriter(h)
	for slot, v := range frames {
		assertion.MustNoError(t, w.SetFrame(slot, v))
	}
	buf := &bytes.Buffer{}
	n, err := w.WriteTo(buf)
	assertion.MustNoError(t, err)
	assertion.Equal(t, n, int64(buf.Len()))

	r, err := NewReader(buf)
	assertion.MustNoError(t, err)
	assertion.Equal(t, r.Header().Tile, h.Tile)
	assertion.Equal(t, r.Header().FrameCount, h.FrameCount)
	assertion.Equal(t, r.Kinds(), []Kind{
		KindKeyframe,
		KindUnchanged,
		KindDelta,
		KindMissing,
		KindDelta,
		KindMissing,
		KindMissing,
		KindDelta,
	})

	t.Run("every slot round trips", func(t *testing.T) {
		for slot, want := range frames {
			got, err := r.Frame(slot)
			assertion.MustNoError(t, err)
			assertion.Equal(t, got, want)
		}
	})

	cases := []struct {
		name string
		slot int
	}{
		{name: "missing", slot: 3},
		{name: "negative", slot: -1},
		{name: "beyond the last", slot: h.FrameCount},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.Frame(tc.slot)
			assertion.Error(t, err)
		})
	}
}

func TestReadInRandomOrder(t *testing.T) {
	h := smallHeader()
	h.FrameCount = 64
	size := h.frameSize()

	want := make([][]byte, h.FrameCount)
	w := NewWriter(h)
	for slot := 0; slot < h.FrameCount; slot++ {
		v := fillFrame(size, byte(slot))
		want[slot] = v
		assertion.MustNoError(t, w.SetFrame(slot, v))
	}
	buf := &bytes.Buffer{}
	_, err := w.WriteTo(buf)
	assertion.MustNoError(t, err)

	r, err := NewReader(buf)
	assertion.MustNoError(t, err)

	t.Run("random order", func(t *testing.T) {
		for _, slot := range rand.Perm(h.FrameCount) {
			got, err := r.Frame(slot)
			assertion.MustNoError(t, err)
			assertion.Equal(t, got, want[slot])
		}
	})

	t.Run("repeated slots", func(t *testing.T) {
		for _, slot := range []int{0, 63, 63, 0, 32} {
			got, err := r.Frame(slot)
			assertion.MustNoError(t, err)
			assertion.Equal(t, got, want[slot])
		}
	})
}

func TestFrameIsCopied(t *testing.T) {
	h := smallHeader()
	size := h.frameSize()
	w := NewWriter(h)
	assertion.MustNoError(t, w.SetFrame(0, fillFrame(size, 1)))
	assertion.MustNoError(t, w.SetFrame(1, fillFrame(size, 2)))
	buf := &bytes.Buffer{}
	_, err := w.WriteTo(buf)
	assertion.MustNoError(t, err)

	r, err := NewReader(buf)
	assertion.MustNoError(t, err)
	got, err := r.Frame(1)
	assertion.MustNoError(t, err)
	for i := range got {
		got[i] = 0xff
	}
	again, err := r.Frame(1)
	assertion.MustNoError(t, err)
	assertion.Equal(t, again, fillFrame(size, 2))
}

func TestSetFrameIsCopied(t *testing.T) {
	h := smallHeader()
	size := h.frameSize()
	w := NewWriter(h)
	v := fillFrame(size, 1)
	assertion.MustNoError(t, w.SetFrame(0, v))
	for i := range v {
		v[i] = 9
	}
	buf := &bytes.Buffer{}
	_, err := w.WriteTo(buf)
	assertion.MustNoError(t, err)

	r, err := NewReader(buf)
	assertion.MustNoError(t, err)
	got, err := r.Frame(0)
	assertion.MustNoError(t, err)
	assertion.Equal(t, got, fillFrame(size, 1))
}

func TestKeyframeIsTheFirstAvailableFrame(t *testing.T) {
	h := smallHeader()
	size := h.frameSize()
	w := NewWriter(h)
	assertion.MustNoError(t, w.SetFrame(5, fillFrame(size, 7)))
	assertion.MustNoError(t, w.SetFrame(6, fillFrame(size, 8)))
	buf := &bytes.Buffer{}
	_, err := w.WriteTo(buf)
	assertion.MustNoError(t, err)

	r, err := NewReader(buf)
	assertion.MustNoError(t, err)
	kinds := r.Kinds()
	assertion.Equal(t, kinds[5], KindKeyframe)
	assertion.Equal(t, kinds[6], KindDelta)

	got, err := r.Frame(6)
	assertion.MustNoError(t, err)
	assertion.Equal(t, got, fillFrame(size, 8))
}

func TestDeltaSkipsMissing(t *testing.T) {
	h := smallHeader()
	size := h.frameSize()
	w := NewWriter(h)
	assertion.MustNoError(t, w.SetFrame(0, fillFrame(size, 1)))
	assertion.MustNoError(t, w.SetFrame(3, fillFrame(size, 5)))
	buf := &bytes.Buffer{}
	_, err := w.WriteTo(buf)
	assertion.MustNoError(t, err)

	r, err := NewReader(buf)
	assertion.MustNoError(t, err)
	got, err := r.Frame(3)
	assertion.MustNoError(t, err)
	assertion.Equal(t, got, fillFrame(size, 5))
}

func TestCell(t *testing.T) {
	h := smallHeader()
	size := h.frameSize()
	v := make([]byte, size)
	v[0] = 3
	v[h.TileSize+2] = 9

	w := NewWriter(h)
	assertion.MustNoError(t, w.SetFrame(0, v))
	buf := &bytes.Buffer{}
	_, err := w.WriteTo(buf)
	assertion.MustNoError(t, err)

	r, err := NewReader(buf)
	assertion.MustNoError(t, err)

	cases := []struct {
		name    string
		px, py  int
		want    uint8
		wantErr bool
	}{
		{name: "origin", px: 0, py: 0, want: 3},
		{name: "second row", px: 2, py: 1, want: 9},
		{name: "out of range", px: h.TileSize, py: 0, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.Cell(0, tc.px, tc.py)
			if tc.wantErr {
				assertion.Error(t, err)
				return
			}
			assertion.MustNoError(t, err)
			assertion.Equal(t, got, tc.want)
		})
	}
}

func TestFrameAt(t *testing.T) {
	h := smallHeader()
	h.FrameCount = radar.FrameCount(time.Minute)
	size := h.frameSize()
	w := NewWriter(h)
	slot := radar.Slot(time.Date(2026, 7, 16, 23, 59, 0, 0, radar.JST), time.Minute)
	assertion.Equal(t, slot, 1439)
	assertion.MustNoError(t, w.SetFrame(0, fillFrame(size, 1)))
	assertion.MustNoError(t, w.SetFrame(slot, fillFrame(size, 2)))
	buf := &bytes.Buffer{}
	_, err := w.WriteTo(buf)
	assertion.MustNoError(t, err)

	r, err := NewReader(buf)
	assertion.MustNoError(t, err)
	got, err := r.FrameAt(time.Date(2026, 7, 16, 23, 59, 30, 0, radar.JST))
	assertion.MustNoError(t, err)
	assertion.Equal(t, got, fillFrame(size, 2))
}

func TestSetFrameValidation(t *testing.T) {
	h := smallHeader()

	cases := []struct {
		name string
		slot int
		size int
	}{
		{name: "negative slot", slot: -1, size: h.frameSize()},
		{name: "slot beyond the last", slot: h.FrameCount, size: h.frameSize()},
		{name: "unexpected length", slot: 0, size: h.frameSize() - 1},
	}

	w := NewWriter(h)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertion.Error(t, w.SetFrame(tc.slot, fillFrame(tc.size, 1)))
		})
	}
}

func TestEmptyDay(t *testing.T) {
	h := smallHeader()
	w := NewWriter(h)
	buf := &bytes.Buffer{}
	_, err := w.WriteTo(buf)
	assertion.MustNoError(t, err)

	r, err := NewReader(buf)
	assertion.MustNoError(t, err)
	for _, k := range r.Kinds() {
		assertion.Equal(t, k, KindMissing)
	}
	_, err = r.Frame(0)
	assertion.Error(t, err)
}

func TestReadBrokenInput(t *testing.T) {
	_, err := NewReader(bytes.NewReader([]byte("not a zstd stream")))
	assertion.Error(t, err)
}

func TestFullDay(t *testing.T) {
	h := testHeader()
	size := h.frameSize()
	w := NewWriter(h)

	base := make([]byte, size)
	for slot := 0; slot < h.FrameCount; slot++ {
		v := bytes.Clone(base)
		for i := 0; i < 64; i++ {
			v[(slot*7+i)%size] = byte(i%11) + 1
		}
		assertion.MustNoError(t, w.SetFrame(slot, v))
	}

	buf := &bytes.Buffer{}
	_, err := w.WriteTo(buf)
	assertion.MustNoError(t, err)
	compressed := buf.Len()

	r, err := NewReader(bytes.NewReader(buf.Bytes()))
	assertion.MustNoError(t, err)

	last, err := r.Frame(h.FrameCount - 1)
	assertion.MustNoError(t, err)
	want := bytes.Clone(base)
	for i := 0; i < 64; i++ {
		want[((h.FrameCount-1)*7+i)%size] = byte(i%11) + 1
	}
	assertion.Equal(t, last, want)

	raw := size * h.FrameCount
	assertion.True(t, compressed < raw/10)
	t.Logf("raw=%d compressed=%d ratio=%.4f", raw, compressed, float64(compressed)/float64(raw))
}
