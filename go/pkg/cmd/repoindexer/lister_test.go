package repoindexer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceRule(t *testing.T) {
	cases := []struct {
		Regexp   string
		In       string
		Expected string
	}{
		{
			Regexp:   `s/github\.com/gitlab.com/`,
			In:       "ssh://github.com/octocat/test.git",
			Expected: "ssh://gitlab.com/octocat/test.git",
		},
		{
			Regexp:   `s/ssh:\/\/github.com/https:\/\/gitlab.com/`,
			In:       "ssh://github.com/octocat/test.git",
			Expected: "https://gitlab.com/octocat/test.git",
		},
		{
			Regexp:   `s/ssh:\/\/github.com/https:\/\/example.com\/proxy/`,
			In:       "ssh://github.com/octocat/test.git",
			Expected: "https://example.com/proxy/octocat/test.git",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Regexp, func(t *testing.T) {
			r, err := parseRegexp(tc.Regexp)
			require.NoError(t, err)

			actual := r.re.ReplaceAllString(tc.In, r.replace)
			assert.Equal(t, tc.Expected, actual)
		})
	}
}
