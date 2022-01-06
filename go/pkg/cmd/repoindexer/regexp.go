package repoindexer

import (
	"regexp"
	"strings"

	"golang.org/x/xerrors"
)

type regexpState int

const (
	regexpStateInit regexpState = iota
	regexpStateMatch
	regexpStateReplace
	regexpStateEnd
)

func parseRegexp(in string) (*replaceRule, error) {
	state := regexpStateInit
	matchStart := 0
	matchEnd := 0
	replaceStart := 0
	replaceEnd := 0
	for i := 0; i < len(in); i++ {
		s := in[i]
		switch s {
		case '/':
			switch state {
			case regexpStateInit:
				matchStart = i + 1
				state = regexpStateMatch
			case regexpStateMatch:
				if in[i-1] == '\\' {
					continue
				}
				matchEnd = i
				replaceStart = i + 1
				state = regexpStateReplace
			case regexpStateReplace:
				if in[i-1] == '\\' {
					continue
				}
				replaceEnd = i
				state = regexpStateEnd
			}
		}
	}
	if len(in)-1 != replaceEnd {
		return nil, xerrors.Errorf("invalid regexp: %s", in)
	}

	matchRe, err := regexp.Compile(in[matchStart:matchEnd])
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	replace := strings.ReplaceAll(in[replaceStart:replaceEnd], "\\/", "/")

	return &replaceRule{re: matchRe, replace: replace}, nil
}
