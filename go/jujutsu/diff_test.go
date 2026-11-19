package jujutsu

import (
	"os"
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestParseGitDiff(t *testing.T) {
	// testdata/diff.txt is the output of jj diff by diffArgs which adds, deletes, modifies and renames the files.
	out, err := os.ReadFile("testdata/diff.txt")
	assertion.MustNoError(t, err)

	files, err := parseGitDiff(string(out))
	assertion.MustNoError(t, err)
	assertion.MustLen(t, files, 5)

	added := files[0]
	assertion.Equal(t, added.Path, "add.txt")
	assertion.Equal(t, added.Status, FileStatusAdded)
	assertion.Equal(t, added.Added, 1)
	assertion.MustLen(t, added.Hunks, 1)
	assertion.Equal(t, *added.Hunks[0].Lines[0], DiffLine{Kind: DiffLineAdded, Text: "added", NewLine: 1})

	binary := files[1]
	assertion.Equal(t, binary.Path, "bin.dat")
	assertion.True(t, binary.Binary)
	assertion.Len(t, binary.Hunks, 0)

	deleted := files[2]
	assertion.Equal(t, deleted.Status, FileStatusDeleted)
	assertion.Equal(t, deleted.Removed, 1)
	assertion.Equal(t, *deleted.Hunks[0].Lines[0], DiffLine{Kind: DiffLineRemoved, Text: "gone", OldLine: 1})

	modified := files[3]
	assertion.Equal(t, modified.Status, FileStatusModified)
	assertion.Equal(t, modified.Added, 2)
	assertion.Equal(t, modified.Removed, 1)
	h := modified.Hunks[0]
	assertion.Equal(t, []int{h.OldStart, h.OldLines, h.NewStart, h.NewLines}, []int{1, 8, 1, 9})
	assertion.Equal(t, h.Section, "a")
	assertion.MustLen(t, h.Lines, 10)
	assertion.Equal(t, *h.Lines[1], DiffLine{Kind: DiffLineRemoved, Text: "b", OldLine: 2})
	assertion.Equal(t, *h.Lines[2], DiffLine{Kind: DiffLineAdded, Text: "B", NewLine: 2})
	assertion.Equal(t, *h.Lines[3], DiffLine{Kind: DiffLineContext, Text: "c", OldLine: 3, NewLine: 3})
	assertion.Equal(t, *h.Lines[9], DiffLine{Kind: DiffLineAdded, Text: "new", NewLine: 9, NoNewline: true})

	renamed := files[4]
	assertion.Equal(t, renamed.Status, FileStatusRenamed)
	assertion.Equal(t, renamed.Path, "new.txt")
	assertion.Equal(t, renamed.OldPath, "old.txt")
}

func TestParseDiffHeader(t *testing.T) {
	cases := []struct {
		Line    string
		OldPath string
		NewPath string
	}{
		{Line: "diff --git a/foo.go b/foo.go", OldPath: "foo.go", NewPath: "foo.go"},
		{Line: "diff --git a/dir with b/space.txt b/dir with b/space.txt", OldPath: "dir with b/space.txt", NewPath: "dir with b/space.txt"},
		{Line: "diff --git a/old.txt b/new.txt", OldPath: "old.txt", NewPath: "new.txt"},
	}
	for _, tc := range cases {
		t.Run(tc.Line, func(t *testing.T) {
			oldPath, newPath, err := parseDiffHeader(tc.Line)
			assertion.MustNoError(t, err)
			assertion.Equal(t, oldPath, tc.OldPath)
			assertion.Equal(t, newPath, tc.NewPath)
		})
	}
}

func TestParseHunkHeader(t *testing.T) {
	h, err := parseHunkHeader("@@ -3 +3,2 @@")
	assertion.MustNoError(t, err)
	assertion.Equal(t, []int{h.OldStart, h.OldLines, h.NewStart, h.NewLines}, []int{3, 1, 3, 2})
	assertion.Equal(t, h.Section, "")
}

func TestDiffRangeArgs(t *testing.T) {
	args := diffRangeArgs("--config=x", "b")
	assertion.Contains(t, args, "--from=--config=x")
	assertion.Contains(t, args, "--to=b")
}

func TestInterdiffArgs(t *testing.T) {
	args := interdiffArgs("--config=x", "b")
	assertion.Equal(t, args[0], "interdiff")
	assertion.Contains(t, args, "--git")
	assertion.Contains(t, args, "--from=--config=x")
	assertion.Contains(t, args, "--to=b")
}
