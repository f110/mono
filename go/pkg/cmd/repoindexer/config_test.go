package repoindexer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadConfig(t *testing.T) {
	buf := `
refresh_schedule: foobar
rules:
  - owner: f110
    name: example
  - query: org:f110 topics:test`

	config, err := ReadConfig(strings.NewReader(buf))
	require.NoError(t, err)
	if assert.Len(t, config.Rules, 2) {
		assert.Equal(t, "f110", config.Rules[0].Owner)
		assert.Equal(t, "example", config.Rules[0].Name)

		assert.Equal(t, "org:f110 topics:test", config.Rules[1].Query)
	}
}
