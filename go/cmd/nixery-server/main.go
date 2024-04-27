// Copyright 2022 The TVL Contributors
// SPDX-License-Identifier: Apache-2.0

// The nixery server implements a container registry that transparently builds
// container images based on Nix derivations.
//
// The Nix derivation used for image creation is responsible for creating
// objects that are compatible with the registry API. The targeted registry
// protocol is currently Docker's.
//
// When an image is requested, the required contents are parsed out of the
// request and a Nix-build is initiated that eventually responds with the
// manifest as well as information linking each layer digest to a local
// filesystem path.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/google/nixery/builder"
	"github.com/google/nixery/config"
	"github.com/google/nixery/layers"
	mf "github.com/google/nixery/manifest"
	nstorage "github.com/google/nixery/storage"
	"github.com/im7mortal/kmutex"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/nixery"
)

// ManifestMediaType is the Content-Type used for the manifest itself. This
// corresponds to the "Image Manifest V2, Schema 2" described on this page:
//
// https://docs.docker.com/registry/spec/manifest-v2-2/
const manifestMediaType string = "application/vnd.docker.distribution.manifest.v2+json"

// Regexes matching the V2 Registry API routes. This only includes the
// routes required for serving images, since pushing and other such
// functionality is not available.
var (
	manifestRegex = regexp.MustCompile(`^/v2/([\w|\-|\.|\_|\/]+)/manifests/([\w|\-|\.|\_]+)$`)
	blobRegex     = regexp.MustCompile(`^/v2/([\w|\-|\.|\_|\/]+)/(blobs|manifests)/sha256:(\w+)$`)
)

// Downloads the popularity information for the package set from the
// URL specified in Nixery's configuration.
func downloadPopularity(ctx context.Context, url string) (layers.Popularity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, xerrors.Definef("popularity download from '%s' returned status: %s", url, res.Status).WithStack()
	}

	var pop layers.Popularity
	if err := json.NewDecoder(res.Body).Decode(&pop); err != nil {
		return nil, xerrors.WithStack(err)
	}

	return pop, nil
}

// Error format corresponding to the registry protocol V2 specification. This
// allows feeding back errors to clients in a way that can be presented to
// users.
type registryError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type registryErrors struct {
	Errors []registryError `json:"errors"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	err := registryErrors{
		Errors: []registryError{
			{Code: code, Message: message},
		},
	}
	buf, _ := json.Marshal(err)

	w.WriteHeader(status)
	w.Header().Add("Content-Type", "application/json")
	w.Write(buf)
}

type registryHandler struct {
	state *builder.State
}

// Serve a manifest by tag, building it via Nix and populating caches
// if necessary.
func (h *registryHandler) serveManifestTag(w http.ResponseWriter, req *http.Request, name string, tag string) {
	logger.Log.Debug("Requesting image manifest", zap.String("image", name), zap.String("tag", tag))

	image := builder.ImageFromName(name, tag)
	buildResult, err := builder.BuildImage(req.Context(), h.state, &image)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "UNKNOWN", "image build failure")

		logger.Log.Error("Failed to build image manifest", zap.String("image", name), zap.String("tag", tag))
		return
	}

	// Some error types have special handling, which is applied
	// here.
	if buildResult.Error == "not_found" {
		s := fmt.Sprintf("Could not find Nix packages: %v", buildResult.Pkgs)
		writeError(w, http.StatusNotFound, "MANIFEST_UNKNOWN", s)

		logger.Log.Warn("Could not find nix packages", zap.String("image", name), zap.String("tag", tag), zap.Strings("packages", buildResult.Pkgs))
		return
	}

	// This marshaling error is ignored because we know that this
	// field represents valid JSON data.
	manifest, _ := json.Marshal(buildResult.Manifest)
	w.Header().Add("Content-Type", manifestMediaType)

	// The manifest needs to be persisted to the blob storage (to become
	// available for clients that fetch manifests by their hash, e.g.
	// containerd) and served to the client.
	//
	// Since we have no stable key to address this manifest (it may be
	// uncacheable, yet still addressable by blob) we need to separate
	// out the hashing, uploading and serving phases. The latter is
	// especially important as clients may start to fetch it by digest
	// as soon as they see a response.
	sha256sum := fmt.Sprintf("%x", sha256.Sum256(manifest))
	path := "layers/" + sha256sum

	_, _, err = h.state.Storage.Persist(req.Context(), path, mf.ManifestType, func(w io.Writer) (string, int64, error) {
		// We already know the hash, so no additional hash needs to be
		// constructed here.
		written, err := w.Write(manifest)
		return sha256sum, int64(written), err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "MANIFEST_UPLOAD", "could not upload manifest to blob store")

		logger.Log.Error("Could not upload manifest", zap.String("image", name), zap.String("tag", tag))
		return
	}

	w.Write(manifest)
}

// serveBlob serves a blob from storage by digest
func (h *registryHandler) serveBlob(w http.ResponseWriter, req *http.Request, blobType, digest string) {
	if err := h.state.Storage.Serve(digest, req, w); err != nil {
		logger.Log.Error("failed to serve blob", logger.Error(err), zap.String("type", blobType), zap.String("digest", digest))
	}
}

// ServeHTTP dispatches HTTP requests to the matching handlers.
func (h *registryHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Acknowledge that we speak V2 with an empty response
	if req.RequestURI == "/v2/" {
		return
	}

	// Build & serve a manifest by tag
	manifestMatches := manifestRegex.FindStringSubmatch(req.RequestURI)
	if len(manifestMatches) == 3 {
		h.serveManifestTag(w, req, manifestMatches[1], manifestMatches[2])
		return
	}

	// Serve a blob by digest
	layerMatches := blobRegex.FindStringSubmatch(req.RequestURI)
	if len(layerMatches) == 4 {
		h.serveBlob(w, req, layerMatches[2], layerMatches[3])
		return
	}

	logger.Log.Info("Unsupported registry route", zap.String("uri", req.RequestURI))
	w.WriteHeader(http.StatusNotFound)
}

type nixeryServerCmd struct {
	*fsm.FSM

	Listen                 string
	WebDir                 string
	Storage                string
	StorageEndpoint        string
	StorageRegion          string
	StorageAccessKey       string
	StorageSecretAccessKey string
	StoragePath            string
	Bucket                 string
	StorageCAFile          string

	state  *builder.State
	server *http.Server
}

const (
	stateInit fsm.State = iota
	stateStartServer
	stateShuttingDown
)

func newNixeryServerCmd() *nixeryServerCmd {
	c := &nixeryServerCmd{}
	c.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:         c.init,
			stateStartServer:  c.startServer,
			stateShuttingDown: c.shuttingDown,
		},
		stateInit,
		stateShuttingDown,
	)
	c.FSM.CloseContext = func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), 10*time.Second)
	}
	return c
}

func (c *nixeryServerCmd) Flags(fs *pflag.FlagSet) {
	fs.StringVar(&c.Listen, "listen", ":8381", "Listen addr")
	fs.StringVar(&c.WebDir, "web-dir", "", "Directory path for static assets")
	fs.StringVar(&c.Storage, "storage", "s3", "The name of the storage")
	fs.StringVar(&c.StorageEndpoint, "storage-endpoint", c.StorageEndpoint, "The endpoint of the object storage")
	fs.StringVar(&c.StorageRegion, "storage-region", c.StorageRegion, "The region name")
	fs.StringVar(&c.Bucket, "bucket", c.Bucket, "The bucket name that will be used")
	fs.StringVar(&c.StorageAccessKey, "storage-access-key", c.StorageAccessKey, "The access key for the object storage")
	fs.StringVar(&c.StorageSecretAccessKey, "storage-secret-access-key", c.StorageSecretAccessKey, "The secret access key for the object storage")
	fs.StringVar(&c.StorageCAFile, "storage-ca-file", "", "File path that contains CA certificate")
	fs.StringVar(&c.StoragePath, "storage-path", "", "The directory path")
}

func (c *nixeryServerCmd) init(ctx context.Context) (fsm.State, error) {
	pkgSource := &nixery.GitSource{}
	cfg := config.Config{
		Pkgs:    pkgSource,
		Timeout: "60",
		PopUrl:  os.Getenv("NIX_POPULARITY_URL"),
	}

	var s nstorage.Backend
	switch c.Storage {
	case "s3":
		s = nixery.NewS3Storage(c.StorageEndpoint, c.StorageRegion, c.StorageAccessKey, c.StorageSecretAccessKey, c.Bucket, c.StorageCAFile)
	case "fs":
		os.Setenv("STORAGE_PATH", c.StoragePath)
		b, err := nstorage.NewFSBackend()
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		os.Unsetenv("STORAGE_PATH")
		s = b
	}
	cache, err := builder.NewCache()
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}

	var pop layers.Popularity
	if cfg.PopUrl != "" {
		pop, err = downloadPopularity(ctx, cfg.PopUrl)
		if err != nil {
			return fsm.Error(err)
		}
	}

	c.state = &builder.State{
		Cache:       &cache,
		Cfg:         cfg,
		Pop:         pop,
		Storage:     s,
		UploadMutex: kmutex.New(),
	}

	return fsm.Next(stateStartServer)
}

func (c *nixeryServerCmd) startServer(_ context.Context) (fsm.State, error) {
	mux := http.NewServeMux()
	// All /v2/ requests belong to the registry handler.
	mux.Handle("/v2/", &registryHandler{
		state: c.state,
	})

	// All other roots are served by the static file server.
	webDir := http.Dir(c.WebDir)
	mux.Handle("/", http.FileServer(webDir))
	c.server = &http.Server{
		Addr:    c.Listen,
		Handler: mux,
	}
	go func() {
		logger.Log.Info("Start nixery", zap.String("addr", c.server.Addr))
		if err := c.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Error("http server returns error", logger.Error(err))
		}
	}()
	return fsm.Wait()
}

func (c *nixeryServerCmd) shuttingDown(ctx context.Context) (fsm.State, error) {
	if c.server != nil {
		logger.Log.Debug("Shutting down http server")
		if err := c.server.Shutdown(ctx); err != nil {
			logger.Log.Error("Failed to shutting down server", logger.Error(err))
		}
	}

	return fsm.Finish()
}

func main() {
	serverCmd := newNixeryServerCmd()
	cmd := &cobra.Command{
		Use: "nixery-server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := logger.Init(); err != nil {
				return err
			}
			logger.HijackStandardLogrus()
			cmd.SilenceUsage = true
			return serverCmd.LoopContext(cmd.Context())
		},
	}
	logger.Flags(cmd.Flags())
	serverCmd.Flags(cmd.Flags())
	cmd.SilenceErrors = true

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
