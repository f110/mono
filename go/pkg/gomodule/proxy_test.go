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
		{In: "v1.0.0-beta.0.0.2006010101010101-abcdedf12345", IsSemanticVersion: false},
	}

	for _, tc := range cases {
		t.Run(tc.In, func(t *testing.T) {
			assert.Equal(t, tc.IsSemanticVersion, IsSemanticVersion(tc.In))
		})
	}
}

func TestParsePseudoVersion(t *testing.T) {
	cases := []struct {
		In     string
		Parsed *PseudoVersion
	}{
		{
			In:     "v1.0.0-pre.0.20101231010101-abcdef123456",
			Parsed: &PseudoVersion{BaseVersion: "v1.0.0-pre", Timestamp: "20101231010101", Revision: "abcdef123456"},
		},
		{
			In:     "v1.0.0-beta.0.0.20060101010101-abcdef123456",
			Parsed: &PseudoVersion{BaseVersion: "v1.0.0-beta.0", Timestamp: "20060101010101", Revision: "abcdef123456"},
		},
		{
			In:     "v1.0.1-0.20101230010101-abcdef123456",
			Parsed: &PseudoVersion{BaseVersion: "v1.0.1", Timestamp: "20101230010101", Revision: "abcdef123456"},
		},
		{In: "v1.0.0-20060101010101-abc"},
	}

	for _, tc := range cases {
		t.Run(tc.In, func(t *testing.T) {
			v, err := ParsePseudoVersion(tc.In)
			if tc.Parsed != nil {
				assert.NoError(t, err)
				assert.Equal(t, tc.Parsed.String(), v.String())
			} else {
				assert.Error(t, err)
			}
		})
	}
}
