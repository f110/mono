package generator

import (
	"bytes"

	"go.f110.dev/protoc-ddl/internal/schema"
)

type Generator interface {
	Generate(*bytes.Buffer, []*schema.Table)
}
