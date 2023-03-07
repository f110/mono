package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"

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
				return nil, xerrors.New("unexpected minio_cluster_capacity_raw_total_bytes data: has multiple metrics")
			}
			if dto.Metric[0].Gauge == nil {
				return nil, xerrors.New("unexpected minio_cluster_capacity_raw_total_bytes data: the metrics is not gauge value")
			}
			capacity = dto.Metric[0].Gauge.GetValue()
		}
	}
	if capacity == 0 {
		return nil, xerrors.New("could not found capacity in metrics")
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
	t := time.NewTicker(gc.interval)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			if err := gc.gc(ctx, execute); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-gc.shutdown:
			return nil
		}
	}
}

func (gc *fifoObjectGarbageCollector) Shutdown() {
	select {
	case gc.shutdown <- struct{}{}:
	default:
	}
}

func (gc *fifoObjectGarbageCollector) gc(ctx context.Context, execute bool) error {
	logger.Log.Debug("Run GC")
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

func (c *fifoObjectGarbageCollectorCommand) Flags(fs *pflag.FlagSet) {
	fs.StringVar(&c.StorageEndpoint, "storage-endpoint", c.StorageEndpoint, "The endpoint of the object storage")
	fs.StringVar(&c.StorageRegion, "storage-region", c.StorageRegion, "The region name")
	fs.StringVar(&c.Bucket, "bucket", c.Bucket, "The bucket name that will be used")
	fs.StringVar(&c.StorageAccessKey, "storage-access-key", c.StorageAccessKey, "The access key for the object storage")
	fs.StringVar(&c.StorageSecretAccessKey, "storage-secret-access-key", c.StorageSecretAccessKey, "The secret access key for the object storage")
	fs.StringVar(&c.StorageCAFile, "storage-ca-file", "", "File path that contains CA certificate")
	fs.StringVar(&c.Prefix, "prefix", "", "Object prefix")
	fs.DurationVar(&c.Interval, "interval", 60*time.Minute, "GC interval")
	fs.IntVar(&c.MaxUsedPercent, "max-used-percent", 90, "GC threshold")
	fs.IntVar(&c.PurgePercent, "purge-percent", 10, "Will purge data size percent")
	fs.BoolVar(&c.OneShot, "one-shot", false, "Run GC")
	fs.BoolVar(&c.DryRun, "dry-run", false, "Dry run mode")
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
	if !c.OneShot {
		go func() {
			if err := c.gc.Run(ctx, !c.DryRun); err != nil {
				logger.Log.Error("GC error", logger.Error(err))
				c.FSM.Shutdown()
			}
		}()
	} else {
		if err := c.gc.gc(ctx, !c.DryRun); err != nil {
			return fsm.Error(err)
		}
		c.FSM.Shutdown()
	}
	return fsm.Wait()
}

func (c *fifoObjectGarbageCollectorCommand) shuttingDown(_ context.Context) (fsm.State, error) {
	c.gc.Shutdown()
	return fsm.Finish()
}

func FIFOObjectGarbageCollector() error {
	c := newFIFOObjectGarbageCollectorCommand()
	cmd := &cobra.Command{
		Use: "fifo-object-gc",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logger.Init(); err != nil {
				return err
			}
			return c.LoopContext(cmd.Context())
		},
	}
	c.Flags(cmd.Flags())
	logger.Flags(cmd.Flags())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cmd.ExecuteContext(ctx)
}

func main() {
	if err := FIFOObjectGarbageCollector(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
