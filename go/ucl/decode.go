package ucl

import (
	"bufio"
	"errors"
	"io"
)

func Unmarshal(b []byte, v any) error {
	return nil
}

type Decoder struct {
	r *bufio.Reader
}

func NewDecoder(in io.Reader) *Decoder {
	return &Decoder{r: bufio.NewReader(in)}
}

type token struct {
	Type  tokenType
	Pos   int
	Value string
}

type tokenType int

const (
	tokenTypeLiteral tokenType = iota
	tokenTypeEqual
	tokenTypeLeftCurly
	tokenTypeRightCurly
	tokenTypeLeftParen
	tokenTypeRightParen
	tokenTypeDot
	tokenTypeComment
	tokenTypeSymbol
	tokenTypeNewline
	tokenTypeSemiColon
	tokenTypeColon
)

func (d *Decoder) tokenize() []*token {
	var tokens []*token

	lexCtx := &lexerCtx{r: d.r, next: &token{}}
	count := 0
	for ; ; count++ {
		b, err := lexCtx.nextByte()
		if errors.Is(err, io.EOF) {
			if lexCtx.peekOffset > 1 {
				t, err := lexCtx.nextToken(false)
				if err != nil {
					return nil
				}
				tokens = append(tokens, t...)
			}
			break
		}

		switch b {
		case ' ':
			if lexCtx.state == lexStateNormal {
				if lexCtx.peekOffset != 1 {
					t, err := lexCtx.nextToken(false)
					if err != nil {
						return nil
					}
					tokens = append(tokens, t...)
				}
				lexCtx.discard()
			}
		case '{', '}', '(', ')', '.', '=', ';', ':':
			if lexCtx.state == lexStateNormal {
				t, err := lexCtx.nextToken(false)
				if err != nil {
					return nil
				}
				tokens = append(tokens, t...)
			}
		case '"':
			if lexCtx.state == lexStateNormal {
				lexCtx.state = lexStateQuote
			} else if lexCtx.state == lexStateQuote {
				t, err := lexCtx.nextToken(true)
				if err != nil {
					return nil
				}
				tokens = append(tokens, t...)
				lexCtx.state = lexStateNormal
			}
		case '#':
			if lexCtx.state == lexStateNormal {
				t, err := lexCtx.nextTokenTillEndOfLine()
				if err != nil {
					return nil
				}
				t.Type = tokenTypeComment
				tokens = append(tokens, t)
			}
		case '/':
			n, err := lexCtx.peek(1)
			if err != nil {
				return nil
			}
			if n == '*' { // /* is multiline comments
				lexCtx.state = lexStateComment
				if lexCtx.depth == 0 {
					t, err := lexCtx.nextToken(true)
					if err != nil {
						return nil
					}
					tokens = append(tokens, t...)
				}
				lexCtx.depth += 1
			}
		case '*':
			n, err := lexCtx.peek(1)
			if err != nil {
				return nil
			}
			if n == '/' { // */ is end of multiline comments
				if lexCtx.depth == 1 {
					lexCtx.state = lexStateNormal
					t, err := lexCtx.nextToken(true)
					if err != nil {
						return nil
					}
					tokens = append(tokens, t...)
				}
				lexCtx.depth--
			}
		case '<':
			n, err := lexCtx.peek(1)
			if err != nil {
				return nil
			}
			if n == '<' { // << is starting multiline strings
				t, err := lexCtx.nextToken(true)
				if err != nil {
					return nil
				}
				tokens = append(tokens, t...)
			}
		case '\n':
			t, err := lexCtx.nextToken(false)
			if err != nil {
				return nil
			}
			tokens = append(tokens, t...)
		case '\t':
			lexCtx.discard()
		}
	}

	return tokens
}

type lexerCtx struct {
	r          *bufio.Reader
	state      int
	pos        int
	peekOffset int
	byte       byte
	depth      int
	next       *token
}

const (
	lexStateNormal = iota
	lexStateQuote
	lexStateComment
	lexStateMultilineStrings
)

func (c *lexerCtx) nextToken(includeCurrent bool) ([]*token, error) {
	var tokens []*token
	next := c.next

	if c.peekOffset > 1 {
		offset := 1
		if includeCurrent {
			offset = 0
		}
		buf := make([]byte, c.peekOffset-offset)
		_, err := c.r.Read(buf)
		if err != nil {
			return nil, err
		}
		t := &token{Pos: c.pos, Value: string(buf)}
		tokens = append(tokens, t)
		c.pos += len(buf)
	}

	next.Pos = c.pos
	switch c.byte {
	case '=':
		next.Type = tokenTypeEqual
		next.Value = "="
		c.discard()
	case '{':
		next.Type = tokenTypeLeftCurly
		next.Value = "{"
		c.discard()
	case '}':
		next.Type = tokenTypeRightCurly
		next.Value = "}"
		c.discard()
	case '(':
		next.Type = tokenTypeLeftParen
		next.Value = "("
		c.discard()
	case ')':
		next.Type = tokenTypeRightParen
		next.Value = ")"
		c.discard()
	case '.':
		next.Type = tokenTypeDot
		next.Value = "."
		c.discard()
	case '<':
		next.Type = tokenTypeSymbol
		next.Value = "<<"
		c.discard()
		c.discard()
	case '\n':
		next.Type = tokenTypeNewline
		c.discard()
	case '/':
		n, err := c.peek(1)
		if err != nil {
			return nil, err
		}
		if n == '*' {
			next.Type = tokenTypeComment
			next.Value = "/*"
			c.discard()
			c.discard()
		}
	case '*':
		n, err := c.peek(1)
		if err != nil {
			return nil, err
		}
		if n == '/' {
			next.Type = tokenTypeComment
			next.Value = "*/"
			c.discard()
			c.discard()
		}
	case ';':
		next.Type = tokenTypeSemiColon
		next.Value = ";"
		c.discard()
	case ':':
		next.Type = tokenTypeColon
		next.Value = ":"
		c.discard()
	default:
		next = nil
		//default:
		//	offset := 1
		//	if includeCurrent {
		//		offset = 0
		//	}
		//	buf := make([]byte, c.peekOffset-offset)
		//	n, err := c.r.Read(buf)
		//	if errors.Is(err, io.EOF) {
		//		next = nil
		//		break
		//	}
		//	if err != nil {
		//		return nil, err
		//	}
		//	if n != len(buf) {
		//		return nil, errors.New("short buffer")
		//	}
		//	next.Value = string(buf)
		//	c.pos += len(buf)
	}

	c.next = &token{}
	c.peekOffset = 0
	if next != nil {
		tokens = append(tokens, next)
	}
	return tokens, nil
}

func (c *lexerCtx) nextTokenTillEndOfLine() (*token, error) {
	next := c.next

	for {
		n, err := c.peek(1)
		if err != nil {
			return nil, err
		}
		if n == '\n' {
			break
		}
		_, err = c.nextByte()
		if err != nil {
			return nil, err
		}
	}
	line := make([]byte, c.peekOffset)
	if _, err := c.r.Read(line); err != nil {
		return nil, err
	}
	next.Value = string(line)
	c.pos += len(line)

	c.next = &token{}
	c.peekOffset = 0
	return next, nil
}

func (c *lexerCtx) forward(offset int) {
	c.peekOffset += offset
}

func (c *lexerCtx) nextByte() (byte, error) {
	c.peekOffset++
	b, err := c.r.Peek(c.peekOffset)
	if err != nil {
		return 0, err
	}
	c.byte = b[c.peekOffset-1]

	return b[c.peekOffset-1], nil
}

func (c *lexerCtx) prevByte() byte {
	b, _ := c.r.Peek(c.peekOffset)
	return b[c.peekOffset-2]
}

func (c *lexerCtx) peek(offset int) (byte, error) {
	b, err := c.r.Peek(c.peekOffset + offset)
	if err != nil {
		return 0, err
	}
	return b[c.peekOffset+offset-1], nil
}

func (c *lexerCtx) discard() {
	c.peekOffset = 0
	c.pos++
	c.r.Discard(1)
}
