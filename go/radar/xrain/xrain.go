// Package xrain implements the fetcher and the parser of XRAIN of the Ministry of Land,
// Infrastructure, Transport and Tourism.
//
// XRAIN publishes the rainfall of the 250m mesh every minute. It is not delivered as XYZ tiles.
// The area is cut into rectangles of the Japanese standard primary mesh and one PNG is published
// per rectangle. Which rectangles exist is described by a definition file that the site publishes.
package xrain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "image/png"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/radar"
	"go.f110.dev/mono/go/radar/fetch"
	"go.f110.dev/mono/go/radar/parse"
)

const (
	// DefaultBaseURL is the origin that publishes the images.
	DefaultBaseURL = "https://www.river.go.jp"

	// Interval is the publishing interval. It was confirmed against the real endpoint that the
	// image of every minute is published.
	Interval = time.Minute

	// LevelNative is the level whose resolution matches the 250m mesh of XRAIN.
	//
	// The site publishes three levels for the zoom levels of its viewer. The level 1 is halved to
	// the 500m mesh and the level 3 is enlarged to about 60m, so only this one carries the
	// resolution that the source actually observes.
	LevelNative = 2

	// defaultLookback is how many minutes Latest goes back at most.
	defaultLookback = 10
)

var ErrNotConfigured = xerrors.Define("xrain: not configured")

// ScaleVersion1 is the first version of the rainfall classes of XRAIN.
//
// The boundaries are taken from the legend of the viewer, which labels the upper bound of each
// class. The lowest class only has an upper bound of 1 mm/h; the legend does not state the
// detection threshold, so the class is recorded as [0, 1) which means "less than 1 mm/h".
const ScaleVersion1 uint16 = 1

// upperBounds are the upper bound of each class in ascending order.
var upperBounds = []float64{1, 5, 10, 20, 30, 50, 80}

var scale = radar.NewScaleTable(radar.SourceXRAIN, ScaleVersion1, upperBounds)

func init() {
	radar.RegisterScale(scale)
}

// paletteEntries is the palette of the rainfall layer.
// The colors are the radarLevel1 to radarLevel8 of the viewer, in ascending order of the intensity.
var paletteEntries = []parse.PaletteEntry{
	{Color: color.RGBA{R: 0x99, G: 0xff, B: 0xff, A: 255}, MinMMH: 0, MaxMMH: 1},
	{Color: color.RGBA{R: 0x66, G: 0xcc, B: 0xff, A: 255}, MinMMH: 1, MaxMMH: 5},
	{Color: color.RGBA{R: 0x21, G: 0x98, B: 0xff, A: 255}, MinMMH: 5, MaxMMH: 10},
	{Color: color.RGBA{R: 0x00, G: 0x38, B: 0xff, A: 255}, MinMMH: 10, MaxMMH: 20},
	{Color: color.RGBA{R: 0xfa, G: 0xf5, B: 0x00, A: 255}, MinMMH: 20, MaxMMH: 30},
	{Color: color.RGBA{R: 0xff, G: 0x99, B: 0x00, A: 255}, MinMMH: 30, MaxMMH: 50},
	{Color: color.RGBA{R: 0xe7, G: 0x28, B: 0x00, A: 255}, MinMMH: 50, MaxMMH: 80},
	{Color: color.RGBA{R: 0x9a, G: 0x00, B: 0x79, A: 255}, MinMMH: 80, MaxMMH: math.Inf(1)},
}

// NewPalette returns the palette of the rainfall layer.
//
// A transparent pixel maps to radar.BandOutOfRange. The image does not distinguish the area of no
// rainfall from the area that the radar does not reach, so claiming 0 mm/h for it would be a lie.
func NewPalette() (*parse.Palette, error) {
	return parse.NewPalette(scale, radar.BandOutOfRange, paletteEntries)
}

// AreaMesh is one rectangle of the delivery. One PNG is published per AreaMesh.
//
// MeshFrom and MeshTo are the Japanese standard primary mesh codes of the south west corner and
// the north east corner.
type AreaMesh struct {
	Seq      int `json:"seq"`
	AreaCd   int `json:"areaCd"`
	MeshFrom int `json:"meshFrom"`
	MeshTo   int `json:"meshTo"`
}

// Unit returns the identifier of the fetching unit. e.g. "82/4"
func (a AreaMesh) Unit() string {
	return strconv.Itoa(a.AreaCd) + "/" + strconv.Itoa(a.Seq)
}

// Bounds returns the geographic range that the image of the rectangle covers.
func (a AreaMesh) Bounds() (radar.Area, error) {
	from, err := primaryMeshBounds(a.MeshFrom)
	if err != nil {
		return radar.Area{}, err
	}
	to, err := primaryMeshBounds(a.MeshTo)
	if err != nil {
		return radar.Area{}, err
	}
	return radar.Area{
		MinLat: math.Min(from.MinLat, to.MinLat),
		MinLon: math.Min(from.MinLon, to.MinLon),
		MaxLat: math.Max(from.MaxLat, to.MaxLat),
		MaxLon: math.Max(from.MaxLon, to.MaxLon),
	}, nil
}

// primaryMeshBounds returns the range of a primary mesh of the Japanese standard grid.
//
// The code is four digits. The first two are the latitude multiplied by 1.5 and the last two are
// the longitude minus 100. One cell spans 40 minutes of latitude and one degree of longitude.
// The grid is defined on the latitude and the longitude, so an image that is aligned to it is in
// the equirectangular projection.
func primaryMeshBounds(code int) (radar.Area, error) {
	if code < 1000 || code > 9999 {
		return radar.Area{}, xerrors.NewfWithStack("xrain: invalid primary mesh code: %d", code)
	}
	south := float64(code/100) / 1.5
	west := float64(code%100) + 100
	return radar.Area{MinLat: south, MinLon: west, MaxLat: south + 2.0/3.0, MaxLon: west + 1}, nil
}

// MeshDefinition is the definition file of a level.
//
// It is published at {baseURL}/kawabou/file/files/master/radar/xrd/{level}.json
type MeshDefinition struct {
	Type  string     `json:"type"`
	Level int        `json:"level"`
	Areas []AreaMesh `json:"areaInfo"`
}

// DefinitionURL returns the URL of the definition file of the level.
func DefinitionURL(baseURL string, level int) string {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return fmt.Sprintf("%s/kawabou/file/files/master/radar/xrd/%d.json", baseURL, level)
}

// ParseMeshDefinition reads the definition file.
func ParseMeshDefinition(body []byte) (*MeshDefinition, error) {
	var d MeshDefinition
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, xerrors.WithStack(err)
	}
	if len(d.Areas) == 0 {
		return nil, xerrors.WithMessage(ErrNotConfigured, "the definition does not have any area")
	}
	// Every rectangle is validated here so that a mesh code that the site changed makes the process
	// fail at startup. Skipping it later would drop that rectangle from the collection silently and
	// the hole would only be noticed days after.
	for _, a := range d.Areas {
		if _, err := a.Bounds(); err != nil {
			return nil, err
		}
	}
	return &d, nil
}

// Find returns the rectangle of the unit.
func (d *MeshDefinition) Find(unit string) (AreaMesh, error) {
	areaCd, seq, err := parseUnit(unit)
	if err != nil {
		return AreaMesh{}, err
	}
	for _, a := range d.Areas {
		if a.AreaCd == areaCd && a.Seq == seq {
			return a, nil
		}
	}
	return AreaMesh{}, xerrors.NewfWithStack("xrain: unknown unit: %s", unit)
}

func parseUnit(unit string) (int, int, error) {
	e := strings.Split(unit, "/")
	if len(e) != 2 {
		return 0, 0, xerrors.NewfWithStack("xrain: invalid unit: %s", unit)
	}
	areaCd, err := strconv.Atoi(e[0])
	if err != nil {
		return 0, 0, xerrors.NewfWithStack("xrain: invalid unit: %s", unit)
	}
	seq, err := strconv.Atoi(e[1])
	if err != nil {
		return 0, 0, xerrors.NewfWithStack("xrain: invalid unit: %s", unit)
	}
	return areaCd, seq, nil
}

// Fetcher fetches the rainfall images of XRAIN.
type Fetcher struct {
	baseURL  string
	def      *MeshDefinition
	lookback int
	client   *fetch.Client
}

// NewFetcher returns a Fetcher.
func NewFetcher(baseURL string, def *MeshDefinition, client *fetch.Client) (*Fetcher, error) {
	if def == nil {
		return nil, xerrors.WithMessage(ErrNotConfigured, "the mesh definition is required")
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Fetcher{baseURL: baseURL, def: def, lookback: defaultLookback, client: client}, nil
}

func (f *Fetcher) ID() radar.SourceID { return radar.SourceXRAIN }

func (f *Fetcher) Interval() time.Duration { return Interval }

// Units returns the rectangles that overlap the area.
func (f *Fetcher) Units(area radar.Area) []string {
	// ParseMeshDefinition already rejected a rectangle whose mesh code is invalid, so the error
	// here cannot happen.
	overlapped := enumerable.FindAll(f.def.Areas, func(a AreaMesh) bool {
		b, err := a.Bounds()
		if err != nil {
			return false
		}
		return b.Intersects(area)
	})
	return enumerable.Map(overlapped, AreaMesh.Unit)
}

// URL returns the URL of the image of the time and the unit.
//
//	{baseURL}/kawabou/file/radar/image/xrd/{areaCd}/{level}/{YYYYMMDD}/{HHmm}/{seq}.png
func (f *Fetcher) URL(t time.Time, unit string) (string, error) {
	areaCd, seq, err := parseUnit(unit)
	if err != nil {
		return "", err
	}
	jst := t.In(radar.JST)
	return fmt.Sprintf("%s/kawabou/file/radar/image/xrd/%d/%d/%s/%s/%d.png",
		f.baseURL, areaCd, f.def.Level, jst.Format("20060102"), jst.Format("1504"), seq), nil
}

// Latest checks the existence of the recent minutes for one of the rectangles.
func (f *Fetcher) Latest(ctx context.Context, since time.Time) ([]time.Time, error) {
	if len(f.def.Areas) == 0 {
		return nil, nil
	}
	// One rectangle is enough to know whether the frame of the time is published.
	probe := f.def.Areas[0].Unit()

	var times []time.Time
	for _, t := range f.candidates(time.Now().Truncate(Interval), since) {
		u, err := f.URL(t, probe)
		if err != nil {
			return nil, err
		}
		ok, err := f.client.Exists(ctx, u)
		if err != nil {
			return nil, err
		}
		if ok {
			times = append(times, t)
		}
	}
	return times, nil
}

// candidates returns the times to check, in ascending order so that the caller processes the older
// one first.
func (f *Fetcher) candidates(now, since time.Time) []time.Time {
	all := make([]time.Time, f.lookback)
	for i := range all {
		all[i] = now.Add(-time.Duration(i) * Interval)
	}
	candidates := enumerable.FindAll(all, func(t time.Time) bool { return t.After(since) })
	slices.Reverse(candidates)
	return candidates
}

func (f *Fetcher) Fetch(ctx context.Context, t time.Time, unit string) (*radar.RawObject, error) {
	u, err := f.URL(t, unit)
	if err != nil {
		return nil, err
	}
	res, err := f.client.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	return &radar.RawObject{
		Source:      radar.SourceXRAIN,
		Unit:        unit,
		ValidTime:   t.Truncate(Interval),
		ContentType: res.ContentType,
		Body:        res.Body,
	}, nil
}

// Parser converts an image of XRAIN into the rainfall on the common mesh.
// Parser is safe for concurrent use.
type Parser struct {
	palette *parse.Palette
	def     *MeshDefinition
	zoom    int

	// mu guards the caches below. Parse is called from one goroutine per unit, so the maps would
	// be a data race without it.
	mu sync.Mutex
	// mappings caches the mapping per unit. The mapping only depends on the rectangle, the size of
	// the image and the zoom level, so it is reused across the frames.
	mappings map[string]*parse.Mapping
	sizes    map[string]image.Point
}

// NewParser returns a Parser.
func NewParser(def *MeshDefinition, zoom int) (*Parser, error) {
	if def == nil {
		return nil, xerrors.WithMessage(ErrNotConfigured, "the mesh definition is required")
	}
	if zoom <= 0 {
		return nil, xerrors.WithMessage(ErrNotConfigured, "the zoom level is required")
	}
	palette, err := NewPalette()
	if err != nil {
		return nil, err
	}
	return &Parser{
		palette:  palette,
		def:      def,
		zoom:     zoom,
		mappings: make(map[string]*parse.Mapping),
		sizes:    make(map[string]image.Point),
	}, nil
}

func (p *Parser) ID() radar.SourceID { return radar.SourceXRAIN }

// Parse decodes the image and maps it onto the common mesh.
//
// The pixel grid of the image is aligned to the primary mesh, which is defined on the latitude and
// the longitude, so the georeference is the equirectangular projection. The number of the pixels
// per primary mesh differs by the level, so the size is taken from the decoded image instead of
// being a constant.
func (p *Parser) Parse(raw *radar.RawObject) ([]*radar.Frame, error) {
	area, err := p.def.Find(raw.Unit)
	if err != nil {
		return nil, err
	}
	bounds, err := area.Bounds()
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(raw.Body))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, xerrors.NewfWithStack("xrain: %s: empty image", raw.Unit)
	}

	values := make([]uint8, w*h)
	for py := 0; py < h; py++ {
		for px := 0; px < w; px++ {
			c := color.RGBAModel.Convert(img.At(b.Min.X+px, b.Min.Y+py)).(color.RGBA)
			idx, err := p.palette.Index(c)
			if err != nil {
				return nil, err
			}
			values[py*w+px] = idx
		}
	}

	m, err := p.mapping(raw.Unit, w, h, bounds)
	if err != nil {
		return nil, err
	}

	tiles := m.Tiles()
	mapped := m.Apply(func(px, py int) uint8 { return values[py*w+px] })
	frames := make([]*radar.Frame, len(tiles))
	for i := range tiles {
		frames[i] = &radar.Frame{
			Source:       radar.SourceXRAIN,
			Tile:         tiles[i],
			ValidTime:    raw.ValidTime,
			ScaleVersion: scale.Version,
			Values:       mapped[i],
		}
	}
	return frames, nil
}

// mapping returns the cached mapping of the unit, building it on the first call.
//
// The size of a rectangle does not change while the level is the same. A change means that the
// source changed the layout, and mapping it with the old correspondence would silently shift every
// cell.
func (p *Parser) mapping(unit string, w, h int, bounds radar.Area) (*parse.Mapping, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if size, ok := p.sizes[unit]; ok {
		if size.X != w || size.Y != h {
			return nil, xerrors.NewfWithStack("xrain: %s: unexpected image size: %dx%d != %dx%d", unit, w, h, size.X, size.Y)
		}
		return p.mappings[unit], nil
	}

	m := parse.NewMapping(parse.BoundingBox{Width: w, Height: h, Area: bounds}, p.zoom)
	p.mappings[unit] = m
	p.sizes[unit] = image.Point{X: w, Y: h}
	return m, nil
}

// Palette returns the palette. It is used to draw a frame as an image again.
func (p *Parser) Palette() *parse.Palette { return p.palette }
