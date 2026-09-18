// Package cmd implements the subcommands of the radar command.
package cmd

import (
	"context"
	"errors"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/radar"
	"go.f110.dev/mono/go/radar/amesh"
	"go.f110.dev/mono/go/radar/fetch"
	"go.f110.dev/mono/go/radar/parse"
	"go.f110.dev/mono/go/radar/xrain"
)

// fetcher fetches the raw data of a source.
//
// The interface is declared here, where it is used, and the packages of the sources only export
// the concrete types.
type fetcher interface {
	ID() radar.SourceID
	Interval() time.Duration
	Units(area radar.Area) []string
	Latest(ctx context.Context, since time.Time) ([]time.Time, error)
	Fetch(ctx context.Context, t time.Time, unit string) (*radar.RawObject, error)
}

// parser converts the raw data of a source into the rainfall on the common mesh.
type parser interface {
	ID() radar.SourceID
	Parse(raw *radar.RawObject) ([]*radar.Frame, error)
}

// source is a pair of the fetcher and the parser of a single source.
type source struct {
	fetcher fetcher
	parser  parser
}

// ErrNotConfigured means that the flags of a source were not given.
//
// Every constructor of a source reports it the same way so that adding a third source does not
// need to guess which convention to follow.
var ErrNotConfigured = xerrors.Define("radar: the source is not configured")

// subCommand builds a cli.Command. A file that adds one registers it in its init.
type subCommand interface {
	Command() *cli.Command
}

var subCommands []subCommand

func register(c subCommand) {
	subCommands = append(subCommands, c)
}

// AddCommand adds every registered subcommand to the root command.
//
// The logger is initialized here so that each subcommand does not have to do it. It cannot be done
// before Execute because the level is given by a flag.
func AddCommand(rootCmd *cli.Command) {
	for _, s := range subCommands {
		cmd := s.Command()
		run := cmd.Run
		cmd.Run = func(ctx context.Context, c *cli.Command, args []string) error {
			if err := slogger.Init(); err != nil {
				return err
			}
			return run(ctx, c, args)
		}
		rootCmd.AddCommand(cmd)
	}
}

// commonFlags are the flags that decide what the common mesh looks like and which sources are used.
//
// The area and the zoom level are given at runtime instead of being a constant of the build. The
// coverage differs per source and the range to collect is something that is adjusted in operation.
type commonFlags struct {
	zoom    int
	areas   []string
	sources []string

	// ameshBounds is the geographic range of the image of Tokyo Amesh.
	//
	// There is no default on purpose. An error of the georeference cannot be fixed afterwards
	// because only the mapped values are stored, so it has to be determined by fitting the map
	// layer to the coastline before the collecting starts.
	ameshBounds string
	ameshURL    string

	// xrainURL is the origin that publishes XRAIN. The mesh definition of the level is read from
	// it at startup so that the rectangles do not have to be written down by hand.
	xrainURL   string
	xrainLevel int

	requestsLimit int
	// userAgent is sent on every request.
	//
	// www.river.go.jp, which publishes XRAIN, answers 403 to a request whose User-Agent does not
	// look like a browser, so the default is a browser one.
	userAgent string
}

func (f *commonFlags) register(fs *cli.FlagSet) {
	fs.Int("zoom", "Zoom level of the common mesh").Var(&f.zoom).Default(10)
	fs.StringArray("area", "Range to collect as minLat,minLon,maxLat,maxLon. Can be specified multiple times").Var(&f.areas)
	fs.StringArray("source", "Source to use. All the configured sources are used when it is not specified").Var(&f.sources)
	fs.Int("concurrency", "Maximum number of the requests that run at the same time per source").Var(&f.requestsLimit).Default(4)
	fs.String("user-agent", "User-Agent to send. Some sources reject a request that does not look like a browser").Var(&f.userAgent).Default(fetch.BrowserUserAgent)
	fs.String("amesh-bounds", "Geographic range of the Tokyo Amesh image as minLat,minLon,maxLat,maxLon").Var(&f.ameshBounds)
	fs.String("amesh-url", "Origin of Tokyo Amesh").Var(&f.ameshURL).Default(amesh.DefaultBaseURL)
	fs.String("xrain-url", "Origin of XRAIN").Var(&f.xrainURL).Default(xrain.DefaultBaseURL)
	fs.Int("xrain-level", "Level of the XRAIN delivery. 2 matches the 250m mesh that the source observes").Var(&f.xrainLevel).Default(xrain.LevelNative)
}

// areaList parses the areas that were given at runtime.
func (f *commonFlags) areaList() ([]radar.Area, error) {
	if len(f.areas) == 0 {
		return nil, xerrors.NewWithStack("radar: --area is required")
	}
	areas := make([]radar.Area, len(f.areas))
	for i, v := range f.areas {
		a, err := radar.ParseArea(v)
		if err != nil {
			return nil, err
		}
		areas[i] = a
	}
	return areas, nil
}

// sourceList builds the sources that were configured.
// A source that is not configured is left out and the rest of the system never sees it.
func (f *commonFlags) sourceList(ctx context.Context) ([]source, error) {
	if f.zoom <= 0 {
		return nil, xerrors.NewWithStack("radar: --zoom must be positive")
	}

	wanted := make(map[radar.SourceID]bool, len(f.sources))
	for _, v := range f.sources {
		id, err := radar.ParseSourceID(v)
		if err != nil {
			return nil, err
		}
		wanted[id] = true
	}
	enabled := func(id radar.SourceID) bool { return len(wanted) == 0 || wanted[id] }

	builders := []struct {
		id    radar.SourceID
		build func() (*source, error)
	}{
		{id: radar.SourceAmesh, build: f.newAmesh},
		{id: radar.SourceXRAIN, build: func() (*source, error) { return f.newXRAIN(ctx) }},
	}

	var sources []source
	for _, b := range builders {
		if !enabled(b.id) {
			continue
		}
		s, err := b.build()
		if err != nil {
			// A source that was not asked for explicitly is left out when it is not configured.
			// Asking for it by name and leaving it unconfigured is an error.
			if errors.Is(err, ErrNotConfigured) && len(wanted) == 0 {
				continue
			}
			return nil, err
		}
		sources = append(sources, *s)
	}
	if len(sources) == 0 {
		return nil, xerrors.NewWithStack("radar: no source is configured")
	}
	return sources, nil
}

func (f *commonFlags) newAmesh() (*source, error) {
	if f.ameshBounds == "" {
		return nil, xerrors.WithMessage(ErrNotConfigured, "--amesh-bounds is required to use amesh")
	}
	bounds, err := radar.ParseArea(f.ameshBounds)
	if err != nil {
		return nil, err
	}
	client := fetch.NewClient(fetch.ClientOpt{Concurrency: f.requestsLimit, UserAgent: f.userAgent})
	p, err := amesh.NewParser(parse.MercatorBox{Width: amesh.ImageWidth, Height: amesh.ImageHeight, Area: bounds}, f.zoom)
	if err != nil {
		return nil, err
	}
	return &source{fetcher: amesh.NewFetcher(f.ameshURL, bounds, client), parser: p}, nil
}

func (f *commonFlags) newXRAIN(ctx context.Context) (*source, error) {
	client := fetch.NewClient(fetch.ClientOpt{Concurrency: f.requestsLimit, UserAgent: f.userAgent})
	res, err := client.Get(ctx, xrain.DefinitionURL(f.xrainURL, f.xrainLevel))
	if err != nil {
		return nil, err
	}
	def, err := xrain.ParseMeshDefinition(res.Body)
	if err != nil {
		return nil, err
	}
	fe, err := xrain.NewFetcher(f.xrainURL, def, client)
	if err != nil {
		return nil, err
	}
	p, err := xrain.NewParser(def, f.zoom)
	if err != nil {
		return nil, err
	}
	return &source{fetcher: fe, parser: p}, nil
}
