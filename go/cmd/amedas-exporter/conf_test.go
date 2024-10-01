package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/ucl"
)

func TestConfigurationFile(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		c := `
exporter {
	addr = ":8081"
}

amedas_site {
	site "tokyo" {
		no = 132
	}
}
`

		buf, err := ucl.NewDecoder(strings.NewReader(c)).ToJSON(nil)
		require.NoError(t, err)
		var conf configuration
		err = json.Unmarshal(buf, &conf)
		require.NoError(t, err)

		if assert.NotNil(t, conf.Exporter) {
			assert.Equal(t, ":8081", conf.Exporter.Addr)
		}
		require.NotNil(t, conf.AmedasSite)
		assert.Len(t, conf.AmedasSite.Sites, 1)
		assert.Equal(t, &site{"tokyo", 132}, conf.AmedasSite.Sites[0])
	})

	t.Run("Multiple", func(t *testing.T) {
		c := `
exporter {
	addr = ":8081"
}

amedas_site {
	site "tokyo" {
		no = 22
		no = 23 
	}
	site "chiba" {
		no = 100
	}
}`
		buf, err := ucl.NewDecoder(strings.NewReader(c)).ToJSON(nil)
		require.NoError(t, err)
		var conf configuration
		err = json.Unmarshal(buf, &conf)
		require.NoError(t, err)

		if assert.NotNil(t, conf.Exporter) {
			assert.Equal(t, ":8081", conf.Exporter.Addr)
		}
		require.NotNil(t, conf.AmedasSite)
		assert.Len(t, conf.AmedasSite.Sites, 3)
		assert.Equal(t, &site{"tokyo", 22}, conf.AmedasSite.Sites[0])
		assert.Equal(t, &site{"tokyo", 23}, conf.AmedasSite.Sites[1])
		assert.Equal(t, &site{"chiba", 100}, conf.AmedasSite.Sites[2])
	})
}
