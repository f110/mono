package macports

import (
	"bufio"
	"io"
	"strings"
)

type Portfile struct {
	Name            string
	Homepage        string
	Description     string
	LongDescription string
	License         string
}

func ParsePortfile(r io.Reader) (*Portfile, error) {
	return nil, nil
}

type PortfileTokenType string

const (
	PortfileTokenComment   PortfileTokenType = "comment"
	PortfileTokenLineBreak PortfileTokenType = "line_break"
	PortfileTokenIdent     PortfileTokenType = "ident"
	PortfileTokenLBracket  PortfileTokenType = "l_bracket"
	PortfileTokenRBracket  PortfileTokenType = "r_bracket"
)

type PortfileToken struct {
	Type     PortfileTokenType
	Value    string
	StartPos int
}

func (t *PortfileToken) String() string {
	s := new(strings.Builder)
	s.Grow(t.StartPos)
	for i := 0; i < t.StartPos; i++ {
		s.WriteRune(' ')
	}
	s.WriteString(t.Value)
	return s.String()
}

type lexerState int

const (
	lexerStateInit lexerState = iota
	lexerStateValue
	lexerStateValueContinue
	lexerStateInBracket
)

type lexerCtx struct {
	Pos int

	State   lexerState
	Builder *strings.Builder

	r *bufio.Reader
}

func (c *lexerCtx) discard() {
	c.Pos++
	c.r.Discard(1)
}

func (c *lexerCtx) peek() (rune, error) {
	b, err := c.r.Peek(1)
	if err != nil {
		return 0, err
	}

	return rune(b[0]), nil
}

func (c *lexerCtx) skipWhiteSpace() error {
	for {
		b, err := c.peek()
		if err != nil {
			return err
		}
		if isWhiteSpace(b) {
			c.discard()
			continue
		}
		break
	}

	return nil
}

type Lexer struct {
	ctx *lexerCtx
}

func NewLexer(r io.Reader) *Lexer {
	return &Lexer{
		ctx: &lexerCtx{
			r:       bufio.NewReader(r),
			State:   lexerStateInit,
			Builder: new(strings.Builder),
		},
	}
}

func (l *Lexer) Scan() (*PortfileToken, error) {
	var r rune
	for {
		b, err := l.ctx.peek()
		if err == io.EOF {
			return nil, err
		}
		r = b
		break
	}

	switch r {
	case '\n':
		l.ctx.discard()
		l.ctx.Pos = 0
		return &PortfileToken{Type: PortfileTokenLineBreak}, nil
	case '{':
		l.ctx.discard()
		l.ctx.State = lexerStateInBracket
		return &PortfileToken{Type: PortfileTokenLBracket}, nil
	case '}':
		l.ctx.State = lexerStateInit
		l.ctx.discard()
		return &PortfileToken{Type: PortfileTokenRBracket}, nil
	case '#':
		return l.scanComment()
	default:
		return l.scanStatement()
	}
}

func (l *Lexer) scanStatement() (*PortfileToken, error) {
	if err := l.ctx.skipWhiteSpace(); err != nil {
		return nil, err
	}

	startPos := l.ctx.Pos
Loop:
	for {
		b, err := l.ctx.peek()
		if err != nil {
			return nil, err
		}

		switch l.ctx.State {
		case lexerStateValue:
			if isBackSlash(b) {
				l.ctx.discard()
				l.ctx.State = lexerStateValueContinue
				continue
			}
		case lexerStateValueContinue:
			if isWhiteSpace(b) || isLineBreak(b) {
				l.ctx.discard()
				continue
			}
		case lexerStateInBracket:
		default:
			if isWhiteSpace(b) {
				break Loop
			}
		}
		if l.ctx.State == lexerStateValueContinue {
			l.ctx.State = lexerStateValue
		}

		if isLineBreak(b) {
			break
		}

		l.ctx.discard()
		l.ctx.Builder.WriteRune(b)
	}

	value := l.ctx.Builder.String()
	l.ctx.Builder.Reset()

	if err := l.ctx.skipWhiteSpace(); err != nil {
		return nil, err
	}

	switch l.ctx.State {
	case lexerStateInit:
		l.ctx.State = lexerStateValue
	case lexerStateValue:
		l.ctx.State = lexerStateInit
	}
	return &PortfileToken{Type: PortfileTokenIdent, Value: value, StartPos: startPos}, nil
}

func (l *Lexer) scanComment() (*PortfileToken, error) {
	for {
		b, err := l.ctx.peek()
		if err != nil {
			return nil, err
		}
		l.ctx.Pos++
		l.ctx.discard()
		if b == '\n' {
			l.ctx.Pos = 0
			break
		}
		l.ctx.Builder.WriteRune(b)
	}

	value := l.ctx.Builder.String()
	l.ctx.Builder.Reset()
	return &PortfileToken{Type: PortfileTokenComment, Value: value}, nil
}

func isLineBreak(v rune) bool {
	if v == '\n' {
		return true
	}
	return false
}

func isWhiteSpace(v rune) bool {
	if v == ' ' {
		return true
	}
	return false
}

func isBackSlash(v rune) bool {
	if v == '\\' {
		return true
	}
	return false
}
