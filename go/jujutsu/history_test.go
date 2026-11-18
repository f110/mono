package jujutsu

import (
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

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
