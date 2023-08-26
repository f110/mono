package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseChecksumFile(t *testing.T) {
	buf := `c3e8a47b9926adc305cacf64e6d17964dfa08c570c139a734e00c381bf38ba49  bazel-6.3.2-darwin-arm64
`
	checksums, err := parseChecksumFile(strings.NewReader(buf))
	require.NoError(t, err)
	if assert.Len(t, checksums, 1) {
		assert.Equal(t, "c3e8a47b9926adc305cacf64e6d17964dfa08c570c139a734e00c381bf38ba49", checksums[0].Hash)
		assert.Equal(t, "bazel-6.3.2-darwin-arm64", checksums[0].Filename)
	}

	buf = `c3e8a47b9926adc305cacf64e6d17964dfa08c570c139a734e00c381bf38ba49 *bazel-6.3.2-darwin-arm64
`
	checksums, err = parseChecksumFile(strings.NewReader(buf))
	require.NoError(t, err)
	if assert.Len(t, checksums, 1) {
		assert.True(t, checksums[0].Binary)
	}
}
