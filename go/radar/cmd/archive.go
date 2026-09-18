package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"time"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/radar"
	"go.f110.dev/mono/go/radar/amesh"
	"go.f110.dev/mono/go/radar/dayfile"
	"go.f110.dev/mono/go/radar/wal"
	"go.f110.dev/mono/go/radar/xrain"
	"go.f110.dev/mono/go/storage"
)

func init() {
	register(&archiveCommand{})
}

// keyPrefix is the prefix of the object key. The version allows the layout to change later.
const keyPrefix = "radar/v1"

// objectStore is the part of go/storage that the archiver uses.
// The abstraction of the object storage already exists so nothing new is defined here.
type objectStore interface {
	Name() string
	PutReader(ctx context.Context, name string, r io.Reader) error
	Get(ctx context.Context, name string) (*storage.Object, error)
}

type storageFlags struct {
	bucket    string
	endpoint  string
	region    string
	accessKey string
	secretKey string
}

func (f *storageFlags) register(fs *cli.FlagSet) {
	fs.String("bucket", "Bucket of the object storage").Var(&f.bucket).Required()
	fs.String("storage-endpoint", "Endpoint of the object storage").Var(&f.endpoint)
	fs.String("storage-region", "Region of the object storage").Var(&f.region)
	fs.String("storage-access-key", "Access key of the object storage").Var(&f.accessKey)
	fs.String("storage-secret-access-key", "Secret access key of the object storage").Var(&f.secretKey)
}

func (f *storageFlags) newStore() objectStore {
	opt := storage.NewS3OptionToExternal(f.endpoint, f.region, f.accessKey, f.secretKey)
	opt.PathStyle = true
	return storage.NewS3(f.bucket, opt)
}

type archiveCommand struct {
	storageFlags
	workDir string
	date    string
}

func (c *archiveCommand) Command() *cli.Command {
	cmd := &cli.Command{
		Use:   "archive",
		Short: "Build the daily container from the WAL and upload it",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			date, err := parseDate(c.date)
			if err != nil {
				return err
			}
			return archive(ctx, c.workDir, date, c.newStore())
		},
	}
	cmd.Flags().String("workdir", "Directory of the WAL").Var(&c.workDir).Required()
	cmd.Flags().String("date", "Date to finalize in JST. The previous day when it is not specified").Var(&c.date)
	c.storageFlags.register(cmd.Flags())
	return cmd
}

// parseDate returns the JST date to finalize. An empty string means the previous day.
func parseDate(s string) (time.Time, error) {
	if s == "" {
		return radar.DateOf(time.Now()).AddDate(0, 0, -1), nil
	}
	d, err := time.ParseInLocation("2006-01-02", s, radar.JST)
	if err != nil {
		return time.Time{}, xerrors.WithStack(err)
	}
	return d, nil
}

func archive(ctx context.Context, workDir string, date time.Time, store objectStore) error {
	dir := filepath.Join(workDir, "wal")
	// The interval is only used to list the files so any positive value works here.
	entries, err := wal.New(dir, time.Minute).List(date)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		slogger.Log.Info("There is no WAL to archive", slog.String("date", date.Format("2006-01-02")))
		return nil
	}

	var failed int
	for _, e := range entries {
		if err := archiveOne(ctx, dir, e, store); err != nil {
			slogger.Log.Error("Failed to archive",
				slog.String("source", e.Source.String()), slog.String("tile", e.Tile.String()), slogger.E(err))
			failed++
		}
	}
	if failed > 0 {
		return xerrors.NewfWithStack("radar: failed to archive %d of %d", failed, len(entries))
	}
	return nil
}

func archiveOne(ctx context.Context, dir string, e wal.Entry, store objectStore) error {
	key := objectKey(e.Source, e.Tile, e.Date)
	log := slogger.Log.With(slog.String("key", key))

	// The archiver is idempotent. When the object is already there the WAL was either removed or
	// the previous run finished after the upload.
	if obj, err := store.Get(ctx, key); err == nil {
		// Object.Body is a live HTTP response body. Closing it returns the connection to the pool.
		obj.Body.Close()
		log.Info("The object already exists. Removing the WAL")
		return wal.New(dir, intervalOf(e.Source)).Remove(e.Source, e.Tile, e.Date)
	} else if !errors.Is(err, storage.ErrObjectNotFound) {
		return err
	}

	interval := intervalOf(e.Source)
	w := wal.New(dir, interval)
	frames, err := w.Load(e.Source, e.Tile, e.Date)
	if err != nil {
		return err
	}
	if len(frames) == 0 {
		log.Info("The WAL is empty. Nothing to archive")
		return nil
	}

	currentScale, err := radar.CurrentScale(e.Source)
	if err != nil {
		return err
	}
	h := dayfile.Header{
		Source: e.Source,
		Tile:   e.Tile,
		// The scale version of the current build is used. The WAL has to be drained before the
		// scale table of the source is changed, otherwise the frames of the old table would be
		// recorded with the new version.
		ScaleVersion: currentScale.Version,
		DateJST:      e.Date,
		TileSize:     radar.TileSize,
		Interval:     interval,
		FrameCount:   radar.FrameCount(interval),
		Finalized:    true,
	}
	writer := dayfile.NewWriter(h)
	// written counts what actually went into the container. Taking it from len(frames) would break
	// the verification when the WAL holds a slot outside of the day, because such a slot is never
	// written and the mismatch would keep the WAL forever.
	var written int
	for slot := 0; slot < h.FrameCount; slot++ {
		v, ok := frames[slot]
		if !ok {
			continue
		}
		if err := writer.SetFrame(slot, v); err != nil {
			return err
		}
		written++
	}
	if written != len(frames) {
		log.Warn("The WAL has slots outside of the day", slog.Int("dropped", len(frames)-written))
	}

	buf := &bytes.Buffer{}
	if _, err := writer.WriteTo(buf); err != nil {
		return err
	}
	if err := store.PutReader(ctx, key, bytes.NewReader(buf.Bytes())); err != nil {
		return xerrors.WithStack(err)
	}

	if err := verify(ctx, store, key, written); err != nil {
		return err
	}
	log.Info("Archived", slog.Int("frames", written), slog.Int("bytes", buf.Len()))
	return w.Remove(e.Source, e.Tile, e.Date)
}

// verify reads the uploaded object back and decodes every slot.
// The WAL is removed only after this succeeds.
func verify(ctx context.Context, store objectStore, key string, want int) error {
	obj, err := store.Get(ctx, key)
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer obj.Body.Close()

	r, err := dayfile.NewReader(obj.Body)
	if err != nil {
		return err
	}
	var got int
	for slot, kind := range r.Kinds() {
		if kind == dayfile.KindMissing {
			continue
		}
		if _, err := r.Frame(slot); err != nil {
			return err
		}
		got++
	}
	if got != want {
		return xerrors.NewfWithStack("radar: %s: the number of the frames does not match: %d != %d", key, got, want)
	}
	return nil
}

// objectKey returns the key of the daily container.
//
//	radar/v1/{source}/{z}/{x}/{y}/{YYYY}/{MM}/{DD}.rac.zst
func objectKey(source radar.SourceID, tile radar.Tile, date time.Time) string {
	d := radar.DateOf(date)
	return fmt.Sprintf("%s/%s/%d/%d/%d/%s.rac.zst", keyPrefix, source.String(), tile.Z, tile.X, tile.Y, d.Format("2006/01/02"))
}

// intervalOf returns the publishing interval of the source.
//
// The interval is a property of the source itself, so the archiver does not need the whole runtime
// configuration of the fetchers to know it.
func intervalOf(source radar.SourceID) time.Duration {
	switch source {
	case radar.SourceAmesh:
		return amesh.Interval
	case radar.SourceXRAIN:
		return xrain.Interval
	default:
		return time.Minute
	}
}
