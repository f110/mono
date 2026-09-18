package parse

import (
	"image/color"
	"math"
	"testing"

	"go.f110.dev/mono/go/radar"
	"go.f110.dev/mono/go/testing/assertion"
)

func testScale() *radar.ScaleTable {
	return radar.NewScaleTable(radar.SourceAmesh, 1, []float64{0.3, 1, 5, 10, 20, 30, 40, 50, 60, 75})
}

func testPalette(t *testing.T, transparent uint8) *Palette {
	t.Helper()
	p, err := NewPalette(testScale(), transparent, []PaletteEntry{
		{Color: color.RGBA{R: 255, G: 255, B: 255, A: 255}, MinMMH: 0.3, MaxMMH: 1},
		{Color: color.RGBA{R: 204, G: 255, B: 255, A: 255}, MinMMH: 1, MaxMMH: 5},
		{Color: color.RGBA{R: 51, G: 51, B: 255, A: 255}, MinMMH: 10, MaxMMH: 20},
	})
	assertion.MustNoError(t, err)
	return p
}

func TestPaletteIndex(t *testing.T) {
	cases := []struct {
		name        string
		transparent uint8
		color       color.RGBA
		want        uint8
		wantErr     bool
	}{
		{name: "known color", transparent: 1, color: color.RGBA{R: 51, G: 51, B: 255, A: 255}, want: 5},
		{name: "transparent as no rainfall", transparent: 1, color: color.RGBA{}, want: 1},
		{name: "transparent as out of range", transparent: radar.BandOutOfRange, color: color.RGBA{}, want: radar.BandOutOfRange},
		{name: "unknown color", transparent: 1, color: color.RGBA{R: 1, G: 2, B: 3, A: 255}, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := testPalette(t, tc.transparent)
			idx, err := p.Index(tc.color)
			if tc.wantErr {
				assertion.Error(t, err)
				return
			}
			assertion.MustNoError(t, err)
			assertion.Equal(t, idx, tc.want)
		})
	}

	t.Run("the rainfall of the index", func(t *testing.T) {
		p := testPalette(t, 1)
		minMMH, maxMMH := p.Scale().Rainfall(5)
		assertion.Equal(t, minMMH, float64(10))
		assertion.Equal(t, maxMMH, float64(20))
	})
}

func TestNewPaletteRejectsAClassThatStraddlesTheTable(t *testing.T) {
	_, err := NewPalette(testScale(), 1, []PaletteEntry{
		{Color: color.RGBA{R: 1, A: 255}, MinMMH: 10, MaxMMH: 25},
	})
	assertion.Error(t, err)
}

func TestPaletteColor(t *testing.T) {
	cases := []struct {
		name  string
		index uint8
		want  color.RGBA
		found bool
	}{
		{name: "known band", index: 5, want: color.RGBA{R: 51, G: 51, B: 255, A: 255}, found: true},
		{name: "band without a color", index: 11},
	}

	p := testPalette(t, 1)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, ok := p.Color(tc.index)
			assertion.Equal(t, ok, tc.found)
			if tc.found {
				assertion.Equal(t, c, tc.want)
			}
		})
	}
}

func TestGeoreferenceCorners(t *testing.T) {
	cases := []struct {
		name   string
		georef Georeference
	}{
		{name: "bounding box", georef: BoundingBox{Width: 100, Height: 50, Area: radar.Area{MinLat: 35, MinLon: 139, MaxLat: 36, MaxLon: 141}}},
		{name: "mercator box", georef: MercatorBox{Width: 100, Height: 50, Area: radar.Area{MinLat: 35, MinLon: 139, MaxLat: 36, MaxLon: 141}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lat, lon := tc.georef.Coord(0, 0)
			assertion.True(t, lat < 36 && lat > 35.98)
			assertion.True(t, lon > 139 && lon < 139.02)

			lat, lon = tc.georef.Coord(99, 49)
			assertion.True(t, lat > 35 && lat < 35.02)
			assertion.True(t, lon < 141 && lon > 140.98)
		})
	}
}

func TestGeoreferenceLatitudeSpacing(t *testing.T) {
	area := radar.Area{MinLat: 35, MinLon: 139, MaxLat: 36, MaxLon: 141}

	t.Run("bounding box is even", func(t *testing.T) {
		b := BoundingBox{Width: 100, Height: 50, Area: area}
		lat1, _ := b.Coord(0, 10)
		lat2, _ := b.Coord(0, 11)
		lat3, _ := b.Coord(0, 40)
		lat4, _ := b.Coord(0, 41)
		assertion.True(t, math.Abs((lat1-lat2)-(lat3-lat4)) < 1e-9)
	})

	t.Run("mercator box is not even", func(t *testing.T) {
		m := MercatorBox{Width: 100, Height: 50, Area: area}
		lat1, _ := m.Coord(0, 0)
		lat2, _ := m.Coord(0, 1)
		lat3, _ := m.Coord(0, 48)
		lat4, _ := m.Coord(0, 49)
		assertion.True(t, (lat1-lat2) < (lat3-lat4))
	})
}

func TestMappingCoversTheWholeArea(t *testing.T) {
	g := BoundingBox{
		Width:  64,
		Height: 32,
		Area:   radar.Area{MinLat: 35.6, MinLon: 139.6, MaxLat: 35.8, MaxLon: 139.9},
	}
	m := NewMapping(g, 10)
	assertion.NotEmpty(t, m.Tiles())

	values := m.Apply(func(px, py int) uint8 { return uint8(px%10) + 1 })
	assertion.Len(t, values, len(m.Tiles()))

	var inside, outside int
	for i, tile := range m.Tiles() {
		for py := 0; py < radar.TileSize; py++ {
			for px := 0; px < radar.TileSize; px++ {
				lat, lon := tile.Cell(px, py)
				v := values[i][py*radar.TileSize+px]
				if g.Area.Contains(lat, lon) {
					assertion.NotEqual(t, v, radar.BandOutOfRange)
					inside++
				} else {
					assertion.Equal(t, v, radar.BandOutOfRange)
					outside++
				}
			}
		}
	}
	assertion.True(t, inside > 0)
	assertion.True(t, outside > 0)
}

func TestMappingIsNearestNeighbour(t *testing.T) {
	g := BoundingBox{
		Width:  2,
		Height: 2,
		Area:   radar.Area{MinLat: 35.0, MinLon: 139.0, MaxLat: 36.0, MaxLon: 140.0},
	}
	m := NewMapping(g, 9)
	values := m.Apply(func(px, py int) uint8 { return uint8(py*2+px) + 1 })

	seen := make(map[uint8]bool)
	for i, tile := range m.Tiles() {
		for py := 0; py < radar.TileSize; py++ {
			for px := 0; px < radar.TileSize; px++ {
				v := values[i][py*radar.TileSize+px]
				if v == radar.BandOutOfRange {
					continue
				}
				assertion.True(t, v >= 1 && v <= 4)
				seen[v] = true

				lat, lon := tile.Cell(px, py)
				wantX, wantY := 0, 0
				if lon >= 139.5 {
					wantX = 1
				}
				if lat < 35.5 {
					wantY = 1
				}
				assertion.Equal(t, v, uint8(wantY*2+wantX)+1)
			}
		}
	}
	assertion.Len(t, seen, 4)
}

// slowGeoreference is a Georeference that does not know its inverse. NewMapping has to fall back
// to the linear search for it, which is what the Inverter interface exists to avoid.
//
// The methods delegate instead of embedding so that Inverse is not promoted.
type slowGeoreference struct {
	box BoundingBox
}

func (g slowGeoreference) Size() (int, int) { return g.box.Size() }

func (g slowGeoreference) Bounds() radar.Area { return g.box.Bounds() }

func (g slowGeoreference) Coord(px, py int) (float64, float64) { return g.box.Coord(px, py) }

func TestMappingUsesTheInverse(t *testing.T) {
	g := BoundingBox{Width: 8, Height: 8, Area: radar.Area{MinLat: 35.6, MinLon: 139.6, MaxLat: 35.8, MaxLon: 139.9}}
	at := func(px, py int) uint8 { return uint8(py*8+px) + 1 }

	cases := []struct {
		name   string
		georef Georeference
	}{
		{name: "with the inverse", georef: g},
		{name: "with the linear search", georef: slowGeoreference{box: g}},
	}

	var want [][]byte
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := tc.georef.(Inverter)
			assertion.Equal(t, ok, tc.name == "with the inverse")

			got := NewMapping(tc.georef, 12).Apply(at)
			if want == nil {
				want = got
				return
			}
			assertion.Equal(t, got, want)
		})
	}
}

func TestMappingIsStable(t *testing.T) {
	g := BoundingBox{
		Width:  32,
		Height: 16,
		Area:   radar.Area{MinLat: 35.6, MinLon: 139.6, MaxLat: 35.8, MaxLon: 139.9},
	}
	at := func(px, py int) uint8 { return uint8((px*py)%7) + 1 }
	assertion.Equal(t, NewMapping(g, 10).Apply(at), NewMapping(g, 10).Apply(at))
}
