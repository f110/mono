package gomodule

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
