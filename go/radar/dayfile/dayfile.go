// Package dayfile encodes and decodes the daily container of the radar rainfall archive.
//
// The container holds the frames of one day of a single (Source, Tile). The first available frame
// of the day is stored as it is and the rest are stored as the XOR difference from the previous
// available frame. The whole byte sequence is compressed by zstd.
//
// This package depends on neither the object storage nor HTTP. It only takes io.Reader and
// io.Writer so that it can be used as a library.
package dayfile

import (
	"bytes"
	"crypto/subtle"
	"encoding/binary"
	"io"
	"time"

	"github.com/klauspost/compress/zstd"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/radar"
)

const (
	// FormatVersion is the version of the container format.
	FormatVersion uint16 = 1

	headerSize     = 32
	indexEntrySize = 8

	// maxFrameLength is the limit of the length field of the index entry. It is 3 bytes.
	maxFrameLength = 1<<24 - 1
)

var magic = [4]byte{'R', 'A', 'C', '1'}

var (
	ErrFrameMissing  = xerrors.Define("dayfile: frame is missing")
	ErrInvalidFormat = xerrors.Define("dayfile: invalid format")
	ErrInvalidHeader = xerrors.Define("dayfile: invalid header")
)

// Kind is the type of a frame in the container.
type Kind uint8

const (
	// KindMissing means that the frame was not collected. The offset and the length are zero.
	KindMissing Kind = iota
	// KindKeyframe means that the frame is stored as the raw values.
	KindKeyframe
	// KindDelta means that the frame is the XOR difference from the previous available frame.
	KindDelta
	// KindUnchanged means that the frame is identical to the previous available frame.
	KindUnchanged
)

func (k Kind) String() string {
	switch k {
	case KindMissing:
		return "missing"
	case KindKeyframe:
		return "keyframe"
	case KindDelta:
		return "delta"
	case KindUnchanged:
		return "unchanged"
	default:
		return "unknown"
	}
}

const flagFinalized uint16 = 1 << 0

// Header is the header of the container.
type Header struct {
	FormatVersion uint16
	Source        radar.SourceID
	Tile          radar.Tile
	ScaleVersion  uint16
	// DateJST is the midnight of JST of the day.
	DateJST time.Time
	// TileSize is the number of cells on a side. It is square because the frame is normalized into
	// the common mesh.
	TileSize   int
	Interval   time.Duration
	FrameCount int
	Finalized  bool
}

// frameSize returns the number of bytes of a single frame.
func (h Header) frameSize() int {
	return h.TileSize * h.TileSize
}

func (h Header) validate() error {
	if h.FormatVersion != FormatVersion {
		return xerrors.WithMessagef(ErrInvalidHeader, "unsupported format version: %d", h.FormatVersion)
	}
	if h.TileSize <= 0 || h.TileSize > 1<<15 {
		return xerrors.WithMessagef(ErrInvalidHeader, "invalid tile size: %d", h.TileSize)
	}
	if h.Interval <= 0 || h.Interval%time.Second != 0 || h.Interval/time.Second > 1<<16-1 {
		return xerrors.WithMessagef(ErrInvalidHeader, "invalid interval: %s", h.Interval)
	}
	// The slot of a time is the elapsed time divided by the interval, so an interval that does not
	// divide a day would make the last slot fall outside of FrameCount.
	if 24*time.Hour%h.Interval != 0 {
		return xerrors.WithMessagef(ErrInvalidHeader, "the interval does not divide a day: %s", h.Interval)
	}
	if h.FrameCount <= 0 || h.FrameCount > 1<<16-1 {
		return xerrors.WithMessagef(ErrInvalidHeader, "invalid frame count: %d", h.FrameCount)
	}
	if !h.Tile.Valid() {
		return xerrors.WithMessagef(ErrInvalidHeader, "invalid tile: %s", h.Tile.String())
	}
	if h.DateJST.IsZero() {
		return xerrors.WithMessage(ErrInvalidHeader, "date is not set")
	}
	return nil
}

func (h Header) marshal() []byte {
	b := make([]byte, headerSize)
	copy(b[0:4], magic[:])
	binary.LittleEndian.PutUint16(b[4:6], h.FormatVersion)
	binary.LittleEndian.PutUint16(b[6:8], uint16(h.Source))
	binary.LittleEndian.PutUint16(b[8:10], h.ScaleVersion)
	b[10] = uint8(h.Tile.Z)
	// b[11] is a padding.
	binary.LittleEndian.PutUint32(b[12:16], uint32(h.Tile.X))
	binary.LittleEndian.PutUint32(b[16:20], uint32(h.Tile.Y))
	d := h.DateJST.In(radar.JST)
	binary.LittleEndian.PutUint32(b[20:24], uint32(d.Year()*10000+int(d.Month())*100+d.Day()))
	binary.LittleEndian.PutUint16(b[24:26], uint16(h.TileSize))
	binary.LittleEndian.PutUint16(b[26:28], uint16(h.Interval/time.Second))
	binary.LittleEndian.PutUint16(b[28:30], uint16(h.FrameCount))
	var flags uint16
	if h.Finalized {
		flags |= flagFinalized
	}
	binary.LittleEndian.PutUint16(b[30:32], flags)
	return b
}

func unmarshalHeader(b []byte) (Header, error) {
	if len(b) < headerSize {
		return Header{}, xerrors.WithMessage(ErrInvalidFormat, "too short for the header")
	}
	if !bytes.Equal(b[0:4], magic[:]) {
		return Header{}, xerrors.WithMessagef(ErrInvalidFormat, "bad magic: %q", b[0:4])
	}
	date := binary.LittleEndian.Uint32(b[20:24])
	y, m, d := int(date/10000), int(date/100%100), int(date%100)
	if m < 1 || m > 12 || d < 1 || d > 31 {
		return Header{}, xerrors.WithMessagef(ErrInvalidFormat, "invalid date: %d", date)
	}
	h := Header{
		FormatVersion: binary.LittleEndian.Uint16(b[4:6]),
		Source:        radar.SourceID(binary.LittleEndian.Uint16(b[6:8])),
		ScaleVersion:  binary.LittleEndian.Uint16(b[8:10]),
		Tile: radar.Tile{
			Z: int(b[10]),
			X: int(binary.LittleEndian.Uint32(b[12:16])),
			Y: int(binary.LittleEndian.Uint32(b[16:20])),
		},
		DateJST:    time.Date(y, time.Month(m), d, 0, 0, 0, 0, radar.JST),
		TileSize:   int(binary.LittleEndian.Uint16(b[24:26])),
		Interval:   time.Duration(binary.LittleEndian.Uint16(b[26:28])) * time.Second,
		FrameCount: int(binary.LittleEndian.Uint16(b[28:30])),
		Finalized:  binary.LittleEndian.Uint16(b[30:32])&flagFinalized != 0,
	}
	if err := h.validate(); err != nil {
		return Header{}, err
	}
	return h, nil
}

// indexEntry is an entry of the frame index.
type indexEntry struct {
	Offset uint32
	Length uint32 // Only the lower 3 bytes are stored.
	Kind   Kind
}

func (e indexEntry) marshal(b []byte) {
	binary.LittleEndian.PutUint32(b[0:4], e.Offset)
	b[4] = byte(e.Length)
	b[5] = byte(e.Length >> 8)
	b[6] = byte(e.Length >> 16)
	b[7] = byte(e.Kind)
}

func unmarshalIndexEntry(b []byte) indexEntry {
	return indexEntry{
		Offset: binary.LittleEndian.Uint32(b[0:4]),
		Length: uint32(b[4]) | uint32(b[5])<<8 | uint32(b[6])<<16,
		Kind:   Kind(b[7]),
	}
}

// Writer builds a daily container.
//
// The Writer builds the whole file at once. It does not support appending because the whole byte
// sequence is compressed by zstd and appending to the tail is not possible. The collector keeps
// the frames in the WAL and passes them to the Writer when the day is finalized.
type Writer struct {
	h      Header
	frames map[int][]byte
}

// NewWriter returns a Writer that builds an empty daily container.
func NewWriter(h Header) *Writer {
	if h.FormatVersion == 0 {
		h.FormatVersion = FormatVersion
	}
	return &Writer{h: h, frames: make(map[int][]byte)}
}

// SetFrame sets the values of the slot.
//
// The difference from the previous frame and the kind of the frame are decided by the Writer.
// The order of the calls does not have to be ascending. The values are copied so that the caller
// can reuse the buffer.
func (w *Writer) SetFrame(slot int, values []byte) error {
	if err := w.h.validate(); err != nil {
		return err
	}
	if slot < 0 || slot >= w.h.FrameCount {
		return xerrors.NewfWithStack("dayfile: slot is out of range: %d", slot)
	}
	if len(values) != w.h.frameSize() {
		return xerrors.NewfWithStack("dayfile: unexpected length of the values: %d != %d", len(values), w.h.frameSize())
	}
	w.frames[slot] = bytes.Clone(values)
	return nil
}

// WriteTo writes the zstd compressed byte sequence.
func (w *Writer) WriteTo(dst io.Writer) (int64, error) {
	if err := w.h.validate(); err != nil {
		return 0, err
	}

	indexSize := w.h.FrameCount * indexEntrySize
	buf := bytes.NewBuffer(make([]byte, 0, headerSize+indexSize+w.h.frameSize()))
	buf.Write(w.h.marshal())
	index := make([]byte, indexSize)
	buf.Write(index)

	var offset uint32
	var prev []byte
	delta := make([]byte, w.h.frameSize())
	for slot := 0; slot < w.h.FrameCount; slot++ {
		v, ok := w.frames[slot]
		if !ok {
			// The absence of the frame in the WAL is the missing frame. No record is needed.
			indexEntry{Kind: KindMissing}.marshal(index[slot*indexEntrySize:])
			continue
		}

		var e indexEntry
		switch {
		case prev == nil:
			e = indexEntry{Offset: offset, Length: uint32(len(v)), Kind: KindKeyframe}
			buf.Write(v)
		case bytes.Equal(prev, v):
			e = indexEntry{Offset: offset, Kind: KindUnchanged}
		default:
			subtle.XORBytes(delta, prev, v)
			e = indexEntry{Offset: offset, Length: uint32(len(delta)), Kind: KindDelta}
			buf.Write(delta)
		}
		if e.Length > maxFrameLength {
			return 0, xerrors.NewfWithStack("dayfile: frame is too large: %d", e.Length)
		}
		e.marshal(index[slot*indexEntrySize:])
		offset += e.Length
		prev = v
	}
	// The index was written to the buffer before it was filled in.
	copy(buf.Bytes()[headerSize:], index)

	cw := &countWriter{w: dst}
	enc, err := zstd.NewWriter(cw)
	if err != nil {
		return 0, xerrors.WithStack(err)
	}
	if _, err := enc.Write(buf.Bytes()); err != nil {
		enc.Close()
		return cw.n, xerrors.WithStack(err)
	}
	if err := enc.Close(); err != nil {
		return cw.n, xerrors.WithStack(err)
	}
	return cw.n, nil
}

type countWriter struct {
	w io.Writer
	n int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// Reader reads a daily container.
//
// The whole byte sequence is expanded on memory for the random access.
type Reader struct {
	h      Header
	index  []indexEntry
	frames []byte

	// cacheSlot is the slot of cacheBuf. -1 means that the cache is invalid.
	cacheSlot int
	cacheBuf  []byte
}

// NewReader reads the whole zstd compressed byte sequence.
func NewReader(src io.Reader) (*Reader, error) {
	dec, err := zstd.NewReader(src)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	defer dec.Close()
	raw, err := io.ReadAll(dec)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	h, err := unmarshalHeader(raw)
	if err != nil {
		return nil, err
	}
	indexSize := h.FrameCount * indexEntrySize
	if len(raw) < headerSize+indexSize {
		return nil, xerrors.WithMessage(ErrInvalidFormat, "too short for the frame index")
	}
	frames := raw[headerSize+indexSize:]

	index := make([]indexEntry, h.FrameCount)
	frameSize := h.frameSize()
	for i := range index {
		e := unmarshalIndexEntry(raw[headerSize+i*indexEntrySize:])
		switch e.Kind {
		case KindMissing, KindUnchanged:
			// These do not have a body.
		case KindKeyframe, KindDelta:
			if int(e.Length) != frameSize {
				return nil, xerrors.WithMessagef(ErrInvalidFormat, "slot %d: unexpected frame length: %d", i, e.Length)
			}
			if int(e.Offset)+int(e.Length) > len(frames) {
				return nil, xerrors.WithMessagef(ErrInvalidFormat, "slot %d: frame is out of the range", i)
			}
		default:
			return nil, xerrors.WithMessagef(ErrInvalidFormat, "slot %d: unknown kind: %d", i, e.Kind)
		}
		index[i] = e
	}

	return &Reader{h: h, index: index, frames: frames, cacheSlot: -1}, nil
}

func (r *Reader) Header() Header {
	return r.h
}

// Kinds returns the kind of every slot. It is for investigating the missing frames.
func (r *Reader) Kinds() []Kind {
	kinds := make([]Kind, len(r.index))
	for i, e := range r.index {
		kinds[i] = e.Kind
	}
	return kinds
}

// Frame returns the values of the slot. The difference from the keyframe is applied internally.
func (r *Reader) Frame(slot int) ([]byte, error) {
	v, err := r.frame(slot)
	if err != nil {
		return nil, err
	}
	return bytes.Clone(v), nil
}

// FrameAt returns the values of the slot of the time.
func (r *Reader) FrameAt(t time.Time) ([]byte, error) {
	return r.Frame(radar.Slot(t, r.h.Interval))
}

// Cell returns the band index of a single cell.
//
// It avoids copying the whole frame which matters when a time series of one point is taken.
func (r *Reader) Cell(slot, px, py int) (uint8, error) {
	if px < 0 || px >= r.h.TileSize || py < 0 || py >= r.h.TileSize {
		return 0, xerrors.NewfWithStack("dayfile: cell is out of range: (%d, %d)", px, py)
	}
	v, err := r.frame(slot)
	if err != nil {
		return 0, err
	}
	return v[py*r.h.TileSize+px], nil
}

// frame returns the internal buffer of the slot. The caller must not modify or retain it.
func (r *Reader) frame(slot int) ([]byte, error) {
	if slot < 0 || slot >= len(r.index) {
		return nil, xerrors.NewfWithStack("dayfile: slot is out of range: %d", slot)
	}
	if r.index[slot].Kind == KindMissing {
		return nil, xerrors.WithMessagef(ErrFrameMissing, "slot %d", slot)
	}

	// Scan backward to find the nearest keyframe.
	keyframe := -1
	for i := slot; i >= 0; i-- {
		if r.index[i].Kind == KindKeyframe {
			keyframe = i
			break
		}
	}
	if keyframe < 0 {
		return nil, xerrors.WithMessagef(ErrInvalidFormat, "slot %d: keyframe is not found", slot)
	}

	// The cache can be continued only when no keyframe exists between the cached slot and the
	// requested slot. Requiring the cached slot to be at or after the nearest keyframe is exactly
	// that condition.
	start := keyframe
	if r.cacheSlot >= keyframe && r.cacheSlot <= slot {
		if r.cacheSlot == slot {
			return r.cacheBuf, nil
		}
		start = r.cacheSlot
	} else {
		e := r.index[keyframe]
		r.cacheBuf = append(r.cacheBuf[:0], r.frames[e.Offset:e.Offset+e.Length]...)
	}

	for i := start + 1; i <= slot; i++ {
		e := r.index[i]
		switch e.Kind {
		case KindMissing, KindUnchanged:
			// Nothing to apply.
		case KindDelta:
			d := r.frames[e.Offset : e.Offset+e.Length]
			subtle.XORBytes(r.cacheBuf, r.cacheBuf, d)
		case KindKeyframe:
			r.cacheBuf = append(r.cacheBuf[:0], r.frames[e.Offset:e.Offset+e.Length]...)
		}
	}
	r.cacheSlot = slot
	return r.cacheBuf, nil
}
