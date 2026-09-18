package xrain

import (
	"context"
	"image/color"
	"math"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"

	"go.f110.dev/mono/go/radar"
	"go.f110.dev/mono/go/radar/fetch"
	"go.f110.dev/mono/go/testing/assertion"
)

func testClient(tr http.RoundTripper) *fetch.Client {
	return fetch.NewClient(fetch.ClientOpt{Transport: tr, MinInterval: time.Nanosecond, RetryWait: time.Millisecond})
}

func testDefinition(t *testing.T) *MeshDefinition {
	t.Helper()
	body, err := os.ReadFile("testdata/xrd_2.json")
	assertion.MustNoError(t, err)
	def, err := ParseMeshDefinition(body)
	assertion.MustNoError(t, err)
	return def
}

func testImage(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/xrd_82_2_4.png")
	assertion.MustNoError(t, err)
	return body
}

func TestPrimaryMeshBounds(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		b, err := primaryMeshBounds(5438)
		assertion.MustNoError(t, err)
		assertion.True(t, math.Abs(b.MinLat-36.0) < 1e-9)
		assertion.True(t, math.Abs(b.MaxLat-(36.0+2.0/3.0)) < 1e-9)
		assertion.Equal(t, b.MinLon, float64(138))
		assertion.Equal(t, b.MaxLon, float64(139))
	})

	t.Run("invalid", func(t *testing.T) {
		cases := []struct {
			name string
			code int
		}{
			{name: "zero", code: 0},
			{name: "too few digits", code: 999},
			{name: "too many digits", code: 10000},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := primaryMeshBounds(tc.code)
				assertion.Error(t, err)
			})
		}
	})
}

func TestAreaMesh(t *testing.T) {
	a := AreaMesh{Seq: 4, AreaCd: 82, MeshFrom: 5638, MeshTo: 5641}

	t.Run("Unit", func(t *testing.T) {
		assertion.Equal(t, a.Unit(), "82/4")
	})

	t.Run("Bounds", func(t *testing.T) {
		b, err := a.Bounds()
		assertion.MustNoError(t, err)
		assertion.True(t, math.Abs(b.MinLat-(56.0/1.5)) < 1e-9)
		assertion.True(t, math.Abs(b.MaxLat-(56.0/1.5+2.0/3.0)) < 1e-9)
		assertion.Equal(t, b.MinLon, float64(138))
		assertion.Equal(t, b.MaxLon, float64(142))
	})

	t.Run("Bounds of an invalid mesh code", func(t *testing.T) {
		_, err := AreaMesh{Seq: 1, AreaCd: 82, MeshFrom: 1, MeshTo: 5641}.Bounds()
		assertion.Error(t, err)
	})
}

func TestParseMeshDefinition(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		def := testDefinition(t)
		assertion.Equal(t, def.Type, "xrain")
		assertion.Equal(t, def.Level, LevelNative)
		assertion.NotEmpty(t, def.Areas)
	})

	t.Run("invalid", func(t *testing.T) {
		cases := []struct {
			name string
			body string
		}{
			{name: "no area", body: `{"type":"xrain","level":2,"areaInfo":[]}`},
			{name: "not json", body: `not json`},
			{
				name: "an invalid mesh code",
				body: `{"type":"xrain","level":2,"areaInfo":[{"seq":1,"areaCd":82,"meshFrom":1,"meshTo":2}]}`,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := ParseMeshDefinition([]byte(tc.body))
				assertion.Error(t, err)
			})
		}
	})
}

func TestMeshDefinition_Find(t *testing.T) {
	def := testDefinition(t)

	t.Run("known unit", func(t *testing.T) {
		a, err := def.Find("82/4")
		assertion.MustNoError(t, err)
		assertion.Equal(t, a.MeshFrom, 5638)
		assertion.Equal(t, a.MeshTo, 5641)
	})

	cases := []struct {
		name string
		unit string
	}{
		{name: "unknown seq", unit: "82/9999"},
		{name: "not a unit", unit: "not-a-unit"},
		{name: "seq is not a number", unit: "82/x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := def.Find(tc.unit)
			assertion.Error(t, err)
		})
	}
}

func TestDefinitionURL(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		level   int
		want    string
	}{
		{name: "default origin", level: 2, want: "https://www.river.go.jp/kawabou/file/files/master/radar/xrd/2.json"},
		{name: "another origin", baseURL: "https://example.invalid", level: 1, want: "https://example.invalid/kawabou/file/files/master/radar/xrd/1.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertion.Equal(t, DefinitionURL(tc.baseURL, tc.level), tc.want)
		})
	}
}

func TestNewFetcher(t *testing.T) {
	t.Run("requires the definition", func(t *testing.T) {
		_, err := NewFetcher("", nil, testClient(nil))
		assertion.Error(t, err)
	})

	t.Run("the default origin is used when it is empty", func(t *testing.T) {
		f, err := NewFetcher("", testDefinition(t), testClient(nil))
		assertion.MustNoError(t, err)
		assertion.Equal(t, f.baseURL, DefaultBaseURL)
	})
}

func TestFetcher_URL(t *testing.T) {
	f, err := NewFetcher("", testDefinition(t), testClient(nil))
	assertion.MustNoError(t, err)

	t.Run("the time is jst", func(t *testing.T) {
		got, err := f.URL(time.Date(2026, 8, 19, 7, 53, 0, 0, time.UTC), "82/4")
		assertion.MustNoError(t, err)
		assertion.Equal(t, got, "https://www.river.go.jp/kawabou/file/radar/image/xrd/82/2/20260819/1653/4.png")
	})

	t.Run("invalid unit", func(t *testing.T) {
		_, err := f.URL(time.Now(), "not-a-unit")
		assertion.Error(t, err)
	})
}

func TestFetcher_Units(t *testing.T) {
	f, err := NewFetcher("", testDefinition(t), testClient(nil))
	assertion.MustNoError(t, err)

	t.Run("inside the coverage", func(t *testing.T) {
		area := radar.Area{MinLat: 37.5, MinLon: 139.0, MaxLat: 37.6, MaxLon: 139.1}
		units := f.Units(area)
		assertion.NotEmpty(t, units)
		for _, u := range units {
			a, err := f.def.Find(u)
			assertion.MustNoError(t, err)
			b, err := a.Bounds()
			assertion.MustNoError(t, err)
			assertion.True(t, b.Intersects(area))
		}
	})

	t.Run("out of the coverage", func(t *testing.T) {
		assertion.Empty(t, f.Units(radar.Area{MinLat: 24.0, MinLon: 123.0, MaxLat: 24.5, MaxLon: 123.5}))
	})
}

func TestFetcher_Fetch(t *testing.T) {
	body := testImage(t)
	at := time.Date(2026, 8, 19, 16, 53, 0, 0, radar.JST)

	t.Run("published", func(t *testing.T) {
		tr := httpmock.NewMockTransport()
		tr.RegisterResponder(http.MethodGet, "https://www.river.go.jp/kawabou/file/radar/image/xrd/82/2/20260819/1653/4.png",
			httpmock.NewBytesResponder(http.StatusOK, body))

		f, err := NewFetcher("", testDefinition(t), testClient(tr))
		assertion.MustNoError(t, err)

		raw, err := f.Fetch(context.Background(), at, "82/4")
		assertion.MustNoError(t, err)
		assertion.Equal(t, raw.Source, radar.SourceXRAIN)
		assertion.Equal(t, raw.Unit, "82/4")
		assertion.Equal(t, raw.ValidTime, at)
		assertion.Equal(t, raw.Body, body)
	})

	t.Run("not published", func(t *testing.T) {
		tr := httpmock.NewMockTransport()
		tr.RegisterNoResponder(httpmock.NewStringResponder(http.StatusNotFound, ""))

		f, err := NewFetcher("", testDefinition(t), testClient(tr))
		assertion.MustNoError(t, err)

		_, err = f.Fetch(context.Background(), at, "82/4")
		assertion.Error(t, err)
	})

	t.Run("invalid unit", func(t *testing.T) {
		f, err := NewFetcher("", testDefinition(t), testClient(nil))
		assertion.MustNoError(t, err)

		_, err = f.Fetch(context.Background(), at, "not-a-unit")
		assertion.Error(t, err)
	})
}

func TestNewParser(t *testing.T) {
	cases := []struct {
		name string
		def  *MeshDefinition
		zoom int
	}{
		{name: "no definition", zoom: 10},
		{name: "no zoom level", def: testDefinition(t)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewParser(tc.def, tc.zoom)
			assertion.Error(t, err)
		})
	}
}

func TestParser_Parse(t *testing.T) {
	body := testImage(t)

	t.Run("the real image", func(t *testing.T) {
		p, err := NewParser(testDefinition(t), 10)
		assertion.MustNoError(t, err)

		frames, err := p.Parse(&radar.RawObject{
			Source:    radar.SourceXRAIN,
			Unit:      "82/4",
			ValidTime: time.Date(2026, 8, 19, 16, 53, 0, 0, radar.JST),
			Body:      body,
		})
		assertion.MustNoError(t, err)
		assertion.NotEmpty(t, frames)

		seen := make(map[uint8]int)
		for _, f := range frames {
			assertion.Equal(t, f.Source, radar.SourceXRAIN)
			assertion.Equal(t, f.ScaleVersion, ScaleVersion1)
			assertion.Equal(t, f.Tile.Z, 10)
			assertion.Len(t, f.Values, radar.TileSize*radar.TileSize)
			for _, v := range f.Values {
				_, err := scale.Band(v)
				assertion.MustNoError(t, err)
				seen[v]++
			}
		}

		assertion.True(t, seen[radar.BandOutOfRange] > 0)
		var rain int
		for band, n := range seen {
			if band != radar.BandOutOfRange {
				rain += n
			}
		}
		assertion.True(t, rain > 0)
		t.Logf("tiles=%d bands=%v", len(frames), seen)
	})

	t.Run("the mapping is cached per unit", func(t *testing.T) {
		p, err := NewParser(testDefinition(t), 10)
		assertion.MustNoError(t, err)

		raw := &radar.RawObject{Source: radar.SourceXRAIN, Unit: "82/4", Body: body}
		first, err := p.Parse(raw)
		assertion.MustNoError(t, err)
		second, err := p.Parse(raw)
		assertion.MustNoError(t, err)
		assertion.Equal(t, len(second), len(first))
		assertion.Len(t, p.mappings, 1)
	})

	t.Run("invalid", func(t *testing.T) {
		cases := []struct {
			name string
			unit string
			body []byte
		}{
			{name: "unknown unit", unit: "82/9999", body: body},
			{name: "not an image", unit: "82/4", body: []byte("not a png")},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				p, err := NewParser(testDefinition(t), 10)
				assertion.MustNoError(t, err)
				_, err = p.Parse(&radar.RawObject{Source: radar.SourceXRAIN, Unit: tc.unit, Body: tc.body})
				assertion.Error(t, err)
			})
		}
	})
}

func TestNewPalette(t *testing.T) {
	p, err := NewPalette()
	assertion.MustNoError(t, err)

	t.Run("every color of the legend", func(t *testing.T) {
		for i, e := range paletteEntries {
			idx, err := p.Index(e.Color)
			assertion.MustNoError(t, err)
			assertion.Equal(t, idx, uint8(i+1))

			minMMH, maxMMH := scale.Rainfall(idx)
			assertion.Equal(t, minMMH, e.MinMMH)
			assertion.Equal(t, maxMMH, e.MaxMMH)
		}
	})

	t.Run("transparent is out of range", func(t *testing.T) {
		idx, err := p.Index(color.RGBA{})
		assertion.MustNoError(t, err)
		assertion.Equal(t, idx, radar.BandOutOfRange)
	})
}

func TestScaleTable(t *testing.T) {
	s, err := radar.CurrentScale(radar.SourceXRAIN)
	assertion.MustNoError(t, err)
	assertion.Equal(t, s.Version, ScaleVersion1)

	t.Run("lowest class", func(t *testing.T) {
		minMMH, maxMMH := s.Rainfall(1)
		assertion.Equal(t, minMMH, float64(0))
		assertion.Equal(t, maxMMH, float64(1))
	})

	t.Run("highest class has no upper bound", func(t *testing.T) {
		minMMH, maxMMH := s.Rainfall(8)
		assertion.Equal(t, minMMH, float64(80))
		assertion.True(t, math.IsInf(maxMMH, 1))
	})

	t.Run("a class of another source does not fit", func(t *testing.T) {
		_, err := s.IndexOf(30, 40)
		assertion.Error(t, err)
	})
}
