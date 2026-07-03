package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.f110.dev/githubmock"
	"golang.org/x/crypto/ripemd160"

	"go.f110.dev/mono/go/macports"
)

func createFakeTarball(t *testing.T) []byte {
	t.Helper()
	return createFakeTarballWithFiles(t, nil)
}

func createFakeTarballWithFiles(t *testing.T, files map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, body := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: name,
			Size: int64(len(body)),
			Mode: 0644,
		}))
		_, err := tw.Write(body)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

type checksums struct {
	rmd160 string
	sha256 string
	size   int64
}

func computeTestChecksums(data []byte) checksums {
	hSha256 := sha256.Sum256(data)
	hRmd160 := ripemd160.New()
	hRmd160.Write(data)
	return checksums{
		rmd160: hex.EncodeToString(hRmd160.Sum(nil)),
		sha256: hex.EncodeToString(hSha256[:]),
		size:   int64(len(data)),
	}
}

func setupMockTransport(t *testing.T, rawURL string, body []byte) http.RoundTripper {
	t.Helper()

	u, err := url.Parse(rawURL)
	require.NoError(t, err)

	mux := http.NewServeMux()
	githubmock.NewMock().RegisterHandler(mux)
	mux.HandleFunc(fmt.Sprintf("GET %s%s", u.Host, u.Path), func(w http.ResponseWriter, _ *http.Request) {
		w.Write(body)
	})

	return &muxTransport{mux: mux}
}

type muxTransport struct {
	mux *http.ServeMux
}

func (t *muxTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	t.mux.ServeHTTP(rec, req)
	return rec.Result(), nil
}

func TestUpdateGolang(t *testing.T) {
	tarball := createFakeTarball(t)
	cs := computeTestChecksums(tarball)
	tr := setupMockTransport(t, "https://github.com/kubecfg/kubecfg/archive/v0.36.0.tar.gz", tarball)

	f, err := os.Open("testdata/Portfile1.before")
	require.NoError(t, err)
	defer f.Close()
	tokens, err := macports.ParseAsTokens(f)
	require.NoError(t, err)

	tokens, err = updateGolang(t.Context(), &http.Client{Transport: tr}, tokens, "v0.36.0")
	require.NoError(t, err)

	out, err := macports.Output(tokens)
	require.NoError(t, err)

	assert.Contains(t, out, "github.com/kubecfg/kubecfg 0.36.0 v")
	assert.Contains(t, out, fmt.Sprintf("rmd160  %s", cs.rmd160))
	assert.Contains(t, out, fmt.Sprintf("sha256  %s", cs.sha256))
	assert.Contains(t, out, fmt.Sprintf("size    %d", cs.size))

	// Verify non-checksum lines are preserved
	assert.Contains(t, out, "PortGroup  golang 1.0")
	assert.Contains(t, out, "name             kubecfg")
	assert.Contains(t, out, "license          Apache-2.0")

	// Verify old version is replaced
	assert.NotContains(t, out, "0.35.2")

	// Verify output is valid by re-parsing
	_, err = macports.ParseAsTokens(strings.NewReader(out))
	require.NoError(t, err)
}

func TestReformatCargoCrates(t *testing.T) {
	// cargo2port right-aligns the version column, but sizes it from a value that
	// does not cover the longest version. A long middle column ("shorter" below)
	// therefore overflows and pushes the checksum column out of alignment.
	in := "cargo.crates \\\n" +
		"    libc                           0.2.155  1111111111111111111111111111111111111111111111111111111111111111 \\\n" +
		"    shorter                       1.2.3-alpha.1+build.longversion  2222222222222222222222222222222222222222222222222222222222222222 \\\n" +
		"    some-crate-with-a-very-long-name-here     0.1.0  3333333333333333333333333333333333333333333333333333333333333333"

	got := reformatCargoCrates(in)
	lines := strings.Split(got, "\n")
	require.Len(t, lines, 4)

	// The header line is passed through untouched.
	assert.Equal(t, "cargo.crates \\", lines[0])

	// Every crate line must share the same checksum column, and each version must
	// be right-aligned so its right edge is exactly two columns before it.
	checksumCol := -1
	for i, line := range lines[1:] {
		fields := strings.Fields(strings.TrimRight(line, " \\"))
		require.Len(t, fields, 3, "line %d: %q", i, line)
		name, version, checksum := fields[0], fields[1], fields[2]
		ci := strings.Index(line, checksum)
		if checksumCol < 0 {
			checksumCol = ci
		}
		assert.Equal(t, checksumCol, ci, "checksum column differs on crate line %d", i)
		assert.Equal(t, checksumCol-2, strings.Index(line, version)+len(version), "version not right-aligned on crate line %d", i)
		assert.True(t, strings.HasPrefix(line, "    "+name), "name not left-aligned on crate line %d", i)
	}

	// The last crate line keeps no continuation backslash.
	assert.False(t, strings.HasSuffix(lines[3], "\\"))

	// The result is still valid Portfile syntax.
	_, err := macports.ParseAsTokens(strings.NewReader(got + "\n"))
	require.NoError(t, err)
}

func TestUpdateRust(t *testing.T) {
	if _, err := exec.LookPath("cargo2port"); err != nil {
		t.Skip("cargo2port not found")
	}

	cargoLock := []byte(`# This file is automatically @generated by Cargo.
# It is not intended for manual editing.
version = 4

[[package]]
name = "jj-cli"
version = "0.40.0"
dependencies = [
 "libc",
]

[[package]]
name = "libc"
version = "0.2.155"
source = "registry+https://github.com/rust-lang/crates.io-index"
checksum = "97b3888a4aecf77e811145cadf6eef5901f4782c53886191b2f693f24761847c"
`)
	tarball := createFakeTarballWithFiles(t, map[string][]byte{
		"jj-0.40.0/Cargo.lock": cargoLock,
	})
	cs := computeTestChecksums(tarball)
	tr := setupMockTransport(t, "https://github.com/jj-vcs/jj/archive/v0.40.0.tar.gz", tarball)

	f, err := os.Open("testdata/Portfile2.before")
	require.NoError(t, err)
	defer f.Close()
	tokens, err := macports.ParseAsTokens(f)
	require.NoError(t, err)

	tokens, err = updateRust(t.Context(), &http.Client{Transport: tr}, tokens, "v0.40.0")
	require.NoError(t, err)

	out, err := macports.Output(tokens)
	require.NoError(t, err)

	// Verify version updated
	assert.Contains(t, out, "jj-vcs jj 0.40.0 v")
	assert.NotContains(t, out, "0.39.0")

	// Verify checksums
	assert.Contains(t, out, fmt.Sprintf("rmd160  %s", cs.rmd160))
	assert.Contains(t, out, fmt.Sprintf("sha256  %s", cs.sha256))
	assert.Contains(t, out, fmt.Sprintf("size    %d", cs.size))

	// Verify cargo.crates was updated with cargo2port output
	assert.Contains(t, out, "cargo.crates")
	assert.Contains(t, out, "libc")
	assert.Contains(t, out, "97b3888a4aecf77e811145cadf6eef5901f4782c53886191b2f693f24761847c")

	// Verify non-updated lines are preserved
	assert.Contains(t, out, "name                jujutsu")
	assert.Contains(t, out, "license             Apache-2")

	// Verify output is valid by re-parsing
	_, err = macports.ParseAsTokens(strings.NewReader(out))
	require.NoError(t, err)
}
