package macports

import (
	"strings"
)

func Output(tokens []*PortfileToken) (string, error) {
	out := &strings.Builder{}

	var cursor int
	for _, v := range tokens {
		switch v.Type {
		case PortfileTokenIdent:
			if v.StartPos > 0 {
				l := v.StartPos - cursor
				if l < 0 {
					l = 1
				}
				_, err := out.WriteString(strings.Repeat(" ", l))
				if err != nil {
					return "", err
				}
			}
			_, err := out.WriteString(v.Value)
			if err != nil {
				return "", err
			}
			cursor += len(v.Value)
		case PortfileTokenLineBreak:
			_, err := out.WriteString("\n")
			if err != nil {
				return "", err
			}
			cursor = 0
		case PortfileTokenComment:
			_, err := out.WriteString(v.Value)
			if err != nil {
				return "", err
			}
			cursor += len(v.Value)
		case PortfileTokenLBracket:
			if v.StartPos > 0 {
				l := v.StartPos - cursor
				if l < 0 {
					l = 1
				}
				_, err := out.WriteString(strings.Repeat(" ", l))
				if err != nil {
					return "", err
				}
				cursor = v.StartPos
			}
			_, err := out.WriteString("{")
			if err != nil {
				return "", err
			}
			cursor++
		case PortfileTokenRBracket:
			_, err := out.WriteString("}")
			if err != nil {
				return "", err
			}
			cursor++
		}
	}
	return out.String(), nil
}
