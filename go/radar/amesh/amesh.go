// Package amesh implements the fetcher and the parser of Tokyo Amesh.
//
// Tokyo Amesh publishes a single image of a fixed area every minute. The rainfall layer and the
// map layer are published as separate images and only the rainfall layer is fetched. Taking the
// image that has the map burned in would break the conversion of the classes.
package amesh

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"math"
	"slices"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/radar"
	"go.f110.dev/mono/go/radar/fetch"
	"go.f110.dev/mono/go/radar/parse"
)

const (
	// DefaultBaseURL is the origin that publishes the images.
	DefaultBaseURL = "https://tokyo-ame.jwa.or.jp"

	// DefaultUnit is the area code of the widest image.
	DefaultUnit = "000"

	// Interval is the publishing interval. It was confirmed against the real endpoint that the
	// image of every minute is published.
	Interval = time.Minute

	// ImageWidth and ImageHeight are the size of the image of DefaultUnit.
	ImageWidth  = 770
	ImageHeight = 480

	// defaultLookback is how many minutes Latest goes back at most.
	defaultLookback = 10
)

// ScaleVersion1 is the first version of the rainfall classes of Tokyo Amesh.
//
// The boundaries are taken from the legend that the site publishes (ja/images/hanrei.gif). The
// legend shows the classes as integer ranges such as "1〜4", which means the continuous [1, 5).
const ScaleVersion1 uint16 = 1

// upperBounds are the upper bound of each class in ascending order.
var upperBounds = []float64{0.3, 1, 5, 10, 20, 30, 40, 50, 60, 75}

var scale = radar.NewScaleTable(radar.SourceAmesh, ScaleVersion1, upperBounds)

func init() {
	radar.RegisterScale(scale)
}

// paletteEntries is the palette of the rainfall layer.
//
// Each color stands for one class of the legend. The band 1 of the table, which is below the
// weakest class of the legend, is what a transparent pixel maps to.
var paletteEntries = []parse.PaletteEntry{
	{Color: color.RGBA{R: 255, G: 255, B: 255, A: 255}, MinMMH: 0.3, MaxMMH: 1},
	{Color: color.RGBA{R: 204, G: 255, B: 255, A: 255}, MinMMH: 1, MaxMMH: 5},
	{Color: color.RGBA{R: 102, G: 153, B: 255, A: 255}, MinMMH: 5, MaxMMH: 10},
	{Color: color.RGBA{R: 51, G: 51, B: 255, A: 255}, MinMMH: 10, MaxMMH: 20},
	{Color: color.RGBA{R: 0, G: 255, B: 0, A: 255}, MinMMH: 20, MaxMMH: 30},
	{Color: color.RGBA{R: 255, G: 255, B: 0, A: 255}, MinMMH: 30, MaxMMH: 40},
	{Color: color.RGBA{R: 255, G: 153, B: 0, A: 255}, MinMMH: 40, MaxMMH: 50},
	{Color: color.RGBA{R: 255, G: 0, B: 0, A: 255}, MinMMH: 50, MaxMMH: 60},
	{Color: color.RGBA{R: 255, G: 0, B: 255, A: 255}, MinMMH: 60, MaxMMH: 75},
	{Color: color.RGBA{R: 112, G: 48, B: 160, A: 255}, MinMMH: 75, MaxMMH: math.Inf(1)},
}

// noRainBand is the band of a transparent pixel inside the image.
//
// The rainfall layer draws the area of no rainfall as transparent. It is not the absence of the
// observation, so it maps to the band below the weakest class of the legend instead of
// radar.BandOutOfRange. The cells outside the image are filled with radar.BandOutOfRange by the
// mapping.
const noRainBand uint8 = 1

// NewPalette returns the palette of the rainfall layer.
//
// It does not need the georeference so it can be used to draw a frame again without configuring
// the whole source.
func NewPalette() (*parse.Palette, error) {
	return parse.NewPalette(scale, noRainBand, paletteEntries)
}

// Fetcher fetches the rainfall layer of Tokyo Amesh.
type Fetcher struct {
	baseURL  string
	unit     string
	coverage radar.Area
	lookback int
	client   *fetch.Client
}

// NewFetcher returns a Fetcher.
//
// coverage is the geographic range that the image covers. It is the same value as the one that is
// given to NewParser and it decides whether the area to collect needs this source at all.
func NewFetcher(baseURL string, coverage radar.Area, client *fetch.Client) *Fetcher {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Fetcher{
		baseURL:  baseURL,
		unit:     DefaultUnit,
		coverage: coverage,
		lookback: defaultLookback,
		client:   client,
	}
}

func (f *Fetcher) ID() radar.SourceID { return radar.SourceAmesh }

func (f *Fetcher) Interval() time.Duration { return Interval }

// Units returns the single unit when the area overlaps the coverage.
func (f *Fetcher) Units(area radar.Area) []string {
	if !f.coverage.Intersects(area) {
		return nil
	}
	return []string{f.unit}
}

// URL returns the URL of the rainfall layer of the time.
func (f *Fetcher) URL(t time.Time, unit string) string {
	return fmt.Sprintf("%s/mesh/%s/%s.gif", f.baseURL, unit, t.In(radar.JST).Format("200601021504"))
}

// Latest checks the existence of the recent minutes and returns the ones that are published.
//
// The URL contains the time so there is no index to read. The existence of each candidate is
// checked by HEAD instead.
func (f *Fetcher) Latest(ctx context.Context, since time.Time) ([]time.Time, error) {
	var times []time.Time
	for _, t := range f.candidates(time.Now().Truncate(Interval), since) {
		ok, err := f.client.Exists(ctx, f.URL(t, f.unit))
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
	res, err := f.client.Get(ctx, f.URL(t, unit))
	if err != nil {
		return nil, err
	}
	return &radar.RawObject{
		Source:      radar.SourceAmesh,
		Unit:        unit,
		ValidTime:   t.Truncate(Interval),
		ContentType: res.ContentType,
		Body:        res.Body,
	}, nil
}

// Parser converts the rainfall layer into the rainfall on the common mesh.
type Parser struct {
	palette *parse.Palette
	mapping *parse.Mapping
	scale   *radar.ScaleTable
	// width and height are the size that the georeference assumes. An image of another size cannot
	// be mapped with this mapping.
	width, height int
}

// NewParser returns a Parser.
//
// georef is the georeference of the image. It has to be determined by fitting the map layer to the
// coastline before the collecting starts. An error of the georeference cannot be fixed afterwards
// because only the mapped values are stored, so there is no default value here on purpose.
func NewParser(georef parse.Georeference, zoom int) (*Parser, error) {
	palette, err := NewPalette()
	if err != nil {
		return nil, err
	}
	w, h := georef.Size()
	if w <= 0 || h <= 0 {
		return nil, xerrors.NewWithStack("amesh: the size of the georeference is not set")
	}
	return &Parser{
		palette: palette,
		mapping: parse.NewMapping(georef, zoom),
		scale:   scale,
		width:   w,
		height:  h,
	}, nil
}

func (p *Parser) ID() radar.SourceID { return radar.SourceAmesh }

// Parse decodes the GIF and maps it onto the common mesh.
//
// The image is a paletted GIF. The color table is resolved through the known palette before any
// pixel is converted so that an unknown color is detected up front. The palette of the GIF is
// optimized per image and the order of the entries is not stable, so the color is what is looked
// up, not the index of the palette.
func (p *Parser) Parse(raw *radar.RawObject) ([]*radar.Frame, error) {
	img, err := gif.Decode(bytes.NewReader(raw.Body))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	paletted, ok := img.(*image.Paletted)
	if !ok {
		return nil, xerrors.NewfWithStack("amesh: unexpected image type: %T", img)
	}

	b := paletted.Bounds()
	// The mapping was built for the size that the georeference declared. An image of another size
	// means that the source changed the layout and mapping it would silently shift every cell.
	if b.Dx() != p.width || b.Dy() != p.height {
		return nil, xerrors.NewfWithStack("amesh: unexpected image size: %dx%d != %dx%d", b.Dx(), b.Dy(), p.width, p.height)
	}

	lut, err := p.lookupTable(paletted)
	if err != nil {
		return nil, err
	}

	at := func(px, py int) uint8 {
		return lut[paletted.Pix[paletted.PixOffset(b.Min.X+px, b.Min.Y+py)]]
	}

	tiles := p.mapping.Tiles()
	values := p.mapping.Apply(at)
	frames := make([]*radar.Frame, len(tiles))
	for i := range tiles {
		frames[i] = &radar.Frame{
			Source:       radar.SourceAmesh,
			Tile:         tiles[i],
			ValidTime:    raw.ValidTime,
			ScaleVersion: p.scale.Version,
			Values:       values[i],
		}
	}
	return frames, nil
}

// lookupTable resolves the color table of the image into the band index of each palette entry.
//
// Only the entries that a pixel actually refers to are resolved. The color table of a GIF is
// padded to a power of two and the padding is opaque black, which is not a color that appeared in
// the data. Rejecting a frame because of the padding would throw away good frames.
func (p *Parser) lookupTable(img *image.Paletted) ([]uint8, error) {
	used := make([]bool, len(img.Palette))
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := img.Pix[img.PixOffset(b.Min.X, y) : img.PixOffset(b.Min.X, y)+b.Dx()]
		for _, v := range row {
			if int(v) < len(used) {
				used[v] = true
			}
		}
	}

	lut := make([]uint8, len(img.Palette))
	for i, c := range img.Palette {
		if !used[i] {
			continue
		}
		idx, err := p.palette.Index(color.RGBAModel.Convert(c).(color.RGBA))
		if err != nil {
			return nil, err
		}
		lut[i] = idx
	}
	return lut, nil
}

// Palette returns the palette. It is used to draw a frame as an image again.
func (p *Parser) Palette() *parse.Palette { return p.palette }
