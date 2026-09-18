package cmd

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/radar"
	"go.f110.dev/mono/go/radar/fetch"
	"go.f110.dev/mono/go/radar/wal"
	"go.f110.dev/mono/go/testing/assertion"
)

// fakeFetcher answers from a fixed set of times and units and records what was asked for.
type fakeFetcher struct {
	times []time.Time
	units map[string][]string
	err   map[string]error

	// mu guards asked. The units of a time are fetched in parallel.
	mu sync.Mutex
	// asked counts the Fetch calls per ValidTime.
	asked map[time.Time]int
}

func (f *fakeFetcher) ID() radar.SourceID { return radar.SourceAmesh }

func (f *fakeFetcher) Interval() time.Duration { return time.Minute }

func (f *fakeFetcher) Units(area radar.Area) []string {
	return f.units[area.String()]
}

func (f *fakeFetcher) Latest(_ context.Context, since time.Time) ([]time.Time, error) {
	var out []time.Time
	for _, t := range f.times {
		if t.After(since) {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeFetcher) Fetch(_ context.Context, t time.Time, unit string) (*radar.RawObject, error) {
	f.mu.Lock()
	if f.asked == nil {
		f.asked = make(map[time.Time]int)
	}
	f.asked[t]++
	f.mu.Unlock()

	if err, ok := f.err[unit]; ok && err != nil {
		return nil, err
	}
	return &radar.RawObject{Source: radar.SourceAmesh, Unit: unit, ValidTime: t, ContentType: "image/gif"}, nil
}

// fetched returns how many times the frame of the time was asked for.
func (f *fakeFetcher) fetched(t time.Time) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked[t]
}

// fakeParser turns a RawObject into one frame of a fixed tile.
type fakeParser struct {
	err error
}

func (p *fakeParser) ID() radar.SourceID { return radar.SourceAmesh }

func (p *fakeParser) Parse(raw *radar.RawObject) ([]*radar.Frame, error) {
	if p.err != nil {
		return nil, p.err
	}
	return []*radar.Frame{{
		Source:    radar.SourceAmesh,
		Tile:      radar.Tile{Z: 10, X: 909, Y: 403},
		ValidTime: raw.ValidTime,
		Values:    make([]byte, radar.TileSize*radar.TileSize),
	}}, nil
}

func newCollectLoop(t *testing.T, f *fakeFetcher, p *fakeParser, areas []radar.Area) *collectLoop {
	t.Helper()
	dir := t.TempDir()
	return &collectLoop{
		src:     source{fetcher: f, parser: p},
		areas:   areas,
		workDir: dir,
		wal:     wal.New(filepath.Join(dir, "wal"), time.Minute),
	}
}

func TestCollectLoopSince(t *testing.T) {
	area := radar.Area{MinLat: 35.6, MinLon: 139.6, MaxLat: 35.8, MaxLon: 139.9}
	at := time.Date(2026, 7, 16, 10, 30, 0, 0, radar.JST)

	cases := []struct {
		name string
		err  error
		// advanced tells whether since is expected to move past the frame.
		advanced bool
	}{
		{name: "success", advanced: true},
		{name: "the frame is not published", err: fetch.ErrNotAvailable, advanced: true},
		{name: "the fetch failed", err: xerrors.NewWithStack("boom")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeFetcher{
				times: []time.Time{at},
				units: map[string][]string{area.String(): {"000"}},
				err:   map[string]error{"000": tc.err},
			}
			l := newCollectLoop(t, f, &fakeParser{}, []radar.Area{area})

			l.once(context.Background(), slog.Default())
			assertion.Equal(t, f.fetched(at), 1)

			// The second tick must retry the frame unless it was accounted for.
			l.once(context.Background(), slog.Default())
			if tc.advanced {
				assertion.Equal(t, f.fetched(at), 1)
				assertion.Equal(t, l.since, at)
			} else {
				assertion.Equal(t, f.fetched(at), 2)
				assertion.True(t, l.since.Before(at))
			}
		})
	}
}

func TestCollectLoopSinceWithAPartialFailure(t *testing.T) {
	// One unit succeeds and the other fails. The frame is not complete so it has to be retried.
	area := radar.Area{MinLat: 35.6, MinLon: 139.6, MaxLat: 35.8, MaxLon: 139.9}
	at := time.Date(2026, 7, 16, 10, 30, 0, 0, radar.JST)

	f := &fakeFetcher{
		times: []time.Time{at},
		units: map[string][]string{area.String(): {"a", "b"}},
		err:   map[string]error{"b": xerrors.NewWithStack("boom")},
	}
	l := newCollectLoop(t, f, &fakeParser{}, []radar.Area{area})

	l.once(context.Background(), slog.Default())
	assertion.True(t, l.since.Before(at))

	l.once(context.Background(), slog.Default())
	assertion.Equal(t, f.fetched(at), 4)
}

func TestCollectLoopQuarantinesAParseFailure(t *testing.T) {
	area := radar.Area{MinLat: 35.6, MinLon: 139.6, MaxLat: 35.8, MaxLon: 139.9}
	at := time.Date(2026, 7, 16, 10, 30, 0, 0, radar.JST)

	f := &fakeFetcher{times: []time.Time{at}, units: map[string][]string{area.String(): {"000"}}}
	l := newCollectLoop(t, f, &fakeParser{err: xerrors.NewWithStack("bad image")}, []radar.Area{area})

	l.once(context.Background(), slog.Default())

	entries, err := wal.New(filepath.Join(l.workDir, "wal"), time.Minute).List(at)
	assertion.MustNoError(t, err)
	assertion.Empty(t, entries)

	assertion.True(t, l.since.Before(at))
}

func TestCollectLoopUnits(t *testing.T) {
	a1 := radar.Area{MinLat: 35.6, MinLon: 139.6, MaxLat: 35.8, MaxLon: 139.9}
	a2 := radar.Area{MinLat: 35.7, MinLon: 139.7, MaxLat: 35.9, MaxLon: 140.0}

	cases := []struct {
		name  string
		units map[string][]string
		want  []string
	}{
		{
			name:  "no overlap",
			units: map[string][]string{a1.String(): {"a"}, a2.String(): {"b"}},
			want:  []string{"a", "b"},
		},
		{
			name:  "the overlap is taken once",
			units: map[string][]string{a1.String(): {"a", "b"}, a2.String(): {"b", "c"}},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "out of the coverage",
			units: map[string][]string{},
			want:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newCollectLoop(t, &fakeFetcher{units: tc.units}, &fakeParser{}, []radar.Area{a1, a2})
			assertion.Equal(t, l.units(), tc.want)
		})
	}
}

func TestPollInterval(t *testing.T) {
	cases := []struct {
		name     string
		interval time.Duration
		want     time.Duration
	}{
		{name: "one minute", interval: time.Minute, want: 30 * time.Second},
		{name: "five minutes", interval: 5 * time.Minute, want: 150 * time.Second},
		{name: "zero", interval: 0, want: time.Second},
		{name: "shorter than the resolution", interval: time.Nanosecond, want: time.Second},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertion.Equal(t, pollInterval(tc.interval), tc.want)
		})
	}
}

func TestSanitizeUnit(t *testing.T) {
	cases := []struct {
		name string
		unit string
		want string
	}{
		{name: "plain", unit: "000", want: "000"},
		{name: "tile coordinate", unit: "82/4", want: "82-4"},
		{name: "empty", unit: "", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertion.Equal(t, sanitizeUnit(tc.unit), tc.want)
		})
	}
}

func TestExtensionOf(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		want        string
	}{
		{name: "gif", contentType: "image/gif", want: ".gif"},
		{name: "png", contentType: "image/png", want: ".png"},
		{name: "jpeg", contentType: "image/jpeg", want: ".jpg"},
		{name: "unknown", contentType: "application/octet-stream", want: ".bin"},
		{name: "empty", contentType: "", want: ".bin"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertion.Equal(t, extensionOf(tc.contentType), tc.want)
		})
	}
}

func TestCollectLoopNotAvailableIsNotAFailure(t *testing.T) {
	// A frame that the source never published must not hold since back, otherwise the collector
	// stops as soon as that time falls out of the lookback window.
	area := radar.Area{MinLat: 35.6, MinLon: 139.6, MaxLat: 35.8, MaxLon: 139.9}
	at := time.Date(2026, 7, 16, 10, 30, 0, 0, radar.JST)
	next := at.Add(time.Minute)

	f := &fakeFetcher{
		times: []time.Time{at, next},
		units: map[string][]string{area.String(): {"000"}},
		err:   map[string]error{"000": fetch.ErrNotAvailable},
	}
	l := newCollectLoop(t, f, &fakeParser{}, []radar.Area{area})

	l.once(context.Background(), slog.Default())
	assertion.Equal(t, l.since, next)
	assertion.True(t, errors.Is(f.err["000"], fetch.ErrNotAvailable))
}

func TestMain(m *testing.M) {
	if err := slogger.Init(); err != nil {
		panic(err)
	}
	m.Run()
}
