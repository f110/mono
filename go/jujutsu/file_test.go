package jujutsu

import (
	"os"
	"path/filepath"
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestFileArgs(t *testing.T) {
	// The path is quoted not to be interpreted as the pattern of the fileset.
	assertion.Equal(t, fileShowArgs("abc", `d i/a"b\c.txt`), []string{"file", "show", "--ignore-working-copy", "--revision=abc", "--", `root-file:"d i/a\"b\\c.txt"`})
	assertion.Equal(t, fileListArgs("--config=x")[3], "--revision=--config=x")
}

func TestWorkingCopyPath(t *testing.T) {
	r := NewRepository("/repo")
	cases := []struct {
		Path string
		Want string
	}{
		{Path: "a/b.txt", Want: "/repo/a/b.txt"},
		{Path: "a/../b.txt", Want: "/repo/b.txt"},
		{Path: "../etc/passwd"},
		{Path: "a/../../etc/passwd"},
		{Path: "/etc/passwd"},
		{Path: ".jj/repo/store"},
		{Path: ".git/config"},
		{Path: ""},
	}
	for _, tc := range cases {
		t.Run(tc.Path, func(t *testing.T) {
			got, err := r.workingCopyPath(tc.Path)
			if tc.Want == "" {
				assertion.MustError(t, err)
				return
			}
			assertion.MustNoError(t, err)
			assertion.Equal(t, got, tc.Want)
		})
	}
}

func TestWriteFileRejectsSymlinkOutside(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	assertion.MustNoError(t, os.WriteFile(filepath.Join(outside, "secret"), []byte("x"), 0644))
	assertion.MustNoError(t, os.Symlink(outside, filepath.Join(root, "link")))

	err := NewRepository(root).WriteFile(t.Context(), "link/secret", []byte("overwritten"))
	assertion.MustError(t, err)
	buf, err := os.ReadFile(filepath.Join(outside, "secret"))
	assertion.MustNoError(t, err)
	assertion.Equal(t, string(buf), "x")
}
