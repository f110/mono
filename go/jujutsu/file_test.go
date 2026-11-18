package jujutsu

import (
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestFileArgs(t *testing.T) {
	// The path is quoted not to be interpreted as the pattern of the fileset.
	assertion.Equal(t, fileShowArgs("abc", `d i/a"b\c.txt`), []string{"file", "show", "--ignore-working-copy", "--revision=abc", "--", `root-file:"d i/a\"b\\c.txt"`})
	assertion.Equal(t, fileListArgs("--config=x")[3], "--revision=--config=x")
}
