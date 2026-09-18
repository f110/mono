// Package parse provides the helpers that convert a source image into the rainfall on the common
// mesh.
//
// A parser does three things. It decodes the image, maps the pixels onto the cells of the common
// mesh through the georeference of the source and converts the classes of the source into the band
// indexes of the scale table. After these three, no difference of the source remains.
package parse

import (
	"fmt"
	"image/color"
	"log/slog"
	"math"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/radar"
)

var (
	// ErrUnknownColor means that a color that is not in the palette appeared.
	// The frame must not be taken in. The raw data is moved to the quarantine and the alert is
	// raised. Rounding to the nearest color silently would break the reversibility and the change
	// of the source would never be noticed.
	ErrUnknownColor = xerrors.Define("parse: unknown color")

	// ErrTranslucentPixel means that a partly transparent pixel appeared.
	//
	// It is reported apart from ErrUnknownColor because the cause is different. A translucent pixel
	// is usually the antialiasing of an edge or a change of how the source draws, not a class that
	// the palette is missing. Whoever digs into the quarantine can tell the two apart from the
	// error alone.
	ErrTranslucentPixel = xerrors.Define("parse: translucent pixel")
)

// PaletteEntry is a color of the source and the rainfall class it stands for.
type PaletteEntry struct {
	Color color.RGBA
	// MinMMH and MaxMMH are the range of the rainfall intensity that the color stands for.
	MinMMH float64
	MaxMMH float64
}

// Palette converts a color of a source into the index of the scale table of the source.
type Palette struct {
	scale       *radar.ScaleTable
	transparent uint8
	index       map[uint32]uint8
	// colors is the reverse lookup. It is built once so that Color is deterministic and O(1).
	colors map[uint8]color.RGBA
}

// NewPalette builds a palette. Every entry has to fit into one class of the scale table.
//
// transparent is the band index of a fully transparent pixel. It differs per source. A source that
// draws the area of no rainfall as transparent maps it to the band of no rainfall, and a source
// that draws the area it does not observe as transparent maps it to radar.BandOutOfRange.
// The entries are also checked for duplication. The palette of a source that is read from a file
// is external input, so a color that appears twice or a band that two colors map to has to be
// rejected instead of letting the last one win.
func NewPalette(scale *radar.ScaleTable, transparent uint8, entries []PaletteEntry) (*Palette, error) {
	p := &Palette{
		scale:       scale,
		transparent: transparent,
		index:       make(map[uint32]uint8, len(entries)),
		colors:      make(map[uint8]color.RGBA, len(entries)),
	}
	for _, e := range entries {
		idx, err := scale.IndexOf(e.MinMMH, e.MaxMMH)
		if err != nil {
			return nil, err
		}
		packed := packColor(e.Color)
		if _, ok := p.index[packed]; ok {
			return nil, xerrors.NewfWithStack("parse: the color appears twice: R=%d G=%d B=%d", e.Color.R, e.Color.G, e.Color.B)
		}
		if _, ok := p.colors[idx]; ok {
			return nil, xerrors.NewfWithStack("parse: two colors map to the band %d", idx)
		}
		p.index[packed] = idx
		p.colors[idx] = e.Color
	}
	return p, nil
}

// Index returns the index of the scale table of the color.
//
// color.RGBA is premultiplied by the alpha, so a translucent pixel never matches an entry of the
// palette. It is rejected with ErrTranslucentPixel rather than being reported as an unknown color.
func (p *Palette) Index(c color.RGBA) (uint8, error) {
	switch c.A {
	case 0:
		return p.transparent, nil
	case 0xff:
		idx, ok := p.index[packColor(c)]
		if !ok {
			return 0, xerrors.WithMessagef(ErrUnknownColor, "R=%d G=%d B=%d", c.R, c.G, c.B)
		}
		return idx, nil
	default:
		return 0, xerrors.WithMessagef(ErrTranslucentPixel, "R=%d G=%d B=%d A=%d", c.R, c.G, c.B, c.A)
	}
}

// Color returns the color of the index. It is used to draw the frame as an image again.
func (p *Palette) Color(idx uint8) (color.RGBA, bool) {
	c, ok := p.colors[idx]
	return c, ok
}

func (p *Palette) Scale() *radar.ScaleTable {
	return p.scale
}

func packColor(c color.RGBA) uint32 {
	return uint32(c.R)<<24 | uint32(c.G)<<16 | uint32(c.B)<<8 | uint32(c.A)
}

// Georeference maps a pixel of the source image to a geographic coordinate.
type Georeference interface {
	// Size returns the size of the source image in pixels.
	Size() (w, h int)
	// Coord returns the coordinate of the center of the pixel.
	Coord(px, py int) (lat, lon float64)
	// Bounds returns the geographic range that the image covers.
	Bounds() radar.Area
}

// Inverse maps a coordinate back to a pixel of the source image.
type Inverse func(lat, lon float64) (px, py int, ok bool)

// Inverter is implemented by a Georeference that knows its own inverse.
//
// NewMapping falls back to a linear search over every pixel without it, which runs inside the loop
// over every cell of every tile. That is the size of the image multiplied by the number of the
// cells, so a Georeference that does not implement this is unusable at a real size. Implement it.
//
// Inverter returns a function instead of taking the coordinate directly so that the implementation
// can compute whatever is constant for the image once, outside of the loop.
type Inverter interface {
	Inverse() Inverse
}

// BoundingBox is a Georeference of an image whose pixels are evenly spaced in latitude and
// longitude, which is the equirectangular projection.
type BoundingBox struct {
	Width, Height int
	Area          radar.Area
}

func (b BoundingBox) Size() (int, int) { return b.Width, b.Height }

func (b BoundingBox) Bounds() radar.Area { return b.Area }

func (b BoundingBox) Coord(px, py int) (float64, float64) {
	lon := b.Area.MinLon + (float64(px)+0.5)/float64(b.Width)*(b.Area.MaxLon-b.Area.MinLon)
	// The pixel of y=0 is the north end of the image.
	lat := b.Area.MaxLat - (float64(py)+0.5)/float64(b.Height)*(b.Area.MaxLat-b.Area.MinLat)
	return lat, lon
}

func (b BoundingBox) Inverse() Inverse {
	lonSpan := b.Area.MaxLon - b.Area.MinLon
	latSpan := b.Area.MaxLat - b.Area.MinLat
	return func(lat, lon float64) (int, int, bool) {
		if !b.Area.Contains(lat, lon) {
			return 0, 0, false
		}
		px := int((lon - b.Area.MinLon) / lonSpan * float64(b.Width))
		py := int((b.Area.MaxLat - lat) / latSpan * float64(b.Height))
		return clampPixel(px, py, b.Width, b.Height)
	}
}

// MercatorBox is a Georeference of an image that is drawn in Web Mercator.
// The pixels are evenly spaced in the projected coordinate, not in the latitude.
type MercatorBox struct {
	Width, Height int
	Area          radar.Area
}

func (m MercatorBox) Size() (int, int) { return m.Width, m.Height }

func (m MercatorBox) Bounds() radar.Area { return m.Area }

func (m MercatorBox) Coord(px, py int) (float64, float64) {
	north, south := mercatorY(m.Area.MaxLat), mercatorY(m.Area.MinLat)
	lon := m.Area.MinLon + (float64(px)+0.5)/float64(m.Width)*(m.Area.MaxLon-m.Area.MinLon)
	y := north + (float64(py)+0.5)/float64(m.Height)*(south-north)
	return inverseMercatorY(y), lon
}

func (m MercatorBox) Inverse() Inverse {
	// The projection of the two edges is constant for the image. Computing it here keeps the
	// logarithm out of the loop that runs once per cell of every tile.
	north, south := mercatorY(m.Area.MaxLat), mercatorY(m.Area.MinLat)
	lonSpan := m.Area.MaxLon - m.Area.MinLon
	return func(lat, lon float64) (int, int, bool) {
		if !m.Area.Contains(lat, lon) {
			return 0, 0, false
		}
		px := int((lon - m.Area.MinLon) / lonSpan * float64(m.Width))
		py := int((mercatorY(lat) - north) / (south - north) * float64(m.Height))
		return clampPixel(px, py, m.Width, m.Height)
	}
}

// mercatorY projects the latitude into the normalized Web Mercator Y in [0, 1].
func mercatorY(lat float64) float64 {
	if lat > radar.MaxLatitude {
		lat = radar.MaxLatitude
	} else if lat < -radar.MaxLatitude {
		lat = -radar.MaxLatitude
	}
	r := lat * math.Pi / 180
	return (1 - math.Log(math.Tan(r)+1/math.Cos(r))/math.Pi) / 2
}

func inverseMercatorY(y float64) float64 {
	return math.Atan(math.Sinh(math.Pi*(1-2*y))) * 180 / math.Pi
}

// Mapping is the correspondence from the cells of the common mesh to the pixels of a source image.
//
// The mapping is built by the inverse nearest neighbour. For every cell of the covered tiles the
// nearest source pixel is looked up. Doing it in this direction guarantees that no cell is left
// empty even when the source image is coarser than the common mesh.
//
// Nothing is interpolated. The value of a cell is a rainfall class and the average of the
// neighbours has no meaning.
//
// The mapping only depends on the georeference and the zoom level, so it is built once and reused.
type Mapping struct {
	zoom  int
	width int
	tiles []radar.Tile
	// pixels[i] holds the source pixel index of every cell of tiles[i]. -1 means out of coverage.
	pixels [][]int32
}

// NewMapping builds the mapping of the georeference at the zoom level.
func NewMapping(g Georeference, zoom int) *Mapping {
	w, h := g.Size()
	area := g.Bounds()
	tiles := area.Tiles(zoom)

	var pixel Inverse
	if inv, ok := g.(Inverter); ok {
		pixel = inv.Inverse()
	} else {
		pixel = func(lat, lon float64) (int, int, bool) { return nearestPixel(g, w, h, lat, lon) }
		if slogger.Enabled() {
			slogger.Log.Warn("parse: the georeference does not implement Inverter. Building the mapping falls back to a linear search",
				slog.String("georeference", fmt.Sprintf("%T", g)))
		}
	}

	m := &Mapping{zoom: zoom, width: w, tiles: tiles, pixels: make([][]int32, len(tiles))}
	for i, tile := range tiles {
		cells := make([]int32, radar.TileSize*radar.TileSize)
		for py := 0; py < radar.TileSize; py++ {
			for px := 0; px < radar.TileSize; px++ {
				lat, lon := tile.Cell(px, py)
				sx, sy, ok := pixel(lat, lon)
				if !ok {
					cells[py*radar.TileSize+px] = -1
					continue
				}
				cells[py*radar.TileSize+px] = int32(sy*w + sx)
			}
		}
		m.pixels[i] = cells
	}
	return m
}

// nearestPixel returns the pixel of the source image that is nearest to the coordinate.
func nearestPixel(g Georeference, w, h int, lat, lon float64) (int, int, bool) {
	if !g.Bounds().Contains(lat, lon) {
		return 0, 0, false
	}

	// A georeference that does not know its inverse falls back to a linear search over the whole
	// image. See the comment on Inverter for why that is not viable at a real size.
	best, bx, by := math.Inf(1), -1, -1
	for py := 0; py < h; py++ {
		for px := 0; px < w; px++ {
			plat, plon := g.Coord(px, py)
			d := (plat-lat)*(plat-lat) + (plon-lon)*(plon-lon)
			if d < best {
				best, bx, by = d, px, py
			}
		}
	}
	if bx < 0 {
		return 0, 0, false
	}
	return bx, by, true
}

func clampPixel(px, py, w, h int) (int, int, bool) {
	if px < 0 {
		px = 0
	} else if px >= w {
		px = w - 1
	}
	if py < 0 {
		py = 0
	} else if py >= h {
		py = h - 1
	}
	return px, py, true
}

// Tiles returns the tiles of the common mesh that the mapping covers.
func (m *Mapping) Tiles() []radar.Tile {
	return m.tiles
}

// Apply builds the values of every tile from the source image.
//
// at returns the band index of a pixel of the source image. The cells that the source does not
// cover are filled with radar.BandOutOfRange. The result is in the same order as Tiles.
func (m *Mapping) Apply(at func(px, py int) uint8) [][]byte {
	out := make([][]byte, len(m.tiles))
	for i, cells := range m.pixels {
		values := make([]byte, radar.TileSize*radar.TileSize)
		for j, p := range cells {
			if p < 0 {
				values[j] = radar.BandOutOfRange
				continue
			}
			values[j] = at(int(p)%m.width, int(p)/m.width)
		}
		out[i] = values
	}
	return out
}
