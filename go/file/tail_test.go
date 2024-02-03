package file

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTailReader(t *testing.T) {
	f, err := os.CreateTemp("", "")
	require.NoError(t, err)

	rf, err := os.Open(f.Name())
	require.NoError(t, err)
	r, err := NewTailReader(rf)
	require.NoError(t, err)

	buf := make([]byte, 128)
	writeString(t, f, "foo")
	n, err := r.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	go func() {
		time.Sleep(10 * time.Millisecond)
		writeString(t, f, "ba")
	}()
	n, err = r.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	ch := make(chan struct{})
	go func() {
		_, err := r.Read(buf)
		assert.ErrorIs(t, err, io.EOF)
		ch <- struct{}{}
	}()
	require.NoError(t, r.Close())

	select {
	case <-ch:
	case <-time.After(100 * time.Millisecond):
		require.Fail(t, "timed out")
	}
}

func writeString(t *testing.T, f *os.File, data string) {
	n, err := f.WriteString(data)
	require.NoError(t, err)
	assert.Equal(t, len(data), n)
	require.NoError(t, f.Sync())
}
