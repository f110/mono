package regexputil

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRegexpLiteral(t *testing.T) {
	cases := []struct {
		In  string
		Out *RegexpLiteral
	}{
		{
			In:  `s/github\.com/gitlab.com/`,
			Out: &RegexpLiteral{Match: regexp.MustCompile(`github\.com`), Replace: "gitlab.com"},
		},
		{
			In:  `s/ssh:\/\/github.com/https:\/\/proxy.example.com/`,
			Out: &RegexpLiteral{Match: regexp.MustCompile(`ssh:\/\/github.com`), Replace: "https://proxy.example.com"},
		},
		{In: `s/github/gitlab//`},
	}

	for _, tc := range cases {
		t.Run(tc.In, func(t *testing.T) {
			actual, err := ParseRegexpLiteral(tc.In)
			if tc.Out == nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				assert.Equal(t, tc.Out.Match.String(), actual.Match.String())
				assert.Equal(t, tc.Out.Replace, actual.Replace)
			}
		})
	}
}
