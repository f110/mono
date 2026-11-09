package sidecar

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestScanMarkersAndHash(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bazel-out", "k8-fastbuild", "bin", "pkg")
	assertion.MustNoError(t, os.MkdirAll(binDir, 0o755))
	binContent := []byte("binary-bytes")
	assertion.MustNoError(t, os.WriteFile(filepath.Join(binDir, "foo"), binContent, 0o755))

	marker := markerFile{
		Target: "//pkg:delivery",
		Artifacts: []markerArtifact{
			{Type: "binary", Path: "bazel-out/k8-fastbuild/bin/pkg/foo", Repository: "example.com/foo", Tag: "latest"},
		},
	}
	mb, err := json.Marshal(marker)
	assertion.MustNoError(t, err)
	assertion.MustNoError(t, os.WriteFile(filepath.Join(binDir, "delivery"+markerSuffix), mb, 0o644))

	entries, err := scanMarkers(root)
	assertion.MustNoError(t, err)
	assertion.Len(t, entries, 1)
	e := entries[0]
	assertion.Equal(t, "binary", e.Type)
	assertion.Equal(t, "foo", e.Name)
	assertion.Equal(t, "example.com/foo", e.Repository)
	assertion.Equal(t, "latest", e.Tag)
	assertion.Equal(t, filepath.Join(binDir, "foo"), e.AbsPath)

	h, err := hashArtifact(e)
	assertion.MustNoError(t, err)
	sum := sha256.Sum256(binContent)
	assertion.Equal(t, hex.EncodeToString(sum[:]), h)
}

func TestExecRootFromMarker(t *testing.T) {
	got, err := execRootFromMarker("/out/execroot/mono/bazel-out/k8-fastbuild/bin/pkg/x.cd_artifact.json")
	assertion.MustNoError(t, err)
	assertion.Equal(t, "/out/execroot/mono", got)

	_, err = execRootFromMarker("/tmp/no-out/x.cd_artifact.json")
	assertion.Error(t, err)
}

func TestTarDirFollowsSymlink(t *testing.T) {
	root := t.TempDir()
	blobContent := []byte("layer-bytes")
	assertion.MustNoError(t, os.WriteFile(filepath.Join(root, "layer.tar"), blobContent, 0o644))
	layoutDir := filepath.Join(root, "image")
	blobDir := filepath.Join(layoutDir, "blobs", "sha256")
	assertion.MustNoError(t, os.MkdirAll(blobDir, 0o755))
	assertion.MustNoError(t, os.Symlink("../../../layer.tar", filepath.Join(blobDir, "abc")))

	var buf bytes.Buffer
	assertion.MustNoError(t, tarDir(layoutDir, &buf))

	tr := tar.NewReader(&buf)
	files := make(map[string][]byte)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		assertion.MustNoError(t, err)
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		b, err := io.ReadAll(tr)
		assertion.MustNoError(t, err)
		files[hdr.Name] = b
	}
	assertion.Equal(t, files["blobs/sha256/abc"], blobContent)
}
