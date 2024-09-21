package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

type GoRemoteCache struct {
	*fsm.FSM

	BaseDir string
	Prefix  string

	client      *storage.S3
	minioClient *storage.MinIO
}

const (
	stateInit fsm.State = iota
	stateStartListen
	stateShuttingDown
)

func NewGoRemoteCacheCmd() *GoRemoteCache {
	c := &GoRemoteCache{}
	c.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:         c.stateInit,
			stateStartListen:  c.startListen,
			stateShuttingDown: c.stateShuttingDown,
		},
		stateInit,
		stateShuttingDown,
	)
	c.FSM.CloseContext = func() (context.Context, context.CancelFunc) {
		return ctxutil.WithTimeout(context.Background(), 10*time.Second)
	}
	c.FSM.DisableErrorOutput = true

	return c
}

func (c *GoRemoteCache) stateInit(_ context.Context) (fsm.State, error) {
	endpoint := os.Getenv("S3_ENDPOINT")
	region := os.Getenv("S3_REGION")
	accessKey := os.Getenv("S3_ACCESS_KEY")
	secretAccessKey := os.Getenv("S3_SECRET_ACCESS_KEY")
	bucket := os.Getenv("S3_BUCKET")
	if endpoint == "" || region == "" || accessKey == "" || secretAccessKey == "" {
		return fsm.Error(xerrors.New("not enough credential"))
	}
	if bucket == "" {
		return fsm.Error(xerrors.New("bucket name is required"))
	}
	opts := storage.NewS3OptionToExternal(endpoint, region, accessKey, secretAccessKey)
	if _, ok := os.LookupEnv("S3_PATH_STYLE"); ok {
		opts.PathStyle = true
	}
	c.client = storage.NewS3(bucket, opts)

	c.Prefix = os.Getenv("CACHE_PREFIX")
	homedir, err := os.UserHomeDir()
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	c.BaseDir = filepath.Join(homedir, ".cache/go-remote-cache")
	if err := os.MkdirAll(c.BaseDir, 0755); err != nil {
		return fsm.Error(err)
	}

	return fsm.Next(stateStartListen)
}

func (c *GoRemoteCache) startListen(_ context.Context) (fsm.State, error) {
	jd := json.NewDecoder(bufio.NewReader(os.Stdin))
	w := bufio.NewWriter(os.Stdout)
	je := json.NewEncoder(w)

	res := &response{KnownCommands: []string{"get", "put", "close"}}
	if err := je.Encode(res); err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	if err := w.Flush(); err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}

	go func() {
		var mu sync.Mutex
		for {
			var req request
			if err := jd.Decode(&req); err == io.EOF {
				c.FSM.Shutdown()
				break
			} else if err != nil {
				c.FSM.Shutdown()
				break
			}
			logger.Log.Debug("Got request", zap.String("command", req.Command), zap.String("action_id", fmt.Sprintf("%x", req.ActionID)))
			if req.Command == "put" && req.BodySize > 0 {
				var buf []byte
				if err := jd.Decode(&buf); err != nil {
					c.FSM.Shutdown()
					break
				}
				logger.Log.Debug("Read body", zap.Int("len", len(buf)))
				req.body = buf
			}

			go func() {
				res, err := c.handleRequest(&req)
				if err != nil {
					logger.Log.Warn("handle error", logger.Error(err), logger.StackTrace(err))
				}
				if res == nil {
					logger.Log.Debug("empty response")
					return
				}
				mu.Lock()
				defer mu.Unlock()
				if err := je.Encode(res); err != nil {
					logger.Log.Error("Failed to encode json", logger.Error(err))
				}
				if err := w.Flush(); err != nil {
					logger.Log.Error("Failed to flush write buffer", logger.Error(err))
				}
			}()
		}
	}()

	return fsm.Wait()
}

func (c *GoRemoteCache) handleRequest(req *request) (*response, error) {
	res := &response{ID: req.ID}

	switch req.Command {
	case "get":
		r, err := c.client.Get(context.Background(), path.Join(c.Prefix, fmt.Sprintf("a-%x", req.ActionID)))
		if errors.Is(err, storage.ErrObjectNotFound) {
			res.Miss = true
			return res, nil
		}
		if err != nil {
			return nil, err
		}
		defer r.Body.Close()
		var e entry
		if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
			return nil, xerrors.WithStack(err)
		}

		outputFile := fmt.Sprintf("o-%s", e.OutputID)
		obj, err := c.client.Get(context.Background(), path.Join(c.Prefix, outputFile))
		if errors.Is(err, storage.ErrObjectNotFound) {
			res.Miss = true
			return res, nil
		}
		if err != nil {
			return nil, err
		}
		defer obj.Body.Close()

		size, err := saveFile(filepath.Join(c.BaseDir, outputFile), obj.Body)
		if err != nil {
			return nil, err
		}
		res.OutputID, err = hex.DecodeString(e.OutputID)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		res.Size = size
		res.DiskPath = filepath.Join(c.BaseDir, outputFile)
		res.TimeNanos = r.LastModified.UnixNano()
	case "put":
		actionFileName, objectFileName := fmt.Sprintf("a-%x", req.ActionID), fmt.Sprintf("o-%x", req.ObjectID)

		if err := c.client.Put(context.Background(), path.Join(c.Prefix, objectFileName), req.body); err != nil {
			return nil, err
		}

		e := &entry{
			OutputID:  fmt.Sprintf("%x", req.ObjectID),
			Size:      req.BodySize,
			TimeNanos: time.Now().UnixNano(),
		}
		buf, err := json.Marshal(e)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		err = c.client.Put(context.Background(), path.Join(c.Prefix, actionFileName), buf)
		if err != nil {
			return nil, err
		}

		logger.Log.Debug("Make local file", zap.Int("len", len(req.body)))
		size, err := saveFile(filepath.Join(c.BaseDir, objectFileName), bytes.NewReader(req.body))
		if err != nil {
			return nil, err
		}
		res.Size = size
		res.DiskPath = filepath.Join(c.BaseDir, objectFileName)
	case "close":
		logger.Log.Debug("Shutdown requested", zap.Int64("id", req.ID))
		c.Shutdown()
	default:
		return nil, xerrors.Definef("unknown command %s", req.Command).WithStack()
	}

	return res, nil
}

func (c *GoRemoteCache) stateShuttingDown(_ context.Context) (fsm.State, error) {
	logger.Log.Debug("Shutting down")
	return fsm.Finish()
}

func saveFile(outPath string, r io.Reader) (int64, error) {
	f, err := os.CreateTemp(filepath.Dir(outPath), filepath.Base(outPath)+".tmp")
	if err != nil {
		return -1, xerrors.WithStack(err)
	}
	size, err := io.Copy(f, r)
	if err != nil {
		return -1, xerrors.WithStack(err)
	}
	if err := f.Close(); err != nil {
		return -1, xerrors.WithStack(err)
	}
	if err := os.Rename(f.Name(), outPath); err != nil {
		return -1, xerrors.WithStack(err)
	}
	return size, nil
}

type request struct {
	ID       int64
	Command  string
	ActionID []byte `json:",omitempty"`
	ObjectID []byte `json:",omitempty"`
	BodySize int64  `json:",omitempty"`

	body []byte
}

type response struct {
	ID            int64
	Err           string   `json:",omitempty"`
	KnownCommands []string `json:",omitempty"`
	Miss          bool     `json:",omitempty"`
	OutputID      []byte   `json:",omitempty"`
	Size          int64    `json:",omitempty"`
	TimeNanos     int64    `json:",omitempty"`
	DiskPath      string   `json:",omitempty"`
}

type entry struct {
	OutputID  string `json:"output_id"`
	Size      int64  `json:"size"`
	TimeNanos int64  `json:"time_nanos"`
}

func goRemoteCache() error {
	c := NewGoRemoteCacheCmd()
	cmd := &cli.Command{
		Use: "go-remote-cache",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			logger.OutputStderr()
			return c.LoopContext(ctx)
		},
	}

	return cmd.Execute(os.Args)
}

func main() {
	if err := goRemoteCache(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
