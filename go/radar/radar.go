// Package radar provides the common vocabulary of the radar rainfall archive system.
//
// Every module of the system (fetch, parse, dayfile, wal) talks in the types defined here.
// The source specific differences of the image format, the way of dividing the space and the
// rainfall classes are absorbed by the parser and never leak into this package.
package radar

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"go.f110.dev/xerrors"
)

// TileSize is the number of cells on a side of a tile of the common mesh.
// It is common to all sources because the parser normalizes every source into the common mesh.
const TileSize = 256

// MaxLatitude is the latitude limit of Web Mercator.
const MaxLatitude = 85.05112878

// JST is the time zone that the daily boundary is based on.
// The location is fixed here instead of using time.LoadLocation to avoid depending on the tzdata
// of the runtime environment.
var JST = time.FixedZone("JST", 9*60*60)

var (
	ErrInvalidTile   = xerrors.Define("radar: invalid tile")
	ErrInvalidArea   = xerrors.Define("radar: invalid area")
	ErrUnknownSource = xerrors.Define("radar: unknown source")
)

// SourceID identifies a data source. It is recorded in the object key and the file header.
type SourceID uint16

const (
	SourceXRAIN SourceID = 1
	SourceAmesh SourceID = 2
)

var sourceNames = map[SourceID]string{
	SourceXRAIN: "xrain",
	SourceAmesh: "amesh",
}

func (s SourceID) String() string {
	if v, ok := sourceNames[s]; ok {
		return v
	}
	return "source(" + strconv.Itoa(int(s)) + ")"
}

// ParseSourceID converts the name of the source to SourceID.
func ParseSourceID(s string) (SourceID, error) {
	for k, v := range sourceNames {
		if v == s {
			return k, nil
		}
	}
	return 0, xerrors.WithMessagef(ErrUnknownSource, "%s", s)
}

// Tile is a tile of the common mesh. It is also the unit of collecting and storing.
//
// The common mesh is the XYZ tile of Web Mercator.
// Z is the zoom level. The whole surface is divided into 2^Z x 2^Z tiles.
// X is the tile number of the longitude direction. 0 is the west end.
// Y is the tile number of the latitude direction. 0 is the north end.
type Tile struct {
	Z, X, Y int
}

func (t Tile) String() string {
	return fmt.Sprintf("%d/%d/%d", t.Z, t.X, t.Y)
}

// ParseTile parses the string representation of Tile. e.g. "10/905/402"
func ParseTile(s string) (Tile, error) {
	e := strings.Split(s, "/")
	if len(e) != 3 {
		return Tile{}, xerrors.WithMessagef(ErrInvalidTile, "%s", s)
	}
	v := make([]int, 3)
	for i := range e {
		n, err := strconv.Atoi(e[i])
		if err != nil {
			return Tile{}, xerrors.WithMessagef(ErrInvalidTile, "%s", s)
		}
		v[i] = n
	}
	t := Tile{Z: v[0], X: v[1], Y: v[2]}
	if !t.Valid() {
		return Tile{}, xerrors.WithMessagef(ErrInvalidTile, "%s", s)
	}
	return t, nil
}

// Valid reports whether the tile is within the range of the zoom level.
func (t Tile) Valid() bool {
	if t.Z < 0 || t.Z > 24 {
		return false
	}
	n := 1 << uint(t.Z)
	return t.X >= 0 && t.X < n && t.Y >= 0 && t.Y < n
}

// Cell returns the geographic coordinate of the center of the cell at (px, py).
func (t Tile) Cell(px, py int) (lat, lon float64) {
	n := float64(uint64(1)<<uint(t.Z)) * TileSize
	gx := float64(t.X*TileSize+px) + 0.5
	gy := float64(t.Y*TileSize+py) + 0.5
	lon = gx/n*360 - 180
	lat = math.Atan(math.Sinh(math.Pi*(1-2*gy/n))) * 180 / math.Pi
	return
}

// Bounds returns the geographic range that the tile covers.
func (t Tile) Bounds() Area {
	n := float64(uint64(1) << uint(t.Z))
	west := float64(t.X)/n*360 - 180
	east := float64(t.X+1)/n*360 - 180
	north := math.Atan(math.Sinh(math.Pi*(1-2*float64(t.Y)/n))) * 180 / math.Pi
	south := math.Atan(math.Sinh(math.Pi*(1-2*float64(t.Y+1)/n))) * 180 / math.Pi
	return Area{MinLat: south, MinLon: west, MaxLat: north, MaxLon: east}
}

// TileOf returns the tile that contains the coordinate and the cell position in the tile.
func TileOf(z int, lat, lon float64) (Tile, int, int) {
	gx, gy := cellCoord(z, lat, lon)
	return Tile{Z: z, X: gx / TileSize, Y: gy / TileSize}, gx % TileSize, gy % TileSize
}

// cellCoord returns the cell position in the global cell coordinate of the zoom level.
func cellCoord(z int, lat, lon float64) (int, int) {
	n := float64(uint64(1)<<uint(z)) * TileSize
	maxIndex := int(n) - 1

	if lat > MaxLatitude {
		lat = MaxLatitude
	} else if lat < -MaxLatitude {
		lat = -MaxLatitude
	}
	lon = math.Mod(lon+180, 360)
	if lon < 0 {
		lon += 360
	}

	x := int(lon / 360 * n)
	r := lat * math.Pi / 180
	y := int((1 - math.Log(math.Tan(r)+1/math.Cos(r))/math.Pi) / 2 * n)
	if x < 0 {
		x = 0
	} else if x > maxIndex {
		x = maxIndex
	}
	if y < 0 {
		y = 0
	} else if y > maxIndex {
		y = maxIndex
	}
	return x, y
}

// Area is the geographic range. It is given at runtime as the range to collect.
type Area struct {
	MinLat, MinLon float64
	MaxLat, MaxLon float64
}

// ParseArea parses the string representation of Area. e.g. "35.4,138.9,36.0,140.0"
func ParseArea(s string) (Area, error) {
	e := strings.Split(s, ",")
	if len(e) != 4 {
		return Area{}, xerrors.WithMessagef(ErrInvalidArea, "%s: want minLat,minLon,maxLat,maxLon", s)
	}
	v := make([]float64, 4)
	for i := range e {
		f, err := strconv.ParseFloat(strings.TrimSpace(e[i]), 64)
		if err != nil {
			return Area{}, xerrors.WithMessagef(ErrInvalidArea, "%s", s)
		}
		v[i] = f
	}
	a := Area{MinLat: v[0], MinLon: v[1], MaxLat: v[2], MaxLon: v[3]}
	if a.MinLat >= a.MaxLat || a.MinLon >= a.MaxLon {
		return Area{}, xerrors.WithMessagef(ErrInvalidArea, "%s: min must be less than max", s)
	}
	if a.MinLat < -MaxLatitude || a.MaxLat > MaxLatitude {
		return Area{}, xerrors.WithMessagef(ErrInvalidArea, "%s: latitude is out of range", s)
	}
	if a.MinLon < -180 || a.MaxLon > 180 {
		return Area{}, xerrors.WithMessagef(ErrInvalidArea, "%s: longitude is out of range", s)
	}
	return a, nil
}

func (a Area) String() string {
	return fmt.Sprintf("%g,%g,%g,%g", a.MinLat, a.MinLon, a.MaxLat, a.MaxLon)
}

// Tiles enumerates the tiles of the common mesh that cover the area.
//
// The minimum edges are inclusive and the maximum edges are exclusive when they land exactly on a
// tile boundary. Without that, enumerating the children of a tile would also pick up the
// neighbours that only touch the edge and the parser would produce tiles that the source does not
// cover at all.
func (a Area) Tiles(z int) []Tile {
	// The north west corner has the smallest X and Y because Y is counted from the north.
	x0, y1 := cellCoord(z, a.MinLat, a.MinLon)
	x1, y0 := cellCoord(z, a.MaxLat, a.MaxLon)
	// The cell of the maximum edge belongs to the next tile only when the edge is exactly on the
	// boundary. In that case the area covers nothing of that tile.
	if x1%TileSize == 0 && x1 > x0 {
		x1--
	}
	if y1%TileSize == 0 && y1 > y0 {
		y1--
	}

	minX, maxX := x0/TileSize, x1/TileSize
	minY, maxY := y0/TileSize, y1/TileSize
	tiles := make([]Tile, 0, (maxX-minX+1)*(maxY-minY+1))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			tiles = append(tiles, Tile{Z: z, X: x, Y: y})
		}
	}
	return tiles
}

// Intersects reports whether the two areas overlap.
func (a Area) Intersects(b Area) bool {
	return a.MinLat <= b.MaxLat && b.MinLat <= a.MaxLat && a.MinLon <= b.MaxLon && b.MinLon <= a.MaxLon
}

// Contains reports whether the coordinate is in the area.
func (a Area) Contains(lat, lon float64) bool {
	return lat >= a.MinLat && lat <= a.MaxLat && lon >= a.MinLon && lon <= a.MaxLon
}

// RawObject is the output of the fetcher. The fetcher never interprets the body.
type RawObject struct {
	Source SourceID
	// Unit is the identifier of the fetching unit of the source. It is used for the log and quarantine.
	Unit        string
	ValidTime   time.Time
	ContentType string
	Body        []byte
}

// Frame is the output of the parser and the input of the compressor.
type Frame struct {
	Source       SourceID
	Tile         Tile
	ValidTime    time.Time
	ScaleVersion uint16
	// Values is the band index of each cell. The length is always TileSize*TileSize.
	Values []byte
}

// FrameCount returns the number of the frames in a day of the interval.
func FrameCount(interval time.Duration) int {
	if interval <= 0 {
		return 0
	}
	return int(24 * time.Hour / interval)
}

// DateOf returns the midnight of JST of the day that t belongs to.
func DateOf(t time.Time) time.Time {
	j := t.In(JST)
	return time.Date(j.Year(), j.Month(), j.Day(), 0, 0, 0, 0, JST)
}

// Slot returns the slot number of t in the day of JST.
func Slot(t time.Time, interval time.Duration) int {
	if interval <= 0 {
		return -1
	}
	j := t.In(JST)
	elapsed := j.Sub(DateOf(t))
	return int(elapsed / interval)
}

// SlotTime returns the time of the slot of the date.
func SlotTime(date time.Time, slot int, interval time.Duration) time.Time {
	return DateOf(date).Add(time.Duration(slot) * interval)
}
