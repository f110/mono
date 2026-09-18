package cmd

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/radar"
	"go.f110.dev/mono/go/radar/fetch"
	"go.f110.dev/mono/go/radar/wal"
)

func init() {
	register(&collectCommand{})
}

type collectCommand struct {
	commonFlags
	workDir string
	keepRaw bool
}

func (c *collectCommand) Command() *cli.Command {
	cmd := &cli.Command{
		Use:   "collect",
		Short: "Collect the rainfall images and store the values into the local WAL",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return c.run(ctx)
		},
	}
	c.commonFlags.register(cmd.Flags())
	cmd.Flags().String("workdir", "Directory of the WAL, the quarantine and the raw data").Var(&c.workDir).Required()
	cmd.Flags().Bool("keep-raw", "Keep the raw data until the georeference is verified").Var(&c.keepRaw).Default(true)
	return cmd
}

func (c *collectCommand) run(ctx context.Context) error {
	areas, err := c.areaList()
	if err != nil {
		return err
	}
	sources, err := c.sourceList(ctx)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	for _, s := range sources {
		l := &collectLoop{
			src:     s,
			areas:   areas,
			workDir: c.workDir,
			keepRaw: c.keepRaw,
			wal:     wal.New(filepath.Join(c.workDir, "wal"), s.fetcher.Interval()),
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.run(ctx)
		}()
	}
	wg.Wait()
	return nil
}

// collectLoop runs the loop of a single source. Each source has its own loop because the
// publishing interval and the rate limit are per source.
type collectLoop struct {
	src     source
	areas   []radar.Area
	workDir string
	keepRaw bool
	wal     *wal.WAL

	// since is the ValidTime that was processed last. Only the newer ones are fetched.
	since time.Time
}

func (l *collectLoop) run(ctx context.Context) {
	interval := pollInterval(l.src.fetcher.Interval())
	log := slogger.Log.With(slog.String("source", l.src.fetcher.ID().String()))
	log.Info("Start collecting", slog.Duration("poll_interval", interval))

	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		l.once(ctx, log)
		select {
		case <-t.C:
		case <-ctx.Done():
			log.Info("Stop collecting")
			return
		}
	}
}

func (l *collectLoop) once(ctx context.Context, log *slog.Logger) {
	times, err := l.src.fetcher.Latest(ctx, l.since)
	if err != nil {
		if ctx.Err() == nil {
			log.Warn("Failed to look up the latest frames", slogger.E(err))
		}
		return
	}

	units := l.units()
	for _, at := range times {
		if ctx.Err() != nil {
			return
		}
		// since only moves past a frame that was accounted for. A frame that failed to be fetched
		// stays a candidate so that the next tick retries it while it is inside the lookback
		// window. Advancing unconditionally would turn every transient 5xx into a permanent hole.
		if !l.fetchAll(ctx, log, at, units) {
			continue
		}
		if at.After(l.since) {
			l.since = at
		}
	}
}

// fetchAll handles every unit of the time and reports whether all of them were accounted for.
//
// The units are fetched in parallel. The rate limit of the shared client still caps the load on
// the counterpart, so the parallelism only decides how long a tick takes.
func (l *collectLoop) fetchAll(ctx context.Context, log *slog.Logger, at time.Time, units []string) bool {
	var wg sync.WaitGroup
	ok := make([]bool, len(units))
	for i, unit := range units {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok[i] = l.handle(ctx, log, at, unit)
		}()
	}
	wg.Wait()

	for _, v := range ok {
		if !v {
			return false
		}
	}
	return true
}

// units returns the fetching units that cover every area without duplication.
func (l *collectLoop) units() []string {
	seen := make(map[string]bool)
	var units []string
	for _, a := range l.areas {
		for _, u := range l.src.fetcher.Units(a) {
			if seen[u] {
				continue
			}
			seen[u] = true
			units = append(units, u)
		}
	}
	return units
}

// handle runs fetch, parse and append as a transaction of a single frame.
//
// When the parse fails the raw data is moved to the quarantine and nothing is written to the WAL.
// A slot that is not in the WAL becomes a missing frame when the day is finalized, so there is no
// need for an explicit record of the failure.
//
// The returned value reports whether the frame was accounted for. A frame that the source has not
// published is accounted for: it will never arrive, so waiting for it would stall the collector
// once the time leaves the lookback window. Everything else is a failure worth retrying.
func (l *collectLoop) handle(ctx context.Context, log *slog.Logger, at time.Time, unit string) bool {
	log = log.With(slog.String("unit", unit), slog.Time("valid_time", at))

	raw, err := l.src.fetcher.Fetch(ctx, at, unit)
	if err != nil {
		if errors.Is(err, fetch.ErrNotAvailable) {
			log.Debug("The frame is not available")
			return true
		}
		log.Warn("Failed to fetch", slogger.E(err))
		return false
	}

	frames, err := l.src.parser.Parse(raw)
	if err != nil {
		log.Error("Failed to parse. Moving the raw data to the quarantine", slogger.E(err))
		if err := l.save(filepath.Join(l.workDir, "quarantine"), raw); err != nil {
			log.Error("Failed to quarantine", slogger.E(err))
		}
		return false
	}

	if l.keepRaw {
		if err := l.save(filepath.Join(l.workDir, "raw"), raw); err != nil {
			log.Warn("Failed to keep the raw data", slogger.E(err))
		}
	}

	for _, v := range frames {
		if err := l.wal.Append(v); err != nil {
			log.Error("Failed to append to the WAL", slog.String("tile", v.Tile.String()), slogger.E(err))
			return false
		}
	}
	log.Debug("Collected", slog.Int("frames", len(frames)))
	return true
}

// save writes the raw data as it is. It is used for both the quarantine and the raw retention.
func (l *collectLoop) save(dir string, raw *radar.RawObject) error {
	jst := raw.ValidTime.In(radar.JST)
	p := filepath.Join(
		dir,
		raw.Source.String(),
		jst.Format("2006"), jst.Format("01"), jst.Format("02"),
		sanitizeUnit(raw.Unit)+"_"+jst.Format("1504")+extensionOf(raw.ContentType),
	)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return xerrors.WithStack(err)
	}
	if err := os.WriteFile(p, raw.Body, 0644); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

// pollInterval is half of the publishing interval so that the delay of the publishing is absorbed.
func pollInterval(d time.Duration) time.Duration {
	v := d / 2
	if v <= 0 {
		return time.Second
	}
	return v
}

// sanitizeUnit makes a unit usable as a part of a file name. The unit of XRAIN contains a slash.
func sanitizeUnit(unit string) string {
	out := []rune(unit)
	for i, r := range out {
		if r == '/' || r == os.PathSeparator {
			out[i] = '-'
		}
	}
	return string(out)
}

func extensionOf(contentType string) string {
	switch contentType {
	case "image/gif":
		return ".gif"
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	default:
		return ".bin"
	}
}
