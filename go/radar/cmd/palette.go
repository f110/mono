package cmd

import (
	"encoding/json"
	"image/color"
	"math"
	"os"
	"strconv"
	"strings"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/radar/parse"
)

// paletteFile is the definition of a palette of a source.
//
// It is read from a file because the classes of XRAIN have not been confirmed against the real
// data yet. Hard coding a guess would be worse than requiring the operator to write down what was
// actually observed.
//
//	[
//	  {"color": "#ccffff", "min_mmh": 1, "max_mmh": 5},
//	  {"color": "#7030a0", "min_mmh": 75}
//	]
//
// max_mmh can be omitted for the last class which does not have an upper bound.
type paletteEntryFile struct {
	Color  string   `json:"color"`
	MinMMH float64  `json:"min_mmh"`
	MaxMMH *float64 `json:"max_mmh"`
}

func loadPalette(path string) ([]parse.PaletteEntry, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	var entries []paletteEntryFile
	if err := json.Unmarshal(buf, &entries); err != nil {
		return nil, xerrors.WithStack(err)
	}
	if len(entries) == 0 {
		return nil, xerrors.NewfWithStack("radar: %s does not have any entry", path)
	}

	out := make([]parse.PaletteEntry, len(entries))
	for i, e := range entries {
		c, err := parseHexColor(e.Color)
		if err != nil {
			return nil, err
		}
		maxMMH := math.Inf(1)
		if e.MaxMMH != nil {
			maxMMH = *e.MaxMMH
		}
		out[i] = parse.PaletteEntry{Color: c, MinMMH: e.MinMMH, MaxMMH: maxMMH}
	}
	return out, nil
}

// parseHexColor parses "#rrggbb". The alpha is always opaque because a transparent pixel is
// handled by the parser of the source, not by the palette.
func parseHexColor(s string) (color.RGBA, error) {
	v := strings.TrimPrefix(s, "#")
	if len(v) != 6 {
		return color.RGBA{}, xerrors.NewfWithStack("radar: invalid color: %s", s)
	}
	n, err := strconv.ParseUint(v, 16, 32)
	if err != nil {
		return color.RGBA{}, xerrors.NewfWithStack("radar: invalid color: %s", s)
	}
	return color.RGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 255}, nil
}
