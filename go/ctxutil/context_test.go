package ctxutil

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithTimeout(t *testing.T) {
	ctx, cancel := WithTimeout(context.Background(), time.Second)
	file, line := Source(ctx)
	assert.NotEmpty(t, file)
	assert.Greater(t, line, 1)
	cancel()
	assert.Contains(t, ctx.Err().Error(), file)
	assert.Contains(t, ctx.Err().Error(), strconv.Itoa(line))

	file, line = Source(context.Background())
	assert.Empty(t, file)
	assert.Equal(t, -1, line)

	ctx1, _ := WithTimeout(context.Background(), time.Second)
	ctx2, _ := WithTimeout(ctx1, time.Second)
	file1, line1 := Source(ctx1)
	file2, line2 := Source(ctx2)
	assert.Equal(t, file1, file2)
	assert.NotEqual(t, line1, line2)
}
