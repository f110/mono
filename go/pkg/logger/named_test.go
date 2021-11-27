package logger

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNamedWriter(t *testing.T) {
	tests := []struct {
		In           string
		In2          string
		ExpectOutput string
	}{
		{In: "foobar", ExpectOutput: "[test] foobar"},
		{In: "foobar\nbaz", ExpectOutput: "[test] foobar\n[test] baz"},
		{In: "foobar\nbaz\n", ExpectOutput: "[test] foobar\n[test] baz\n"},
		{In: "foobar\n", ExpectOutput: "[test] foobar\n"},
		{In: "foobar", In2: "baz", ExpectOutput: "[test] foobarbaz"},
		{In: "foobar\n", In2: "baz", ExpectOutput: "[test] foobar\n[test] baz"},
		{In: "foobar\n", In2: "baz\n", ExpectOutput: "[test] foobar\n[test] baz\n"},
	}

	for _, tt := range tests {
		t.Run(tt.In, func(t *testing.T) {
			buf := new(bytes.Buffer)
			w := NewNamedWriter(buf, "test")
			n, err := io.WriteString(w, tt.In)
			require.NoError(t, err)
			assert.Equal(t, len(tt.In), n)
			if tt.In2 != "" {
				n, err = io.WriteString(w, tt.In2)
				require.NoError(t, err)
				assert.Equal(t, len(tt.In2), n)
			}
			assert.Equal(t, tt.ExpectOutput, buf.String())
		})
	}
}
