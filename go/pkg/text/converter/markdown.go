package converter

import (
	"bytes"

	"github.com/yuin/goldmark"
	"golang.org/x/xerrors"
)

type MarkdownConverter struct{}

func (m *MarkdownConverter) Convert(in string, outFormat Format) (string, error) {
	switch outFormat {
	case Format_HTML:
		buf := new(bytes.Buffer)
		if err := goldmark.Convert([]byte(in), buf); err != nil {
			return "", xerrors.Errorf(": %w", err)
		}
		return buf.String(), nil
	default:
		return "", xerrors.Errorf("%s is not supported", outFormat)
	}
}
