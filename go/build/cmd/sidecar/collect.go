package sidecar

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/layout"
	"go.f110.dev/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.f110.dev/mono/go/bazel/buildeventstream"
	"go.f110.dev/mono/go/build/api"
	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/logger/slogger"
)

const markerSuffix = ".cd_artifact.json"

// markerFile is the JSON the cd_artifact rule writes for each target.
type markerFile struct {
	Target    string           `json:"target"`
	Artifacts []markerArtifact `json:"artifacts"`
}

type markerArtifact struct {
	Type       string `json:"type"` // container_image | binary | k8s_manifest
	Path       string `json:"path"` // exec-root relative
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Format     string `json:"format"` // oci_layout for container images
}

// artifactEntry is a marker artifact resolved to an absolute path (and later its
// hash).
type artifactEntry struct {
	Type       string
	AbsPath    string
	Repository string
	Tag        string
	Name       string
	Hash       string
}

// collectResult is one line of the summary the sidecar writes to stdout.
type collectResult struct {
	Type       string `json:"type"`
	Name       string `json:"name"`
	Hash       string `json:"hash"`
	Repository string `json:"repository"`
	Reference  string `json:"reference,omitempty"`
	Status     string `json:"status"` // uploaded | exists | error
}

// CollectCommand discovers cd_artifact markers in the Bazel output tree, hashes
// each artifact, asks the builder which are new and uploads those. It runs only
// on trunk builds. See docs/working/artifact-collection.md.
type CollectCommand struct {
	outputBase      string
	eventBinaryFile string
	builderAddr     string
	taskID          int
	startUpTimeout  time.Duration
}

func NewCollectCommand() *CollectCommand {
	return &CollectCommand{}
}

func (c *CollectCommand) Name() string {
	return "collect"
}

func (c *CollectCommand) SetFlags(fs *cli.FlagSet) {
	fs.String("output-base", "The Bazel output base shared with the main container").Var(&c.outputBase)
	fs.String("event-binary-file", "The path of the Build Event Protocol file").Var(&c.eventBinaryFile)
	fs.String("builder-addr", "The gRPC address of the builder API").Var(&c.builderAddr)
	fs.Int("task-id", "The id of the task that produced the artifacts").Var(&c.taskID)
	fs.Duration("startup-timeout", "Time to wait for the event file to appear").Var(&c.startUpTimeout)
}

func (c *CollectCommand) Run(ctx context.Context) error {
	// Only collect from a successful build. The BEP doubles as the "build
	// finished" signal since the sidecar runs alongside the main container.
	success, err := c.waitBuildSuccess()
	if err != nil {
		return err
	}
	if !success {
		slogger.Log.Info("Build did not succeed; skip artifact collection")
		return nil
	}

	entries, err := scanMarkers(c.outputBase)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		slogger.Log.Info("No cd_artifact markers found")
		return nil
	}
	for _, e := range entries {
		h, err := hashArtifact(e)
		if err != nil {
			return err
		}
		e.Hash = h
	}

	conn, err := grpc.NewClient(c.builderAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer conn.Close()
	client := api.NewAPIClient(conn)

	refs := make([]*api.ArtifactRef, 0, len(entries))
	for _, e := range entries {
		refs = append(refs, buildArtifactRef(int32(c.taskID), e))
	}
	req := &api.RequestSyncArtifacts{}
	req.SetTaskId(int32(c.taskID))
	req.SetArtifacts(refs)
	resp, err := client.SyncArtifacts(ctx, req)
	if err != nil {
		return xerrors.WithStack(err)
	}
	newHashes := make(map[string]struct{}, len(resp.GetNewHashes()))
	for _, h := range resp.GetNewHashes() {
		newHashes[h] = struct{}{}
	}

	summary := make([]collectResult, 0, len(entries))
	for _, e := range entries {
		res := collectResult{Type: e.Type, Name: e.Name, Hash: e.Hash, Repository: e.Repository}
		if _, ok := newHashes[e.Hash]; !ok {
			res.Status = "exists"
			summary = append(summary, res)
			continue
		}
		// Upload failures are soft: they must not fail the Job (only the main
		// container's exit code does). Record and continue.
		reference, err := c.upload(ctx, client, e)
		if err != nil {
			slogger.Log.Warn("Failed to upload artifact", slog.String("name", e.Name), slogger.E(err))
			res.Status = "error"
			summary = append(summary, res)
			continue
		}
		res.Status = "uploaded"
		res.Reference = reference
		summary = append(summary, res)
	}

	return json.NewEncoder(os.Stdout).Encode(summary)
}

// waitBuildSuccess waits for the BEP file to appear and reports whether the build
// finished successfully.
func (c *CollectCommand) waitBuildSuccess() (bool, error) {
	f, r, err := waitAndOpenBEP(c.eventBinaryFile, c.startUpTimeout)
	if err != nil {
		return false, err
	}
	defer f.Close()

	success := false
	err = iterateBuildEvents(r, func(msg *buildeventstream.BuildEvent) error {
		if bf := msg.GetFinished(); bf != nil {
			// exit code 0 == success; OverallSuccess is deprecated.
			success = bf.GetExitCode().GetCode() == 0
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return success, nil
}

// scanMarkers walks outputBase for cd_artifact markers and resolves each artifact
// to an absolute path. Artifact paths are exec-root relative; the exec root is
// derived from the marker's own location (it always lives under bazel-out).
func scanMarkers(outputBase string) ([]*artifactEntry, error) {
	var entries []*artifactEntry
	err := filepath.WalkDir(outputBase, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable subtrees
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), markerSuffix) {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return xerrors.WithStack(err)
		}
		var mf markerFile
		if err := json.Unmarshal(b, &mf); err != nil {
			return xerrors.Definef("failed to parse marker %s: %v", p, err).WithStack()
		}
		execRoot, err := execRootFromMarker(p)
		if err != nil {
			return err
		}
		for _, a := range mf.Artifacts {
			abs := a.Path
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(execRoot, a.Path)
			}
			entries = append(entries, &artifactEntry{
				Type:       a.Type,
				AbsPath:    abs,
				Repository: a.Repository,
				Tag:        a.Tag,
				Name:       filepath.Base(a.Path),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// execRootFromMarker derives the exec root from a marker's absolute path. Markers
// are build outputs, so their path always contains a /bazel-out/ segment; the
// exec root is everything before it.
func execRootFromMarker(markerPath string) (string, error) {
	before, _, found := strings.Cut(markerPath, "/bazel-out/")
	if !found {
		return "", xerrors.Definef("marker is not under bazel-out: %s", markerPath).WithStack()
	}
	return before, nil
}

func hashArtifact(e *artifactEntry) (string, error) {
	switch e.Type {
	case "container_image":
		return ociImageDigest(e.AbsPath)
	case "binary", "k8s_manifest":
		return fileSHA256(e.AbsPath)
	default:
		return "", xerrors.Definef("unknown artifact type: %s", e.Type).WithStack()
	}
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", xerrors.WithStack(err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ociImageDigest returns the image manifest digest (e.g. "sha256:...") of the
// single image in an OCI layout directory.
func ociImageDigest(dir string) (string, error) {
	idx, err := layout.ImageIndexFromPath(dir)
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	m, err := idx.IndexManifest()
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	for _, desc := range m.Manifests {
		if desc.MediaType.IsImage() {
			return desc.Digest.String(), nil
		}
	}
	return "", xerrors.Definef("no image manifest found in the OCI layout: %s", dir).WithStack()
}

func buildArtifactRef(_ int32, e *artifactEntry) *api.ArtifactRef {
	r := &api.ArtifactRef{}
	r.SetType(apiArtifactType(e.Type))
	r.SetHash(e.Hash)
	r.SetName(e.Name)
	r.SetRepository(e.Repository)
	r.SetTag(e.Tag)
	return r
}

func apiArtifactType(t string) api.ArtifactType {
	switch t {
	case "container_image":
		return api.ArtifactType_ARTIFACT_TYPE_CONTAINER_IMAGE
	case "k8s_manifest":
		return api.ArtifactType_ARTIFACT_TYPE_K8S_MANIFEST
	default:
		return api.ArtifactType_ARTIFACT_TYPE_BINARY
	}
}

// upload streams a single artifact to the builder. For container images the
// content is a tar of the OCI layout; for files it is the raw bytes.
func (c *CollectCommand) upload(ctx context.Context, client api.APIClient, e *artifactEntry) (string, error) {
	stream, err := client.UploadArtifact(ctx)
	if err != nil {
		return "", xerrors.WithStack(err)
	}

	metaMsg := &api.RequestUploadArtifact{}
	meta := &api.ArtifactUploadMeta{}
	meta.SetTaskId(int32(c.taskID))
	meta.SetArtifact(buildArtifactRef(int32(c.taskID), e))
	metaMsg.SetMeta(meta)
	if err := stream.Send(metaMsg); err != nil {
		return "", xerrors.WithStack(err)
	}

	var reader io.Reader
	if e.Type == "container_image" {
		pr, pw := io.Pipe()
		go func() {
			pw.CloseWithError(tarDir(e.AbsPath, pw))
		}()
		reader = pr
	} else {
		f, err := os.Open(e.AbsPath)
		if err != nil {
			return "", xerrors.WithStack(err)
		}
		defer f.Close()
		reader = f
	}

	buf := make([]byte, 1<<20) // 1 MiB, below the default gRPC message limit
	for {
		n, rerr := reader.Read(buf)
		if n > 0 {
			chunk := &api.RequestUploadArtifact{}
			chunk.SetChunk(append([]byte(nil), buf[:n]...))
			if err := stream.Send(chunk); err != nil {
				return "", xerrors.WithStack(err)
			}
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			return "", xerrors.WithStack(rerr)
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	return resp.GetReference(), nil
}

// tarDir writes the contents of dir as a tar stream (paths relative to dir).
func tarDir(dir string, w io.Writer) error {
	tw := tar.NewWriter(w)
	defer tw.Close()
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}
