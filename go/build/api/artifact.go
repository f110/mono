package api

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.f110.dev/xerrors"
	"oras.land/oras-go/v2"
	orascontent "oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
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
	// repository. The content becomes the single layer of the manifest with
	// mediaType. When tag is empty the artifact is referenced by its digest.
	// Returns the stored reference (tag or digest).
	PushFile(ctx context.Context, repository, tag, artifactType, mediaType, filename string, content []byte) (string, error)
}

// craneArtifactRegistry is an ArtifactRegistry backed by go-containerregistry for
// images and oras-go for file artifacts. The repository coming from cd_artifact
// is a full repository reference including the registry host
// (e.g. "repo.center.x.f110.dev/issue/server"). Authentication uses the docker config.
//
// File artifacts are stored in the ORAS layout (OCI 1.1 manifest with an empty
// config and artifactType); see docs/working/artifact-collection.md §8.
type craneArtifactRegistry struct {
	insecure bool
}

// NewCraneArtifactRegistry returns an ArtifactRegistry that pushes to the registry
// named in each repository. When insecure is true registries are accessed over plain HTTP.
func NewCraneArtifactRegistry(insecure bool) ArtifactRegistry {
	return &craneArtifactRegistry{insecure: insecure}
}

func (r *craneArtifactRegistry) options(ctx context.Context) []crane.Option {
	opts := []crane.Option{crane.WithContext(ctx)}
	if r.insecure {
		opts = append(opts, crane.Insecure)
	}
	return opts
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
	ref := repository + ":" + tag
	if tag == "" {
		ref = repository + "@" + digest.String()
	}
	if err := crane.Push(img, ref, r.options(ctx)...); err != nil {
		return "", xerrors.WithStack(err)
	}
	return reference, nil
}

func (r *craneArtifactRegistry) PushFile(ctx context.Context, repository, tag, artifactType, mediaType, filename string, content []byte) (string, error) {
	repo, err := remote.NewRepository(repository)
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	repo.PlainHTTP = r.insecure
	repo.Client = &auth.Client{Client: retry.DefaultClient, Cache: auth.NewCache(), Credential: keychainCredential}

	layer := orascontent.NewDescriptorFromBytes(mediaType, content)
	layer.Annotations = map[string]string{ocispec.AnnotationTitle: filename}
	if err := repo.Push(ctx, layer, bytes.NewReader(content)); err != nil {
		return "", xerrors.WithStack(err)
	}
	// Pin the creation time so that the manifest digest only depends on the content.
	manifest, err := oras.PackManifest(ctx, repo, oras.PackManifestVersion1_1, artifactType, oras.PackManifestOptions{
		Layers:              []ocispec.Descriptor{layer},
		ManifestAnnotations: map[string]string{ocispec.AnnotationCreated: "1970-01-01T00:00:00Z"},
	})
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	if tag == "" {
		return manifest.Digest.String(), nil
	}
	if err := repo.Tag(ctx, manifest, tag); err != nil {
		return "", xerrors.WithStack(err)
	}
	return tag, nil
}

// keychainCredential resolves the credential for hostport from the default keychain
// so that file artifacts are pushed with the same credential as images.
func keychainCredential(_ context.Context, hostport string) (auth.Credential, error) {
	reg, err := name.NewRegistry(hostport)
	if err != nil {
		return auth.EmptyCredential, xerrors.WithStack(err)
	}
	a, err := authn.DefaultKeychain.Resolve(reg)
	if err != nil {
		return auth.EmptyCredential, xerrors.WithStack(err)
	}
	cfg, err := a.Authorization()
	if err != nil {
		return auth.EmptyCredential, xerrors.WithStack(err)
	}
	return auth.Credential{Username: cfg.Username, Password: cfg.Password, RefreshToken: cfg.IdentityToken, AccessToken: cfg.RegistryToken}, nil
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
