package querylog

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/database/dbtestutil"
	"go.f110.dev/mono/go/logger"
)

func TestQueryLog(t *testing.T) {
	if !dbtestutil.CanUseTemporaryMySQL() {
		t.Skipped()
		return
	}

	logger.Init()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	mysql, err := dbtestutil.NewTemporaryMySQL(ctx)
	require.NoError(t, err)
	if testing.Verbose() {
		mysql.Verbose()
	}
	require.NoError(t, mysql.Start())
	t.Cleanup(func() {
		mysql.Close()
	})
	execQuery := func(t *testing.T) {
		conn, err := sql.Open("querylog", fmt.Sprintf("root@tcp(localhost:%d)/mysql", mysql.Port))
		require.NoError(t, err)

		rows, err := conn.QueryContext(context.Background(), "SELECT * FROM user")
		require.NoError(t, err)
		require.NoError(t, rows.Close())

		rows, err = conn.Query("SELECT * FROM user")
		require.NoError(t, err)
		require.NoError(t, rows.Close())

		_, err = conn.Exec("SELECT * FROM user")
		require.NoError(t, err)

		_, err = conn.ExecContext(context.Background(), "SELECT * FROM user")
		require.NoError(t, err)
	}

	t.Run("zap", func(t *testing.T) {
		var buf bytes.Buffer
		log := logger.NewBufferLogger(&buf)
		Init(log)

		execQuery(t)
		assert.Contains(t, buf.String(), "SELECT * FROM user")
	})

	t.Run("SetMinimumDuration", func(t *testing.T) {
		var buf bytes.Buffer
		log := logger.NewBufferLogger(&buf)
		Init(log)
		SetMinimumDuration(1 * time.Millisecond)

		execQuery(t)
		assert.NotContains(t, buf.String(), "SELECT * FROM user")
	})
}
