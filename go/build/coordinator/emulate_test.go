package coordinator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestWriteCRILog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.log")
	assertion.MustNoError(t, writeCRILog(path, []byte("first line\nsecond line\n")))

	buf, err := os.ReadFile(path)
	assertion.MustNoError(t, err)

	lines := strings.Split(strings.TrimRight(string(buf), "\n"), "\n")
	assertion.Equal(t, len(lines), 2)

	// kwok requires the CRI container-log format: "<RFC3339Nano> stdout F <msg>".
	want := []string{"first line", "second line"}
	for i, line := range lines {
		fields := strings.SplitN(line, " ", 4)
		assertion.Equal(t, len(fields), 4)
		_, err := time.Parse("2006-01-02T15:04:05.000000000Z", fields[0])
		assertion.MustNoError(t, err)
		assertion.Equal(t, fields[1], "stdout")
		assertion.Equal(t, fields[2], "F")
		assertion.Equal(t, fields[3], want[i])
	}
}
