package amesh

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"

	"go.f110.dev/mono/go/radar"
	"go.f110.dev/mono/go/radar/fetch"
	"go.f110.dev/mono/go/radar/parse"
	"go.f110.dev/mono/go/testing/assertion"
)

var testCoverage = radar.Area{MinLat: 35.0, MinLon: 138.9, MaxLat: 36.25, MaxLon: 140.9}

func testGeoreference() parse.Georeference {
	return parse.MercatorBox{Width: ImageWidth, Height: ImageHeight, Area: testCoverage}
}

func smallGeoreference() parse.Georeference {
	return parse.MercatorBox{Width: 2, Height: 2, Area: testCoverage}
}

func testClient(tr http.RoundTripper) *fetch.Client {
	return fetch.NewClient(fetch.ClientOpt{
		Transport:   tr,
		MinInterval: time.Nanosecond,
		RetryWait:   time.Millisecond,
	})
}

func TestFetcherURL(t *testing.T) {
	f := NewFetcher("", testCoverage, testClient(nil))
	got := f.URL(time.Date(2026, 7, 16, 15, 0, 0, 0, time.UTC), DefaultUnit)
	assertion.Equal(t, got, "https://tokyo-ame.jwa.or.jp/mesh/000/202607170000.gif")
}

func TestFetcherUnits(t *testing.T) {
	cases := []struct {
		name string
		area radar.Area
		want []string
	}{
		{name: "inside the coverage", area: radar.Area{MinLat: 35.6, MinLon: 139.6, MaxLat: 35.8, MaxLon: 139.9}, want: []string{DefaultUnit}},
		{name: "out of the coverage", area: radar.Area{MinLat: 43.0, MinLon: 141.0, MaxLat: 43.2, MaxLon: 141.5}},
	}

	f := NewFetcher("", testCoverage, testClient(nil))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertion.Equal(t, f.Units(tc.area), tc.want)
		})
	}
}

func TestFetcherFetch(t *testing.T) {
	tr := httpmock.NewMockTransport()
	body, err := os.ReadFile("testdata/mesh_000.gif")
	assertion.MustNoError(t, err)
	tr.RegisterResponder(http.MethodGet, "https://tokyo-ame.jwa.or.jp/mesh/000/202607160900.gif",
		httpmock.NewBytesResponder(http.StatusOK, body))

	f := NewFetcher("", testCoverage, testClient(tr))
	at := time.Date(2026, 7, 16, 9, 0, 0, 0, radar.JST)
	raw, err := f.Fetch(context.Background(), at, DefaultUnit)
	assertion.MustNoError(t, err)
	assertion.Equal(t, raw.Source, radar.SourceAmesh)
	assertion.Equal(t, raw.Unit, DefaultUnit)
	assertion.Equal(t, raw.ValidTime, at)
	assertion.Equal(t, raw.Body, body)
}

func TestFetcherFetchNotFound(t *testing.T) {
	tr := httpmock.NewMockTransport()
	tr.RegisterNoResponder(httpmock.NewStringResponder(http.StatusNotFound, ""))

	f := NewFetcher("", testCoverage, testClient(tr))
	_, err := f.Fetch(context.Background(), time.Now(), DefaultUnit)
	assertion.Error(t, err)
}

func TestFetcherLatest(t *testing.T) {
	tr := httpmock.NewMockTransport()
	now := time.Now().Truncate(Interval)
	published := now.Add(-2 * Interval)
	tr.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
		f := NewFetcher("", testCoverage, testClient(nil))
		if req.URL.String() == f.URL(published, DefaultUnit) {
			return httpmock.NewStringResponse(http.StatusOK, ""), nil
		}
		return httpmock.NewStringResponse(http.StatusNotFound, ""), nil
	})

	f := NewFetcher("", testCoverage, testClient(tr))
	times, err := f.Latest(context.Background(), now.Add(-5*Interval))
	assertion.MustNoError(t, err)
	assertion.Len(t, times, 1)
	assertion.Equal(t, times[0].Unix(), published.Unix())
}

func TestFetcherLatestDoesNotGoBeforeSince(t *testing.T) {
	tr := httpmock.NewMockTransport()
	var requests int
	tr.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
		requests++
		return httpmock.NewStringResponse(http.StatusNotFound, ""), nil
	})

	f := NewFetcher("", testCoverage, testClient(tr))
	now := time.Now().Truncate(Interval)
	_, err := f.Latest(context.Background(), now.Add(-2*Interval))
	assertion.MustNoError(t, err)
	assertion.Equal(t, requests, 2)
}

func TestParseRealImage(t *testing.T) {
	cases := []struct {
		name string
		file string
	}{
		{name: "ordinary color table", file: "testdata/mesh_000.gif"},
		{name: "color table padded with opaque black", file: "testdata/mesh_000_padded_palette.gif"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := os.ReadFile(tc.file)
			assertion.MustNoError(t, err)

			p, err := NewParser(testGeoreference(), 10)
			assertion.MustNoError(t, err)

			frames, err := p.Parse(&radar.RawObject{
				Source:    radar.SourceAmesh,
				Unit:      DefaultUnit,
				ValidTime: time.Date(2026, 8, 19, 16, 0, 0, 0, radar.JST),
				Body:      body,
			})
			assertion.MustNoError(t, err)
			assertion.NotEmpty(t, frames)

			seen := make(map[uint8]int)
			for _, f := range frames {
				assertion.Equal(t, f.Source, radar.SourceAmesh)
				assertion.Equal(t, f.ScaleVersion, ScaleVersion1)
				assertion.Equal(t, f.Tile.Z, 10)
				assertion.Len(t, f.Values, radar.TileSize*radar.TileSize)
				for _, v := range f.Values {
					_, err := scale.Band(v)
					assertion.MustNoError(t, err)
					seen[v]++
				}
			}

			assertion.True(t, seen[noRainBand] > 0)
			assertion.True(t, seen[radar.BandOutOfRange] > 0)
			t.Logf("tiles=%d bands=%v", len(frames), seen)
		})
	}
}

func buildGIF(t *testing.T, used bool) []byte {
	t.Helper()
	img := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{
		color.RGBA{R: 255, G: 255, B: 255, A: 255},
		color.RGBA{R: 1, G: 2, B: 3, A: 255},
	})
	if used {
		img.Pix[0] = 1
	}
	buf := &bytes.Buffer{}
	assertion.MustNoError(t, gif.Encode(buf, img, nil))
	return buf.Bytes()
}

func TestParseUnknownColor(t *testing.T) {
	cases := []struct {
		name    string
		used    bool
		wantErr bool
	}{
		{name: "used by a pixel", used: true, wantErr: true},
		{name: "only in the color table", used: false, wantErr: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewParser(smallGeoreference(), 10)
			assertion.MustNoError(t, err)

			frames, err := p.Parse(&radar.RawObject{Source: radar.SourceAmesh, Body: buildGIF(t, tc.used)})
			if tc.wantErr {
				assertion.Error(t, err)
				return
			}
			assertion.MustNoError(t, err)
			assertion.NotEmpty(t, frames)
		})
	}
}

func TestParseRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		body []byte
	}{
		{name: "not an image", body: []byte("not a gif")},
		{name: "unexpected size", body: buildGIF(t, false)},
	}

	p, err := NewParser(testGeoreference(), 10)
	assertion.MustNoError(t, err)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p.Parse(&radar.RawObject{Source: radar.SourceAmesh, Body: tc.body})
			assertion.Error(t, err)
		})
	}
}

func TestNewParserRequiresTheSizeOfTheGeoreference(t *testing.T) {
	_, err := NewParser(parse.MercatorBox{Area: testCoverage}, 10)
	assertion.Error(t, err)
}
