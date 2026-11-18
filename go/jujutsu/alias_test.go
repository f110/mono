package jujutsu

import (
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestParseAliases(t *testing.T) {
	out := `{"name":"aliases.f","value":["git","fetch"]}
{"name":"aliases.weird","value":{"x":1}}
`
	aliases, err := parseAliases([]byte(out))
	assertion.MustNoError(t, err)
	assertion.MustLen(t, aliases, 1)
	assertion.Equal(t, *aliases[0], Alias{Name: "f", Args: []string{"git", "fetch"}})
}
