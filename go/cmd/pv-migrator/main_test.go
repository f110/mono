package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/logger"
)

func checkFile(t *testing.T, from, to string) {
	fs, err := os.Stat(from)
	require.NoError(t, err, "Probably test BUG")
	ts, err := os.Stat(to)
	require.NoErrorf(t, err, "%s (destination path) is not exist", to)

	assert.Equal(t, fs.Size(), ts.Size())
	assert.Equal(t, fs.Mode(), ts.Mode())

	ft, ok := fs.Sys().(*syscall.Stat_t)
	require.True(t, ok, "could not covert to syscall.Stat_t")
	tt, ok := fs.Sys().(*syscall.Stat_t)
	require.True(t, ok, "could not covert to syscall.Stat_t")

	assert.Equal(t, ft.Uid, tt.Uid)
	assert.Equal(t, ft.Gid, tt.Gid)
}

func checkDir(t *testing.T, src, dst string) {
	fs, err := os.Stat(src)
	require.NoError(t, err, "Probably test BUG")
	ts, err := os.Stat(dst)
	require.NoErrorf(t, err, "%s (destination path) is not exist", dst)

	assert.Equal(t, fs.Mode(), ts.Mode())

	ft, ok := fs.Sys().(*syscall.Stat_t)
	require.True(t, ok, "could not covert to syscall.Stat_t")
	tt, ok := fs.Sys().(*syscall.Stat_t)
	require.True(t, ok, "could not covert to syscall.Stat_t")

	assert.Equal(t, ft.Uid, tt.Uid)
	assert.Equal(t, ft.Gid, tt.Gid)
}

func TestMigrateDirectory(t *testing.T) {
	logger.Init()

	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "regular"), []byte("regular"), 0644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(tmpDir, "dir1"), 0755)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(tmpDir, "dir1", "dir2"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "dir1", "dir2", "regular"), []byte("regular"), 0600)
	require.NoError(t, err)

	dest := t.TempDir()
	err = migrateDirectory(tmpDir, dest)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(dest, lockFilename))

	checkFile(t, filepath.Join(tmpDir, "regular"), filepath.Join(dest, "regular"))
	checkDir(t, filepath.Join(tmpDir, "dir1"), filepath.Join(dest, "dir1"))
	checkDir(t, filepath.Join(tmpDir, "dir1", "dir2"), filepath.Join(dest, "dir1", "dir2"))
	checkFile(t, filepath.Join(tmpDir, "dir1", "dir2", "regular"), filepath.Join(dest, "dir1", "dir2", "regular"))
}
