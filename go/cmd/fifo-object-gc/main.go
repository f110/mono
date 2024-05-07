package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"time"

	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

type fifoObjectGarbageCollector struct {
	client         *storage.S3
	prefix         string
	interval       time.Duration
	capacity       float64
	maxUsedPercent float64
	purgePercent   float64
	shutdown       chan struct{}
}

func newFIFOObjectGarbageCollector(ctx context.Context, client *storage.S3, prefix string, interval time.Duration, maxUsedPercent, purgePercent int) (*fifoObjectGarbageCollector, error) {
	// Get cluster information from metrics endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/minio/v2/metrics/cluster", client.Endpoint()), nil)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	req.Header.Add("Accept", string(expfmt.FmtProtoDelim))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	defer res.Body.Close()
	d := expfmt.NewDecoder(res.Body, expfmt.FmtProtoDelim)
	var dto io_prometheus_client.MetricFamily
	var capacity float64
	for {
		err := d.Decode(&dto)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		if dto.GetName() == "minio_cluster_capacity_raw_total_bytes" {
			if len(dto.Metric) != 1 {
				return nil, xerrors.Define("unexpected minio_cluster_capacity_raw_total_bytes data: has multiple metrics").WithStack()
			}
			if dto.Metric[0].Gauge == nil {
				return nil, xerrors.Define("unexpected minio_cluster_capacity_raw_total_bytes data: the metrics is not gauge value").WithStack()
			}
			capacity = dto.Metric[0].Gauge.GetValue()
		}
	}
	if capacity == 0 {
		return nil, xerrors.Define("could not found capacity in metrics").WithStack()
	}

	logger.Log.Debug("Got cluster info from metrics endpoint", zap.Float64("capacity", capacity))
	return &fifoObjectGarbageCollector{
		client:         client,
		prefix:         prefix,
		interval:       interval,
		capacity:       capacity,
		maxUsedPercent: float64(maxUsedPercent) / 100,
		purgePercent:   float64(purgePercent) / 100,
		shutdown:       make(chan struct{}),
	}, nil
}

func (gc *fifoObjectGarbageCollector) Run(ctx context.Context, execute bool) error {
	defer logger.Log.Info("Finish garbage collector")
	t := time.NewTicker(gc.interval)
	defer t.Stop()

	go func() {
		if err := gc.startInvokeServer(execute); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Error("Something happen", zap.Error(err))
		}
	}()
	logger.Log.Info("Start garbage collector")
	for {
		select {
		case <-t.C:
			c, cancel := context.WithTimeout(ctx, gc.interval/2)
			if err := gc.gc(c, execute); err != nil {
				cancel()
				return err
			}
			cancel()
		case <-ctx.Done():
			return ctx.Err()
		case <-gc.shutdown:
			return nil
		}
	}
}

func (gc *fifoObjectGarbageCollector) startInvokeServer(execute bool) error {
	s := &http.Server{
		Addr: ":8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), gc.interval/2)
				defer cancel()
				if err := gc.gc(ctx, execute); err != nil {
					logger.Log.Error("Failed garbage collecting", zap.Error(err))
				}
			}()
		}),
	}

	logger.Log.Info("Start invoke server", zap.String("addr", s.Addr))
	return s.ListenAndServe()
}

func (gc *fifoObjectGarbageCollector) Shutdown() {
	select {
	case gc.shutdown <- struct{}{}:
	default:
	}
}

func (gc *fifoObjectGarbageCollector) gc(ctx context.Context, execute bool) error {
	logger.Log.Debug("Run GC")
	defer logger.Log.Debug("Finish GC")
	objects, err := gc.client.List(ctx, gc.prefix)
	if err != nil {
		return err
	}
	logger.Log.Debug("Found objects", zap.Int("num", len(objects)))

	totalSize := enumerable.Sum(objects, func(obj *storage.Object) int64 { return obj.Size })
	maxUsedSize := int64(gc.capacity * gc.maxUsedPercent)
	if totalSize < maxUsedSize {
		logger.Log.Debug("Current used size is less than max used size", zap.Int64("current", totalSize), zap.Int64("max_used_size", maxUsedSize), zap.Float64("usage", float64(totalSize)/float64(maxUsedSize)))
		return nil
	}

	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.Before(objects[j].LastModified)
	})
	deleteSize := int64(float64(totalSize-maxUsedSize) * gc.purgePercent)
	var size int64
	var deleteObjects []*storage.Object
	for i := 0; i < len(objects); i++ {
		size += objects[i].Size
		if size > deleteSize {
			deleteObjects = objects[:i+1]
			break
		}
	}

	logger.Log.Info("Delete some objects", zap.Int("num", len(deleteObjects)), zap.Int64("size", size))
	for _, v := range deleteObjects {
		if execute {
			if err := gc.client.Delete(ctx, v.Name); err != nil {
				return err
			}
		} else {
			logger.Log.Info("Delete", zap.String("name", v.Name), zap.Int64("size", v.Size), zap.Time("created_at", v.LastModified))
		}
	}

	return nil
}

type fifoObjectGarbageCollectorCommand struct {
	*fsm.FSM

	StorageEndpoint        string
	StorageRegion          string
	Bucket                 string
	StorageAccessKey       string
	StorageSecretAccessKey string
	StorageCAFile          string
	Prefix                 string
	Interval               time.Duration
	MaxUsedPercent         int
	PurgePercent           int
	OneShot                bool
	DryRun                 bool

	client *storage.S3
	gc     *fifoObjectGarbageCollector
}

const (
	stateInit fsm.State = iota
	stateStartGC
	stateShuttingDown
)

func newFIFOObjectGarbageCollectorCommand() *fifoObjectGarbageCollectorCommand {
	c := &fifoObjectGarbageCollectorCommand{}
	c.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:         c.init,
			stateStartGC:      c.startGC,
			stateShuttingDown: c.shuttingDown,
		},
		stateInit,
		stateShuttingDown,
	)

	return c
}

func (c *fifoObjectGarbageCollectorCommand) Flags(fs *cli.FlagSet) {
	fs.String("storage-endpoint", "The endpoint of the object storage").Var(&c.StorageEndpoint)
	fs.String("storage-region", "The region name").Var(&c.StorageRegion)
	fs.String("bucket", "The bucket name that will be used").Var(&c.Bucket)
	fs.String("storage-access-key", "The access key for the object storage").Var(&c.StorageAccessKey)
	fs.String("storage-secret-access-key", "The secret access key for the object storage").Var(&c.StorageSecretAccessKey)
	fs.String("storage-ca-file", "File path that contains CA certificate").Var(&c.StorageCAFile)
	fs.String("prefix", "Object prefix").Var(&c.Prefix)
	fs.Duration("interval", "GC interval").Var(&c.Interval).Default(60 * time.Minute)
	fs.Int("max-used-percent", "GC threshold").Var(&c.MaxUsedPercent).Default(90)
	fs.Int("purge-percent", "Will purge data size percent").Var(&c.PurgePercent).Default(10)
	fs.Bool("one-shot", "Run GC").Var(&c.OneShot)
	fs.Bool("dry-run", "Dry run mode").Var(&c.DryRun)
}

func (c *fifoObjectGarbageCollectorCommand) init(ctx context.Context) (fsm.State, error) {
	opt := storage.NewS3OptionToExternal(c.StorageEndpoint, c.StorageRegion, c.StorageAccessKey, c.StorageSecretAccessKey)
	opt.PathStyle = true
	s3Client := storage.NewS3(c.Bucket, opt)
	c.client = s3Client
	gc, err := newFIFOObjectGarbageCollector(ctx, c.client, c.Prefix, c.Interval, c.MaxUsedPercent, c.PurgePercent)
	if err != nil {
		return fsm.Error(err)
	}
	c.gc = gc

	return fsm.Next(stateStartGC)
}

func (c *fifoObjectGarbageCollectorCommand) startGC(ctx context.Context) (fsm.State, error) {
	if c.OneShot {
		if err := c.gc.gc(ctx, !c.DryRun); err != nil {
			return fsm.Error(err)
		}
		c.FSM.Shutdown()
	} else {
		go func() {
			if err := c.gc.Run(ctx, !c.DryRun); err != nil {
				logger.Log.Error("GC error", logger.Error(err))
				c.FSM.Shutdown()
			}
		}()
	}
	return fsm.Wait()
}

func (c *fifoObjectGarbageCollectorCommand) shuttingDown(_ context.Context) (fsm.State, error) {
	c.gc.Shutdown()
	return fsm.Finish()
}

func FIFOObjectGarbageCollector() error {
	c := newFIFOObjectGarbageCollectorCommand()
	cmd := &cli.Command{
		Use: "fifo-object-gc",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return c.LoopContext(ctx)
		},
	}
	c.Flags(cmd.Flags())

	return cmd.Execute(os.Args)
}

func main() {
	if err := FIFOObjectGarbageCollector(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
