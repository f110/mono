package docutil

import (
	"testing"

	"go.f110.dev/mono/go/pkg/logger"
)

func TestMain(m *testing.M) {
	logger.Init()
	m.Run()
}
