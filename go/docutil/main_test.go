package docutil

import (
	"testing"

	"go.f110.dev/mono/go/logger/slogger"
)

func TestMain(m *testing.M) {
	slogger.Init()
	m.Run()
}
