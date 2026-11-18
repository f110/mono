package static

import (
	"io/fs"
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestFS(t *testing.T) {
	buf, err := fs.ReadFile(FS(), "index.html")
	assertion.MustNoError(t, err)
	assertion.Contains(t, string(buf), "<title>Codexel</title>")
}
