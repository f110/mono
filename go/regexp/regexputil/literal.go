package regexputil

import (
	"regexp"
	"strings"

	"go.f110.dev/xerrors"
)

type Operator int

const (
	OperatorMatch Operator = iota
	OperatorReplace
)

type RegexpLiteral struct {
	Operator Operator

	Match   *regexp.Regexp
	Replace string
}

type regexpState int

const (
	regexpStateInit regexpState = iota
	regexpStateMatch
	regexpStateReplace
	regexpStateEnd
)

func ParseRegexpLiteral(v string) (*RegexpLiteral, error) {
	operator := OperatorMatch
	switch v[0] {
	case 's':
		operator = OperatorReplace
	}

	state := regexpStateInit
	matchStart := 0
	matchEnd := 0
	replaceStart := 0
	replaceEnd := 0
	for i := 1; i < len(v); i++ {
		s := v[i]
		switch s {
		case '/':
			switch state {
			case regexpStateInit:
				matchStart = i + 1
				state = regexpStateMatch
			case regexpStateMatch:
				if v[i-1] == '\\' {
					continue
				}
				matchEnd = i
				replaceStart = i + 1
				state = regexpStateReplace
			case regexpStateReplace:
				if v[i-1] == '\\' {
					continue
				}
				replaceEnd = i
				state = regexpStateEnd
			}
		}
	}
	if len(v)-1 != replaceEnd {
		return nil, xerrors.Newf("invalid regexp: %s", v)
	}

	matchRe, err := regexp.Compile(v[matchStart:matchEnd])
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	replace := strings.ReplaceAll(v[replaceStart:replaceEnd], "\\/", "/")

	return &RegexpLiteral{Operator: operator, Match: matchRe, Replace: replace}, nil
}
