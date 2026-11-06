package etcd

import (
	"bytes"
	"compress/zlib"
	"io"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackup_Compressed(t *testing.T) {
	snapshot := make([]byte, 4*1024*1024)
	r := rand.New(rand.NewPCG(1, 2))
	for i := range snapshot {
		snapshot[i] = byte(r.IntN(16))
	}
	b := &Backup{data: io.NopCloser(bytes.NewReader(snapshot))}

	zr, err := zlib.NewReader(b.Compressed())
	require.NoError(t, err)
	got, err := io.ReadAll(zr)
	require.NoError(t, err)
	assert.Equal(t, snapshot, got)
}
