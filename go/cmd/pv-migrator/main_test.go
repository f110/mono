package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func checkFile(t *testing.T, from, to string) {
	fs, err := os.Stat(from)
	if os.IsNotExist(err) {
		t.Fatalf("Probably test BUG. from path is not exist: %v", from)
	}
	ts, err := os.Stat(to)
	if os.IsNotExist(err) {
		t.Fatalf("%s (destination path) is not exist", to)
	}

	if fs.Size() != ts.Size() {
		t.Errorf("each file size is not equal.")
	}
	if fs.Mode() != ts.Mode() {
		t.Errorf("FileMode is mismatch")
	}

	ft, ok := fs.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("could not covert to syscall.Stat_t")
	}
	tt, ok := fs.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("could not convert to syscall.Stat_t")
	}
	if ft.Uid != tt.Uid {
		t.Errorf("UID is mismatch")
	}
	if ft.Gid != tt.Gid {
		t.Errorf("GID is mismatch")
	}
}

func checkDir(t *testing.T, src, dst string) {
	fs, err := os.Stat(src)
	if os.IsNotExist(err) {
		t.Fatalf("Probably test BUG. from path is not exist: %v", src)
	}
	ts, err := os.Stat(dst)
	if os.IsNotExist(err) {
		t.Fatalf("%s (destination path) is not exist", dst)
	}

	if fs.Mode() != ts.Mode() {
		t.Errorf("FileMode is mismatch")
	}

	ft, ok := fs.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("could not covert to syscall.Stat_t")
	}
	tt, ok := fs.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("could not convert to syscall.Stat_t")
	}
	if ft.Uid != tt.Uid {
		t.Errorf("UID is mismatch")
	}
	if ft.Gid != tt.Gid {
		t.Errorf("GID is mismatch")
	}
}

func TestMigrateDirectory(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	err = ioutil.WriteFile(filepath.Join(tmpDir, "regular"), []byte("regular"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(filepath.Join(tmpDir, "dir1"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(filepath.Join(tmpDir, "dir1", "dir2"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(tmpDir, "dir1", "dir2", "regular"), []byte("regular"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	dest, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal()
	}
	defer os.RemoveAll(dest)

	if err := migrateDirectory(tmpDir, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, lockFilename)); os.IsNotExist(err) {
		t.Fatal("Expect create lock file")
	}

	checkFile(t, filepath.Join(tmpDir, "regular"), filepath.Join(dest, "regular"))
	checkDir(t, filepath.Join(tmpDir, "dir1"), filepath.Join(dest, "dir1"))
	checkDir(t, filepath.Join(tmpDir, "dir1", "dir2"), filepath.Join(dest, "dir1", "dir2"))
	checkFile(t, filepath.Join(tmpDir, "dir1", "dir2", "regular"), filepath.Join(dest, "dir1", "dir2", "regular"))
}
