package converter

import (
	"bytes"

	"github.com/yuin/goldmark"
	"go.f110.dev/xerrors"
)

type MarkdownConverter struct{}

func (m *MarkdownConverter) Convert(in string, outFormat Format) (string, error) {
	switch outFormat {
	case Format_HTML:
		buf := new(bytes.Buffer)
		if err := goldmark.Convert([]byte(in), buf); err != nil {
			return "", xerrors.WithStack(err)
		}
		return buf.String(), nil
	default:
		return "", xerrors.Definef("%s is not supported", outFormat).WithStack()
	}
}
