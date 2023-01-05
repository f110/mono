package vault

import (
	"context"
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var vaultBinaryPath = flag.String("test.vault-bin", "", "")

func TestClient(t *testing.T) {
	if *vaultBinaryPath == "" {
		t.Skip("Skip the test for vault client due to -test.vault-bin is not set.")
	}
	s := NewServerManager(t, *vaultBinaryPath)
	client, err := NewClient(s.Addr(), s.Token())
	require.NoError(t, err)

	err = client.Set(context.Background(), "/secret", "simple", map[string]string{"foo": "bar"})
	require.NoError(t, err)

	cacheCtx := NewCache(context.Background())
	val, err := client.Get(cacheCtx, "/secret", "simple", "foo")
	require.NoError(t, err)
	assert.Equal(t, "bar", val)
}
