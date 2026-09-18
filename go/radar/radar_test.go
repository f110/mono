package radar

import (
	"math"
	"slices"
	"testing"
	"time"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestTileOf(t *testing.T) {
	cases := []struct {
		name     string
		z        int
		lat, lon float64
		tile     Tile
	}{
		{name: "origin", z: 0, lat: 0, lon: 0, tile: Tile{Z: 0, X: 0, Y: 0}},
		{name: "tokyo station z10", z: 10, lat: 35.681236, lon: 139.767125, tile: Tile{Z: 10, X: 909, Y: 403}},
		{name: "tokyo station z14", z: 14, lat: 35.681236, lon: 139.767125, tile: Tile{Z: 14, X: 14552, Y: 6451}},
		{name: "north west end", z: 2, lat: 85, lon: -180, tile: Tile{Z: 2, X: 0, Y: 0}},
		{name: "south east end", z: 2, lat: -85, lon: 179.999, tile: Tile{Z: 2, X: 3, Y: 3}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tile, px, py := TileOf(tc.z, tc.lat, tc.lon)
			assertion.Equal(t, tile, tc.tile)
			assertion.True(t, px >= 0 && px < TileSize)
			assertion.True(t, py >= 0 && py < TileSize)
		})
	}
}

func TestTileCellRoundTrip(t *testing.T) {
	cases := []struct {
		name   string
		tile   Tile
		px, py int
	}{
		{name: "north west cell", tile: Tile{Z: 10, X: 908, Y: 403}, px: 0, py: 0},
		{name: "south east cell", tile: Tile{Z: 10, X: 908, Y: 403}, px: 255, py: 255},
		{name: "middle cell", tile: Tile{Z: 10, X: 908, Y: 403}, px: 128, py: 77},
		{name: "deep zoom", tile: Tile{Z: 14, X: 14552, Y: 6451}, px: 3, py: 200},
		{name: "shallow zoom", tile: Tile{Z: 6, X: 1, Y: 62}, px: 17, py: 17},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lat, lon := tc.tile.Cell(tc.px, tc.py)
			tile, px, py := TileOf(tc.tile.Z, lat, lon)
			assertion.Equal(t, tile, tc.tile)
			assertion.Equal(t, px, tc.px)
			assertion.Equal(t, py, tc.py)
		})
	}
}

func TestTileBounds(t *testing.T) {
	tile := Tile{Z: 10, X: 908, Y: 403}
	b := tile.Bounds()

	t.Run("ordered", func(t *testing.T) {
		assertion.True(t, b.MinLat < b.MaxLat)
		assertion.True(t, b.MinLon < b.MaxLon)
	})

	t.Run("contains every cell", func(t *testing.T) {
		for _, p := range [][2]int{{0, 0}, {255, 255}, {128, 128}} {
			lat, lon := tile.Cell(p[0], p[1])
			assertion.True(t, b.Contains(lat, lon))
		}
	})

	t.Run("adjacent to the next tile", func(t *testing.T) {
		next := Tile{Z: 10, X: 909, Y: 403}.Bounds()
		assertion.True(t, math.Abs(next.MinLon-b.MaxLon) < 1e-9)
	})
}

func TestParseTile(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		tile, err := ParseTile("10/905/402")
		assertion.MustNoError(t, err)
		assertion.Equal(t, tile, Tile{Z: 10, X: 905, Y: 402})
		assertion.Equal(t, tile.String(), "10/905/402")
	})

	cases := []struct {
		name string
		in   string
	}{
		{name: "empty", in: ""},
		{name: "too few elements", in: "10/905"},
		{name: "too many elements", in: "10/905/402/1"},
		{name: "not a number", in: "a/905/402"},
		{name: "negative", in: "10/-1/402"},
		{name: "out of the zoom level", in: "10/1024/402"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseTile(tc.in)
			assertion.Error(t, err)
		})
	}
}

func TestParseArea(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		a, err := ParseArea("35.4,138.9,36.0,140.0")
		assertion.MustNoError(t, err)
		assertion.Equal(t, a, Area{MinLat: 35.4, MinLon: 138.9, MaxLat: 36.0, MaxLon: 140.0})
	})

	cases := []struct {
		name string
		in   string
	}{
		{name: "empty", in: ""},
		{name: "too few elements", in: "35.4,138.9,36.0"},
		{name: "not a number", in: "a,138.9,36.0,140.0"},
		{name: "latitude is inverted", in: "36.0,138.9,35.4,140.0"},
		{name: "longitude is inverted", in: "35.4,140.0,36.0,138.9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseArea(tc.in)
			assertion.Error(t, err)
		})
	}
}

func TestAreaTiles(t *testing.T) {
	t.Run("covers the area", func(t *testing.T) {
		a := Area{MinLat: 35.4, MinLon: 138.9, MaxLat: 36.0, MaxLon: 140.0}
		tiles := a.Tiles(10)
		assertion.NotEmpty(t, tiles)

		for _, v := range tiles {
			assertion.Equal(t, v.Z, 10)
			assertion.True(t, v.Bounds().Intersects(a))
		}
		for _, c := range [][2]float64{{a.MinLat, a.MinLon}, {a.MaxLat, a.MaxLon}, {a.MinLat, a.MaxLon}, {a.MaxLat, a.MinLon}} {
			want, _, _ := TileOf(10, c[0], c[1])
			assertion.True(t, slices.Contains(tiles, want))
		}
	})

	t.Run("tile aligned area", func(t *testing.T) {
		parent := Tile{Z: 9, X: 454, Y: 201}
		assertion.Equal(t, parent.Bounds().Tiles(10), []Tile{
			{Z: 10, X: 908, Y: 402},
			{Z: 10, X: 909, Y: 402},
			{Z: 10, X: 908, Y: 403},
			{Z: 10, X: 909, Y: 403},
		})
		assertion.Equal(t, parent.Bounds().Tiles(9), []Tile{parent})
	})

	t.Run("smaller than a tile", func(t *testing.T) {
		tile := Tile{Z: 10, X: 908, Y: 403}
		b := tile.Bounds()
		a := Area{
			MinLat: (b.MinLat + b.MaxLat) / 2,
			MinLon: (b.MinLon + b.MaxLon) / 2,
			MaxLat: (b.MinLat+b.MaxLat)/2 + 1e-6,
			MaxLon: (b.MinLon+b.MaxLon)/2 + 1e-6,
		}
		assertion.Equal(t, a.Tiles(10), []Tile{tile})
	})
}

func TestFrameCount(t *testing.T) {
	cases := []struct {
		name     string
		interval time.Duration
		want     int
	}{
		{name: "one minute", interval: time.Minute, want: 1440},
		{name: "five minutes", interval: 5 * time.Minute, want: 288},
		{name: "zero", interval: 0, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertion.Equal(t, FrameCount(tc.interval), tc.want)
		})
	}
}

func TestSlot(t *testing.T) {
	cases := []struct {
		name string
		at   time.Time
		slot int
	}{
		{name: "midnight", at: time.Date(2026, 7, 16, 0, 0, 0, 0, JST), slot: 0},
		{name: "one minute", at: time.Date(2026, 7, 16, 0, 1, 0, 0, JST), slot: 1},
		{name: "last", at: time.Date(2026, 7, 16, 23, 59, 0, 0, JST), slot: 1439},
		{name: "not on the boundary", at: time.Date(2026, 7, 16, 0, 1, 30, 0, JST), slot: 1},
		{name: "utc", at: time.Date(2026, 7, 15, 15, 0, 0, 0, time.UTC), slot: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertion.Equal(t, Slot(tc.at, time.Minute), tc.slot)
		})
	}
}

func TestDateOf(t *testing.T) {
	cases := []struct {
		name string
		at   time.Time
		want string
	}{
		{name: "jst", at: time.Date(2026, 7, 16, 12, 0, 0, 0, JST), want: "2026-07-16 00:00:00 +0900"},
		{name: "utc rolls over", at: time.Date(2026, 7, 16, 23, 0, 0, 0, time.UTC), want: "2026-07-17 00:00:00 +0900"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertion.Equal(t, DateOf(tc.at).Format("2006-01-02 15:04:05 -0700"), tc.want)
		})
	}
}

func TestSlotTime(t *testing.T) {
	date := time.Date(2026, 7, 16, 0, 0, 0, 0, JST)

	cases := []struct {
		name string
		slot int
		want string
	}{
		{name: "first", slot: 0, want: "2026-07-16T00:00:00+09:00"},
		{name: "noon", slot: 720, want: "2026-07-16T12:00:00+09:00"},
		{name: "last", slot: 1439, want: "2026-07-16T23:59:00+09:00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			at := SlotTime(date, tc.slot, time.Minute)
			assertion.Equal(t, at.Format(time.RFC3339), tc.want)
			assertion.Equal(t, Slot(at, time.Minute), tc.slot)
		})
	}
}

func TestNewScaleTable(t *testing.T) {
	s := NewScaleTable(SourceAmesh, 1, []float64{1, 5, 10})

	t.Run("out of range has no rainfall", func(t *testing.T) {
		minMMH, maxMMH := s.Rainfall(BandOutOfRange)
		assertion.True(t, math.IsNaN(minMMH))
		assertion.True(t, math.IsNaN(maxMMH))
	})

	t.Run("bands", func(t *testing.T) {
		assertion.Equal(t, s.Bands[1], Band{Index: 1, MinMMH: 0, MaxMMH: 1})
		assertion.Equal(t, s.Bands[2], Band{Index: 2, MinMMH: 1, MaxMMH: 5})
		assertion.Equal(t, s.Bands[3], Band{Index: 3, MinMMH: 5, MaxMMH: 10})
		assertion.Equal(t, s.Bands[4].MinMMH, float64(10))
		assertion.True(t, math.IsInf(s.Bands[4].MaxMMH, 1))
	})

	t.Run("continuous", func(t *testing.T) {
		for i := 2; i < len(s.Bands); i++ {
			assertion.Equal(t, s.Bands[i].MinMMH, s.Bands[i-1].MaxMMH)
			assertion.Equal(t, int(s.Bands[i].Index), i)
		}
	})

	t.Run("index of a known class", func(t *testing.T) {
		idx, err := s.IndexOf(1, 5)
		assertion.MustNoError(t, err)
		assertion.Equal(t, idx, uint8(2))
	})

	t.Run("index of a class that does not match", func(t *testing.T) {
		_, err := s.IndexOf(1, 7)
		assertion.Error(t, err)
	})
}

func TestScaleRegistry(t *testing.T) {
	RegisterScale(NewScaleTable(SourceAmesh, 7, []float64{1, 5}))
	RegisterScale(NewScaleTable(SourceXRAIN, 7, []float64{2, 9}))

	t.Run("the same version is kept per source", func(t *testing.T) {
		cases := []struct {
			name   string
			source SourceID
			want   float64
		}{
			{name: "amesh", source: SourceAmesh, want: 1},
			{name: "xrain", source: SourceXRAIN, want: 2},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := Scale(tc.source, 7)
				assertion.MustNoError(t, err)
				assertion.Equal(t, got.Bands[1].MaxMMH, tc.want)
			})
		}
	})

	t.Run("unknown version", func(t *testing.T) {
		_, err := Scale(SourceAmesh, 999)
		assertion.Error(t, err)
	})

	t.Run("unknown source", func(t *testing.T) {
		_, err := CurrentScale(SourceID(9999))
		assertion.Error(t, err)
	})
}

func TestSourceID(t *testing.T) {
	cases := []struct {
		name   string
		source SourceID
		want   string
	}{
		{name: "xrain", source: SourceXRAIN, want: "xrain"},
		{name: "amesh", source: SourceAmesh, want: "amesh"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertion.Equal(t, tc.source.String(), tc.want)

			id, err := ParseSourceID(tc.want)
			assertion.MustNoError(t, err)
			assertion.Equal(t, id, tc.source)
		})
	}

	t.Run("unknown", func(t *testing.T) {
		_, err := ParseSourceID("unknown")
		assertion.Error(t, err)
	})
}
