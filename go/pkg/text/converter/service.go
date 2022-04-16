package converter

import (
	"context"
)

type MarkdownConverterService struct{}

func (m *MarkdownConverterService) Convert(ctx context.Context, in *RequestConvert) (*ResponseConvert, error) {
	c := &MarkdownConverter{}
	out, err := c.Convert(in.In, in.Format)
	if err != nil {
		return nil, err
	}
	return &ResponseConvert{Out: out}, nil
}
