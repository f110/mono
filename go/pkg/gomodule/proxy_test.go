package gomodule

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSemanticVersion(t *testing.T) {
	cases := []struct {
		In                string
		IsSemanticVersion bool
	}{
		{In: "v1.0.0", IsSemanticVersion: true},
		{In: "v0.0.1", IsSemanticVersion: true},
		{In: "v0.1.1+meta", IsSemanticVersion: true},
		{In: "v1.0.0-pre", IsSemanticVersion: true},
		{In: "", IsSemanticVersion: false},                             // Empty string
		{In: "v1.0.0-20060101010101-abcdef", IsSemanticVersion: false}, // Pseudo-version
	}

	for _, tc := range cases {
		t.Run(tc.In, func(t *testing.T) {
			assert.Equal(t, tc.IsSemanticVersion, IsSemanticVersion(tc.In))
		})
	}
}

func TestParsePseudoVersion(t *testing.T) {
	cases := []struct {
		In    string
		Valid bool
	}{
		{In: "v1.0.0-20060101010101-abc", Valid: false},
	}

	for _, tc := range cases {
		t.Run(tc.In, func(t *testing.T) {
			_, err := ParsePseudoVersion(tc.In)
			if tc.Valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
