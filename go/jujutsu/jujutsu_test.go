package jujutsu

import (
	"os"
	"path/filepath"
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestParseLog(t *testing.T) {
	// testdata/log.txt is the output of jj log by logArgs for the repository which has the merge commit.
	out, err := os.ReadFile("testdata/log.txt")
	assertion.MustNoError(t, err)

	rows, err := parseLog(out)
	assertion.MustNoError(t, err)
	assertion.MustLen(t, rows, 5)

	merge := rows[0]
	assertion.Equal(t, merge.Commit.Description, "merge")
	assertion.True(t, merge.Commit.CurrentWorkingCopy)
	assertion.Len(t, merge.Commit.Parents, 2)
	assertion.Equal(t, merge.Lines, []GraphLine{{Gutter: "@    ", Node: true}, {Gutter: "├─╮"}})

	assertion.Equal(t, rows[1].Lines, []GraphLine{{Gutter: "│ ○  ", Node: true}})

	second := rows[2]
	assertion.Equal(t, second.Commit.Description, "second")
	assertion.Equal(t, second.Commit.LocalBookmarks, []string{"main"})
	assertion.Len(t, second.Commit.RemoteBookmarks, 0)
	assertion.Equal(t, second.Commit.Trailers, []Trailer{{Key: "Agent", Value: "claude"}, {Key: "Agent-Session", Value: "sess-1"}})
	assertion.Equal(t, second.Commit.Timestamp, int64(1700000000))
	assertion.Equal(t, second.Lines, []GraphLine{{Gutter: "○ │  ", Node: true}, {Gutter: "├─╯"}})

	root := rows[4]
	assertion.True(t, root.Commit.Immutable)
	assertion.Equal(t, root.Commit.ChangeID, "zzzzzzzzzzzz")
}

func TestParseLogElided(t *testing.T) {
	out := "○  \x1F{\"change_id\":\"a\",\"timestamp\":\"0\"}\n~  (elided revisions)\n◆  \x1F{\"change_id\":\"b\",\"timestamp\":\"0\"}\n│\n~\n"

	rows, err := parseLog([]byte(out))
	assertion.MustNoError(t, err)
	assertion.MustLen(t, rows, 2)
	assertion.Equal(t, rows[0].Lines, []GraphLine{{Gutter: "○  ", Node: true}, {Gutter: "~  ", Content: "(elided revisions)"}})
	assertion.Equal(t, rows[1].Lines, []GraphLine{{Gutter: "◆  ", Node: true}, {Gutter: "│"}, {Gutter: "~"}})
}

func TestOperationHeadsDir(t *testing.T) {
	dir := t.TempDir()
	heads := filepath.Join(dir, "main", ".jj", "repo", "op_heads", "heads")
	assertion.MustNoError(t, os.MkdirAll(heads, 0755))
	assertion.MustNoError(t, os.WriteFile(filepath.Join(heads, "abc"), nil, 0644))
	// .jj/repo of the secondary workspace has the path relative to .jj.
	assertion.MustNoError(t, os.MkdirAll(filepath.Join(dir, "second", ".jj"), 0755))
	assertion.MustNoError(t, os.WriteFile(filepath.Join(dir, "second", ".jj", "repo"), []byte("../../main/.jj/repo"), 0644))

	for _, ws := range []string{"main", "second"} {
		t.Run(ws, func(t *testing.T) {
			r := NewRepository(filepath.Join(dir, ws))
			got, err := r.OperationHeadsDir()
			assertion.MustNoError(t, err)
			assertion.Equal(t, filepath.Clean(got), heads)
			id, err := r.OperationID(t.Context())
			assertion.MustNoError(t, err)
			assertion.Equal(t, id, "abc")
		})
	}
}

func TestReadArgs(t *testing.T) {
	// The revset is joined with "=" not to be interpreted as the option.
	assertion.Contains(t, logArgs("--config=x", 0), "--revisions=--config=x")
	assertion.NotContains(t, logArgs("--config=x", 0), "-r")
	assertion.Contains(t, diffArgs("--config=x"), "--revisions=--config=x")
	assertion.Contains(t, showArgs("--config=x"), "--revisions=--config=x")
}

func TestAgentRevset(t *testing.T) {
	assertion.Equal(
		t,
		agentRevset("claude", `s.1"\`),
		`description(regex:"(?m)^Agent: claude$") & description(regex:"(?m)^Agent-Session: s\\.1\"\\\\$")`,
	)
}

func TestCommitCreatedBy(t *testing.T) {
	c := &Commit{Trailers: []Trailer{{Key: "Agent", Value: "claude"}, {Key: "Agent-Session", Value: "sess-1"}}}
	assertion.True(t, c.CreatedBy("claude", "sess-1"))
	assertion.False(t, c.CreatedBy("opencode", "sess-1"))
	assertion.False(t, c.CreatedBy("claude", "sess-2"))
	assertion.False(t, (&Commit{Trailers: []Trailer{{Key: "Agent-Session", Value: "sess-1"}}}).CreatedBy("claude", "sess-1"))
}

func TestOperationArgs(t *testing.T) {
	// The revisions and the messages which look like the options must not be the options.
	assertion.Equal(t, newArgs([]string{"--config=x", "abc"}), []string{"new", "--", "--config=x", "abc"})
	assertion.Equal(t, abandonArgs([]string{"abc"}), []string{"abandon", "--", "abc"})
	assertion.Equal(t, editArgs("--config=x"), []string{"edit", "--", "--config=x"})
	assertion.Equal(t, describeArgs("abc", "-m"), []string{"describe", "--message=-m", "--", "abc"})
	assertion.Equal(t, commitArgs("--foo"), []string{"commit", "--message=--foo"})
}

func TestRebaseArgs(t *testing.T) {
	assertion.Equal(t, rebaseArgs([]string{"a", "b"}, RebaseRevisions, []string{"c"}, RebaseOnto), []string{"rebase", "--revisions=a", "--revisions=b", "--onto=c"})
	assertion.Equal(t, rebaseArgs([]string{"a"}, RebaseSourceDescendants, []string{"c", "d"}, RebaseInsertAfter), []string{"rebase", "--source=a", "--insert-after=c", "--insert-after=d"})
	assertion.Equal(t, rebaseArgs([]string{"a"}, RebaseBranch, []string{"c"}, RebaseInsertBefore), []string{"rebase", "--branch=a", "--insert-before=c"})
}

func TestSquashArgs(t *testing.T) {
	assertion.Equal(t, squashArgs([]string{"a"}, "b", ""), []string{"squash", "--from=a", "--into=b", "--use-destination-message"})
	assertion.Equal(t, squashArgs([]string{"a", "c"}, "b", "msg"), []string{"squash", "--from=a", "--from=c", "--into=b", "--message=msg"})
}

func TestSplitAndRestoreArgs(t *testing.T) {
	assertion.Equal(t, splitArgs("abc", "-m", []string{"a b", "c"}), []string{"split", "--revision=abc", "--message=-m", "--", `root-file:"a b"`, `root-file:"c"`})
	assertion.Equal(t, restoreArgs("abc", []string{"f"}), []string{"restore", "--changes-in=abc", "--", `root-file:"f"`})
}

func TestParseBookmarks(t *testing.T) {
	// The output of jj bookmark list by bookmarksArgs.
	out := `{"name":"main","remote":"","tracked":false,"conflict":false,"synced":false,"target":"abc","ahead":0,"behind":0,"description":"local"}
{"name":"main","remote":"git","tracked":true,"conflict":false,"synced":true,"target":"abc","ahead":0,"behind":0,"description":"local"}
{"name":"main","remote":"origin","tracked":true,"conflict":false,"synced":false,"target":"def","ahead":2,"behind":1,"description":"remote"}
`
	bookmarks, err := parseBookmarks([]byte(out))
	assertion.MustNoError(t, err)
	assertion.MustLen(t, bookmarks, 2)
	assertion.Equal(t, *bookmarks[0], Bookmark{Name: "main", Target: "abc", Description: "local"})
	assertion.Equal(t, *bookmarks[1], Bookmark{Name: "main", Remote: "origin", Tracked: true, Target: "def", Ahead: 2, Behind: 1, Description: "remote"})
}

func TestBookmarkArgs(t *testing.T) {
	// The names are exact and the values are not the options.
	assertion.Equal(t, bookmarkSetArgs("--foo", "--config=x"), []string{"bookmark", "set", "--revision=--config=x", "--", "--foo"})
	assertion.Equal(t, bookmarkMoveArgs("feat-*", "abc"), []string{"bookmark", "move", "--to=abc", "--allow-backwards", "--", "exact:feat-*"})
	assertion.Equal(t, bookmarkDeleteArgs("feat-*"), []string{"bookmark", "delete", "--", "exact:feat-*"})
	assertion.Equal(t, bookmarkTrackArgs("main", "origin"), []string{"bookmark", "track", "--remote=exact:origin", "--", "exact:main"})
	assertion.Equal(t, gitPushArgs(""), []string{"git", "push"})
	assertion.Equal(t, gitPushArgs("feat-*"), []string{"git", "push", "--bookmark=exact:feat-*"})
}

func TestWorkspaceAddArgs(t *testing.T) {
	assertion.Equal(t, workspaceAddArgs("--x", "abc", "/src/repo-x"), []string{"workspace", "add", "--name=--x", "--revision=abc", "--", "/src/repo-x"})
}

func TestFileArgs(t *testing.T) {
	// The path is quoted not to be interpreted as the pattern of the fileset.
	assertion.Equal(t, fileShowArgs("abc", `d i/a"b\c.txt`), []string{"file", "show", "--ignore-working-copy", "--revision=abc", "--", `root-file:"d i/a\"b\\c.txt"`})
	assertion.Equal(t, fileListArgs("--config=x")[3], "--revision=--config=x")
}

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

func TestConflictArgs(t *testing.T) {
	assertion.Equal(t, conflictsArgs("abc"), []string{"log", "--no-graph", "--ignore-working-copy", "--revisions=abc", "-T", conflictTemplate})
}

func TestParseEvolog(t *testing.T) {
	// The output of jj evolog by evologArgs.
	out := `{"commit_id":"bbb","time":"1700000100","operation":"describe commit aaa","predecessors":["aaa"]}` + "\x1E" +
		"diff --git a/a.txt b/a.txt\nindex 1..2 100644\n--- a/a.txt\n+++ b/a.txt\n@@ -1,1 +1,1 @@\n-a\n+b\n" + "\x1D" +
		`{"commit_id":"aaa","time":"1700000000","operation":"new empty commit","predecessors":[]}` + "\x1E" + "\x1D"

	entries, err := parseEvolog(out)
	assertion.MustNoError(t, err)
	assertion.MustLen(t, entries, 2)
	assertion.Equal(t, entries[0].CommitID, "bbb")
	assertion.Equal(t, entries[0].Predecessors, []string{"aaa"})
	assertion.Equal(t, entries[0].Timestamp, int64(1700000100))
	assertion.MustLen(t, entries[0].Files, 1)
	assertion.Equal(t, entries[0].Files[0].Path, "a.txt")
	assertion.Equal(t, entries[1].Operation, "new empty commit")
	assertion.Len(t, entries[1].Files, 0)
}

func TestParseAliases(t *testing.T) {
	out := `{"name":"aliases.f","value":["git","fetch"]}
{"name":"aliases.weird","value":{"x":1}}
`
	aliases, err := parseAliases([]byte(out))
	assertion.MustNoError(t, err)
	assertion.MustLen(t, aliases, 1)
	assertion.Equal(t, *aliases[0], Alias{Name: "f", Args: []string{"git", "fetch"}})
}
