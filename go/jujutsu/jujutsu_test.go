package jujutsu

import (
	"os"
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
