package jujutsu

import (
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestConflictArgs(t *testing.T) {
	assertion.Equal(t, conflictFileArgs("abc", "a b.txt"), []string{"file", "show", "--ignore-working-copy", "--config", "ui.conflict-marker-style=snapshot", "--revision=abc", "--", `root-file:"a b.txt"`})
	assertion.Equal(t, resolveArgs("abc", "f", ResolveOurs), []string{"resolve", "--revision=abc", "--tool=:ours", "--", `root-file:"f"`})
}
