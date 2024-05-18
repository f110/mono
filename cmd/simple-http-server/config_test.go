package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/ucl"
)

func TestConfig(t *testing.T) {
	t.Run("MultiServer", func(t *testing.T) {
		multiServer := `
server {
  listen: ":8081"
  access_log = "/dev/stdout"

  path "/*" {
    root = "."
  }
}

server {
  listen = ":8082"

  path "/" {
    root = "."
  }
}
`

		d := ucl.NewDecoder(strings.NewReader(multiServer))
		conf, err := readConfig(d)
		require.NoError(t, err)
		assert.Len(t, conf.Servers(), 2)
	})

	t.Run("SingleServer", func(t *testing.T) {
		singleServer := `
server {
  listen: ":8080"

  path "/*" {
    proxy: "incluster-hl-svc.storage.svc.cluster.local:9000/mirror/"
  }
}`

		d := ucl.NewDecoder(strings.NewReader(singleServer))
		conf, err := readConfig(d)
		require.NoError(t, err)
		assert.Len(t, conf.Servers(), 1)
	})
}
