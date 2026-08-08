package api

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"go.f110.dev/xerrors"
)

// ArtifactRegistry stores build artifacts in an OCI registry (zot). The builder
// is the only component that holds the registry credentials; the collect sidecar
// never talks to the registry directly (it uploads to the builder instead).
type ArtifactRegistry interface {
	// PushImage stores an OCI image, provided as a tarball of its OCI layout, at
	// repository. It fails if the image's manifest digest does not equal
	// expectedDigest (the hash the sidecar computed). When tag is empty the image
	// is referenced by its digest. Returns the stored reference (tag or digest).
	PushImage(ctx context.Context, repository, tag, expectedDigest string, ociLayoutTar io.Reader) (string, error)
	// PushFile stores content as an OCI artifact of the given artifactType under
	// repository. When tag is empty the artifact is referenced by its digest.
	// Returns the stored reference (tag or digest).
	PushFile(ctx context.Context, repository, tag, artifactType, filename string, content []byte) (string, error)
}

// craneArtifactRegistry is a go-containerregistry-backed ArtifactRegistry. host is
// the registry hostname (e.g. "zot.example.com"); repository paths coming from
// cd_artifact are joined under it. Authentication uses the default keychain
// (docker config / ambient credentials).
//
// NOTE: file artifacts are stored as a single-layer OCI image with the
// artifactType carried on the config media type. Switching to the ORAS/OCI-1.1
// empty-config layout (oras-go) is a follow-up; see docs/working/artifact-collection.md §8.
type craneArtifactRegistry struct {
	host string
}

// NewCraneArtifactRegistry returns an ArtifactRegistry that pushes to the OCI
// registry reachable at host.
func NewCraneArtifactRegistry(host string) ArtifactRegistry {
	return &craneArtifactRegistry{host: strings.TrimRight(host, "/")}
}

func (r *craneArtifactRegistry) repoPath(repository string) string {
	if r.host == "" {
		return repository
	}
	return r.host + "/" + repository
}

func (r *craneArtifactRegistry) PushImage(ctx context.Context, repository, tag, expectedDigest string, ociLayoutTar io.Reader) (string, error) {
	dir, err := os.MkdirTemp("", "artifact-image-")
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	defer os.RemoveAll(dir)
	if err := extractTar(ociLayoutTar, dir); err != nil {
		return "", err
	}

	img, err := imageFromOCILayout(dir)
	if err != nil {
		return "", err
	}
	digest, err := img.Digest()
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	if expectedDigest != "" && digest.String() != expectedDigest {
		return "", xerrors.Definef("image digest mismatch: got %s, declared %s", digest.String(), expectedDigest).WithStack()
	}

	reference := tag
	if reference == "" {
		reference = digest.String()
	}
	ref := r.repoPath(repository) + ":" + tag
	if tag == "" {
		ref = r.repoPath(repository) + "@" + digest.String()
	}
	if err := crane.Push(img, ref, crane.WithContext(ctx)); err != nil {
		return "", xerrors.WithStack(err)
	}
	return reference, nil
}

func (r *craneArtifactRegistry) PushFile(ctx context.Context, repository, tag, artifactType, filename string, content []byte) (string, error) {
	img, err := crane.Image(map[string][]byte{filename: content})
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	// Carry the artifact type on the config media type and record the file name.
	img = mutate.ConfigMediaType(img, types.MediaType(artifactType))
	if withAnnotations, ok := mutate.Annotations(img, map[string]string{
		"org.opencontainers.image.title": filename,
	}).(v1.Image); ok {
		img = withAnnotations
	}
	digest, err := img.Digest()
	if err != nil {
		return "", xerrors.WithStack(err)
	}

	reference := tag
	if reference == "" {
		reference = digest.String()
	}
	ref := r.repoPath(repository) + ":" + tag
	if tag == "" {
		ref = r.repoPath(repository) + "@" + digest.String()
	}
	if err := crane.Push(img, ref, crane.WithContext(ctx)); err != nil {
		return "", xerrors.WithStack(err)
	}
	return reference, nil
}

// imageFromOCILayout loads the single image from an OCI layout directory. When
// the layout is an index it returns the first image manifest.
func imageFromOCILayout(dir string) (v1.Image, error) {
	idx, err := layout.ImageIndexFromPath(dir)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	m, err := idx.IndexManifest()
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	for _, desc := range m.Manifests {
		if desc.MediaType.IsImage() {
			img, err := idx.Image(desc.Digest)
			if err != nil {
				return nil, xerrors.WithStack(err)
			}
			return img, nil
		}
	}
	return nil, xerrors.Define("no image manifest found in the OCI layout").WithStack()
}

// extractTar unpacks a tar stream into dir, guarding against path traversal.
func extractTar(r io.Reader, dir string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return xerrors.WithStack(err)
		}
		target := filepath.Join(dir, hdr.Name)
		if !strings.HasPrefix(target, filepath.Clean(dir)+string(os.PathSeparator)) && target != filepath.Clean(dir) {
			return xerrors.Definef("tar entry escapes destination: %s", hdr.Name).WithStack()
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return xerrors.WithStack(err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return xerrors.WithStack(err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return xerrors.WithStack(err)
			}
			if _, err := io.Copy(f, tr); err != nil { //nolint:gosec // sizes are bounded by the upload limit
				f.Close()
				return xerrors.WithStack(err)
			}
			if err := f.Close(); err != nil {
				return xerrors.WithStack(err)
			}
		}
	}
}
