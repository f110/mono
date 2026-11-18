package jujutsu

import (
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

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
