package jujutsu

import (
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestWorkspaceAddArgs(t *testing.T) {
	assertion.Equal(t, workspaceAddArgs("--x", "abc", "/src/repo-x"), []string{"workspace", "add", "--name=--x", "--revision=abc", "--", "/src/repo-x"})
}
