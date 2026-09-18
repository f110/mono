package cmd

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/radar"
	"go.f110.dev/mono/go/radar/amesh"
	"go.f110.dev/mono/go/radar/dayfile"
	"go.f110.dev/mono/go/radar/parse"
	"go.f110.dev/mono/go/radar/xrain"
)

type extractFlags struct {
	source string
	tile   string
	// zoom is the zoom level of the common mesh. A coordinate is turned into a tile with it, so it
	// has to match the value the collector ran with.
	zoom     int
	lat, lon float64
	date     string
	at       string
	timeSpan string
	output   string
	palette  string
}

func init() {
	register(&extractCommand{})
}

type extractCommand struct {
	extractFlags
	storageFlags
}

func (c *extractCommand) Command() *cli.Command {
	f := &c.extractFlags
	cmd := &cli.Command{
		Use:   "extract",
		Short: "Take the rainfall of a point or a tile out of the archive",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return extract(ctx, f, c.newStore(), os.Stdout)
		},
	}
	cmd.Flags().String("source", "Source to read").Var(&f.source).Required()
	cmd.Flags().String("tile", "Tile of the common mesh as z/x/y").Var(&f.tile)
	cmd.Flags().Int("zoom", "Zoom level of the common mesh. It has to match the collector").Var(&f.zoom).Default(10)
	cmd.Flags().Float64("lat", "Latitude of the point").Var(&f.lat)
	cmd.Flags().Float64("lon", "Longitude of the point").Var(&f.lon)
	cmd.Flags().String("date", "Date in JST as YYYY-MM-DD").Var(&f.date)
	cmd.Flags().String("at", "Time in RFC3339").Var(&f.at)
	cmd.Flags().String("range", "Time range of the day as hh:mm-hh:mm").Var(&f.timeSpan)
	cmd.Flags().String("output", "Directory or the file to write the images into").Var(&f.output).Shorthand("o")
	cmd.Flags().String("palette", "Path of the palette definition. It overrides the palette that the source has in the code").Var(&f.palette)
	c.storageFlags.register(cmd.Flags())
	return cmd
}

func extract(ctx context.Context, f *extractFlags, store objectStore, out *os.File) error {
	source, err := radar.ParseSourceID(f.source)
	if err != nil {
		return err
	}
	date, at, err := f.when()
	if err != nil {
		return err
	}

	byPoint := f.tile == ""
	var tile radar.Tile
	var px, py int
	if byPoint {
		if f.lat == 0 && f.lon == 0 {
			return xerrors.NewWithStack("radar: either --tile or --lat and --lon is required")
		}
		// The coordinate decides the tile by calculation. No index is needed.
		tile, px, py = radar.TileOf(f.zoom, f.lat, f.lon)
	} else {
		tile, err = radar.ParseTile(f.tile)
		if err != nil {
			return err
		}
	}

	key := objectKey(source, tile, date)
	obj, err := store.Get(ctx, key)
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer obj.Body.Close()

	r, err := dayfile.NewReader(obj.Body)
	if err != nil {
		return err
	}
	slots, err := f.slots(r, date, at)
	if err != nil {
		return err
	}

	if byPoint {
		return printPoint(out, r, date, slots, px, py)
	}
	return writeImages(f, r, source, date, slots)
}

// when returns the date and the exact time to read.
func (f *extractFlags) when() (time.Time, time.Time, error) {
	if f.at != "" {
		t, err := time.Parse(time.RFC3339, f.at)
		if err != nil {
			return time.Time{}, time.Time{}, xerrors.WithStack(err)
		}
		return radar.DateOf(t), t, nil
	}
	if f.date == "" {
		return time.Time{}, time.Time{}, xerrors.NewWithStack("radar: either --at or --date is required")
	}
	d, err := time.ParseInLocation("2006-01-02", f.date, radar.JST)
	if err != nil {
		return time.Time{}, time.Time{}, xerrors.WithStack(err)
	}
	return d, time.Time{}, nil
}

// slots returns the slots to read.
func (f *extractFlags) slots(r *dayfile.Reader, date time.Time, at time.Time) ([]int, error) {
	h := r.Header()
	if !at.IsZero() {
		// The caller indexes into the frame index with what is returned, so an out of range slot
		// has to be an error here instead of a panic there.
		slot := radar.Slot(at, h.Interval)
		if slot < 0 || slot >= h.FrameCount {
			return nil, xerrors.NewfWithStack("radar: %s is out of the day", at.Format(time.RFC3339))
		}
		return []int{slot}, nil
	}
	if f.timeSpan == "" {
		// The whole day.
		slots := make([]int, 0, h.FrameCount)
		for i := 0; i < h.FrameCount; i++ {
			slots = append(slots, i)
		}
		return slots, nil
	}

	var fromHour, fromMin, toHour, toMin int
	if _, err := fmt.Sscanf(f.timeSpan, "%d:%d-%d:%d", &fromHour, &fromMin, &toHour, &toMin); err != nil {
		return nil, xerrors.NewfWithStack("radar: invalid range: %s", f.timeSpan)
	}
	from := radar.Slot(date.Add(time.Duration(fromHour)*time.Hour+time.Duration(fromMin)*time.Minute), h.Interval)
	to := radar.Slot(date.Add(time.Duration(toHour)*time.Hour+time.Duration(toMin)*time.Minute), h.Interval)
	if from < 0 || from > to {
		return nil, xerrors.NewfWithStack("radar: invalid range: %s", f.timeSpan)
	}
	slots := make([]int, 0, to-from+1)
	for i := from; i <= to && i < h.FrameCount; i++ {
		slots = append(slots, i)
	}
	return slots, nil
}

// printPoint writes the rainfall of a single cell for each slot.
//
// Only one cell is read per slot so the whole frame is never copied.
func printPoint(out *os.File, r *dayfile.Reader, date time.Time, slots []int, px, py int) error {
	h := r.Header()
	scale, err := radar.Scale(h.Source, h.ScaleVersion)
	if err != nil {
		return err
	}
	kinds := r.Kinds()

	for _, slot := range slots {
		at := radar.SlotTime(date, slot, h.Interval)
		if kinds[slot] == dayfile.KindMissing {
			fmt.Fprintf(out, "%s\tmissing\n", at.Format(time.RFC3339))
			continue
		}
		band, err := r.Cell(slot, px, py)
		if err != nil {
			return err
		}
		if band == radar.BandOutOfRange {
			fmt.Fprintf(out, "%s\tout of range\n", at.Format(time.RFC3339))
			continue
		}
		minMMH, maxMMH := scale.Rainfall(band)
		if math.IsInf(maxMMH, 1) {
			fmt.Fprintf(out, "%s\t%g-\n", at.Format(time.RFC3339), minMMH)
			continue
		}
		fmt.Fprintf(out, "%s\t%g-%g\n", at.Format(time.RFC3339), minMMH, maxMMH)
	}
	return nil
}

// writeImages draws the tile of each slot as a PNG.
func writeImages(f *extractFlags, r *dayfile.Reader, source radar.SourceID, date time.Time, slots []int) error {
	if f.output == "" {
		return xerrors.NewWithStack("radar: --output is required to draw a tile")
	}
	palette, err := drawingPalette(source, f.palette)
	if err != nil {
		return err
	}

	h := r.Header()
	kinds := r.Kinds()
	multiple := len(slots) > 1
	if multiple {
		if err := os.MkdirAll(f.output, 0755); err != nil {
			return xerrors.WithStack(err)
		}
	}

	for _, slot := range slots {
		if kinds[slot] == dayfile.KindMissing {
			continue
		}
		values, err := r.Frame(slot)
		if err != nil {
			return err
		}
		img := image.NewRGBA(image.Rect(0, 0, h.TileSize, h.TileSize))
		for i, v := range values {
			if v == radar.BandOutOfRange {
				continue
			}
			c, ok := palette.Color(v)
			if !ok {
				continue
			}
			img.SetRGBA(i%h.TileSize, i/h.TileSize, c)
		}

		p := f.output
		if multiple {
			at := radar.SlotTime(date, slot, h.Interval)
			p = filepath.Join(f.output, at.Format("1504")+".png")
		}
		file, err := os.Create(p)
		if err != nil {
			return xerrors.WithStack(err)
		}
		if err := png.Encode(file, img); err != nil {
			file.Close()
			return xerrors.WithStack(err)
		}
		if err := file.Close(); err != nil {
			return xerrors.WithStack(err)
		}
	}
	return nil
}

// drawingPalette returns the palette to draw a frame again.
//
// Both sources have their palette in the code because their legends were confirmed against the
// real data. A path overrides it, which is how a source that is added later can be drawn before
// its palette is written down in the code.
func drawingPalette(source radar.SourceID, path string) (*parse.Palette, error) {
	if path != "" {
		entries, err := loadPalette(path)
		if err != nil {
			return nil, err
		}
		s, err := radar.CurrentScale(source)
		if err != nil {
			return nil, err
		}
		return parse.NewPalette(s, radar.BandOutOfRange, entries)
	}
	switch source {
	case radar.SourceAmesh:
		return amesh.NewPalette()
	case radar.SourceXRAIN:
		return xrain.NewPalette()
	}
	return nil, xerrors.NewfWithStack("radar: --palette is required to draw %s", source.String())
}
