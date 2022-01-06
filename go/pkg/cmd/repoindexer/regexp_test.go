package repoindexer

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRegexp(t *testing.T) {
	cases := []struct {
		In  string
		Out *replaceRule
	}{
		{
			In:  `s/github\.com/gitlab.com/`,
			Out: &replaceRule{re: regexp.MustCompile(`github\.com`), replace: "gitlab.com"},
		},
		{
			In:  `s/ssh:\/\/github.com/https:\/\/proxy.example.com/`,
			Out: &replaceRule{re: regexp.MustCompile(`ssh:\/\/github.com`), replace: "https://proxy.example.com"},
		},
		{In: `s/github/gitlab//`},
	}

	for _, tc := range cases {
		t.Run(tc.In, func(t *testing.T) {
			actual, err := parseRegexp(tc.In)
			if tc.Out == nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				assert.Equal(t, tc.Out.re.String(), actual.re.String())
				assert.Equal(t, tc.Out.replace, actual.replace)
			}
		})
	}
}
