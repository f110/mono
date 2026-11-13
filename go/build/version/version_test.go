package version

import (
	"testing"
	"time"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestCommitTimestamp(t *testing.T) {
	cases := []struct {
		Name       string
		CommitTime string
		Want       time.Time
	}{
		{Name: "RFC 3339", CommitTime: "2026-11-13T14:03:20Z", Want: time.Date(2026, 11, 13, 14, 3, 20, 0, time.UTC)},
		{Name: "not set", CommitTime: ""},
		{Name: "invalid format", CommitTime: "1763042600"},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			orig := CommitTime
			t.Cleanup(func() { CommitTime = orig })
			CommitTime = tc.CommitTime

			assertion.True(t, CommitTimestamp().Equal(tc.Want))
		})
	}
}
