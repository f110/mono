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
