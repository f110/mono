package main

import (
	"testing"

	"go.f110.dev/mono/go/logger"
)

func TestMain(m *testing.M) {
	logger.Init()

	m.Run()
}
