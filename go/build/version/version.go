// Package version provides the commit which the binary is built from.
// The variables are set by x_defs of go_binary at link time.
package version

import (
	"time"
)

var (
	CommitSha string
	// CommitTime is the committer time of CommitSha in RFC 3339.
	CommitTime string
)

// CommitTimestamp returns CommitTime as time.Time.
// It returns the zero value if CommitTime is not set or invalid.
func CommitTimestamp() time.Time {
	t, err := time.Parse(time.RFC3339, CommitTime)
	if err != nil {
		return time.Time{}
	}
	return t
}
