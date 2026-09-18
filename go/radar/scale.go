package radar

import (
	"math"
	"sync"

	"go.f110.dev/xerrors"
)

// BandOutOfRange is the band index of the cell that the source does not cover.
// It is distinct from the band of no rainfall.
const BandOutOfRange uint8 = 0

var ErrUnknownBand = xerrors.Define("radar: unknown band")

// Band is a class of the rainfall intensity. The range is [MinMMH, MaxMMH).
type Band struct {
	Index  uint8
	MinMMH float64
	MaxMMH float64
}

// ScaleTable is the table of the rainfall classes of a source.
//
// The table is per source, not shared. The two sources that are implemented divide the rainfall
// into classes that do not refine each other: the [30, 50) of XRAIN straddles the boundary at 40
// of Tokyo Amesh, and its [50, 80) straddles the 60 and the 75. A single table that every source
// maps into would therefore have to either merge the classes and throw away resolution, or split
// them and invent a precision that the source never reported.
//
// The values that are stored are the index of the table of the source, and the file header records
// both the Source and the Version, so a stored frame is always decoded with the table it was
// written with. A consumer that wants to compare the sources converts through the tables.
type ScaleTable struct {
	Source  SourceID
	Version uint16
	Bands   []Band
}

var (
	scaleMu     sync.RWMutex
	scaleTables = make(map[SourceID]map[uint16]*ScaleTable)
	scaleLatest = make(map[SourceID]*ScaleTable)
)

// RegisterScale adds the table of a source. The table of the largest version becomes the current
// one. It is called by the package of each source.
func RegisterScale(t *ScaleTable) {
	scaleMu.Lock()
	defer scaleMu.Unlock()
	if _, ok := scaleTables[t.Source]; !ok {
		scaleTables[t.Source] = make(map[uint16]*ScaleTable)
	}
	scaleTables[t.Source][t.Version] = t
	if cur, ok := scaleLatest[t.Source]; !ok || t.Version > cur.Version {
		scaleLatest[t.Source] = t
	}
}

// Scale returns the table of the source and the version.
func Scale(source SourceID, version uint16) (*ScaleTable, error) {
	scaleMu.RLock()
	defer scaleMu.RUnlock()
	v, ok := scaleTables[source][version]
	if !ok {
		return nil, xerrors.NewfWithStack("radar: unknown scale: %s version %d", source.String(), version)
	}
	return v, nil
}

// CurrentScale returns the table that the parser of the source writes.
func CurrentScale(source SourceID) (*ScaleTable, error) {
	scaleMu.RLock()
	defer scaleMu.RUnlock()
	v, ok := scaleLatest[source]
	if !ok {
		return nil, xerrors.NewfWithStack("radar: no scale is registered for %s", source.String())
	}
	return v, nil
}

// Rainfall returns the range of the rainfall intensity of the band in mm/h.
// Both of the returned values are NaN when idx is BandOutOfRange.
func (s *ScaleTable) Rainfall(idx uint8) (minMMH, maxMMH float64) {
	if int(idx) >= len(s.Bands) {
		return math.NaN(), math.NaN()
	}
	b := s.Bands[idx]
	return b.MinMMH, b.MaxMMH
}

// Band returns the band of the index.
func (s *ScaleTable) Band(idx uint8) (Band, error) {
	if int(idx) >= len(s.Bands) {
		return Band{}, xerrors.WithMessagef(ErrUnknownBand, "%d", idx)
	}
	return s.Bands[idx], nil
}

// IndexOf returns the index of the band of the range [minMMH, maxMMH).
// It is used by the parser to convert a class of the source into the index that is stored.
func (s *ScaleTable) IndexOf(minMMH, maxMMH float64) (uint8, error) {
	for _, b := range s.Bands {
		if b.Index == BandOutOfRange {
			continue
		}
		if b.MinMMH == minMMH && b.MaxMMH == maxMMH {
			return b.Index, nil
		}
	}
	return 0, xerrors.WithMessagef(ErrUnknownBand, "%s: [%g, %g)", s.Source.String(), minMMH, maxMMH)
}

// NewScaleTable builds a table from the upper bounds of the classes in ascending order.
//
// The band 0 is always BandOutOfRange. The band 1 starts at 0 and the last band has no upper
// bound. Passing [1, 5, 10] makes [0,1), [1,5), [5,10) and [10, inf).
func NewScaleTable(source SourceID, version uint16, upperBounds []float64) *ScaleTable {
	bands := make([]Band, 0, len(upperBounds)+2)
	bands = append(bands, Band{Index: BandOutOfRange, MinMMH: math.NaN(), MaxMMH: math.NaN()})
	min := float64(0)
	for i, upper := range upperBounds {
		bands = append(bands, Band{Index: uint8(i + 1), MinMMH: min, MaxMMH: upper})
		min = upper
	}
	bands = append(bands, Band{Index: uint8(len(upperBounds) + 1), MinMMH: min, MaxMMH: math.Inf(1)})
	return &ScaleTable{Source: source, Version: version, Bands: bands}
}
