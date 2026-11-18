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
