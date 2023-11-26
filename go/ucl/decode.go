package ucl

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
)

func Unmarshal(b []byte, vars map[string]any, v any) error {
	return NewDecoder(bytes.NewReader(b)).Decode(vars, v)
}

type Decoder struct {
	r *bufio.Reader
}

func NewDecoder(in io.Reader) *Decoder {
	return &Decoder{r: bufio.NewReader(in)}
}

func (d *Decoder) Decode(vars map[string]any, v any) error {
	tokens, err := d.tokenize()
	if err != nil {
		return err
	}
	c := decodeCtx{tokens: tokens, vars: vars}
	return c.unmarshal(v)
}

func (d *Decoder) ToJSON(vars map[string]any) ([]byte, error) {
	var j any
	if err := d.Decode(vars, &j); err != nil {
		return nil, err
	}
	return json.Marshal(j)
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

func (v tokenType) String() string {
	switch v {
	case tokenTypeLiteral:
		return "literal"
	case tokenTypeEqual:
		return "equal"
	case tokenTypeLeftCurly:
		return "left curly bracket"
	case tokenTypeRightCurly:
		return "right curly bracket"
	case tokenTypeNewline:
		return "new-line"
	case tokenTypeSemiColon:
		return "semicolon"
	case tokenTypeColon:
		return "colon"
	}

	return fmt.Sprintf("%d", v)
}

func (d *Decoder) tokenize() ([]*token, error) {
	var tokens []*token

	lexCtx := &lexerCtx{r: d.r, next: &token{}}
	for {
		b, err := lexCtx.nextByte()
		if errors.Is(err, io.EOF) {
			if lexCtx.peekOffset > 1 {
				t, err := lexCtx.nextToken(false)
				if err != nil {
					return nil, err
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
						return nil, err
					}
					tokens = append(tokens, t...)
				}
				lexCtx.discard()
			}
		case '{', '}', '(', ')', '.', '=', ';', ':':
			if lexCtx.state == lexStateNormal {
				t, err := lexCtx.nextToken(false)
				if err != nil {
					return nil, err
				}
				tokens = append(tokens, t...)
			}
		case '"':
			if lexCtx.state == lexStateNormal {
				lexCtx.state = lexStateQuote
			} else if lexCtx.state == lexStateQuote {
				t, err := lexCtx.nextToken(true)
				if err != nil {
					return nil, err
				}
				tokens = append(tokens, t...)
				lexCtx.state = lexStateNormal
			}
		case '#':
			if lexCtx.state == lexStateNormal {
				t, err := lexCtx.nextTokenTillEndOfLine()
				if err != nil {
					return nil, err
				}
				t.Type = tokenTypeComment
				tokens = append(tokens, t)
			}
		case '/':
			n, err := lexCtx.peek(1)
			if err != nil {
				return nil, err
			}
			if n == '*' { // /* is multiline comments
				lexCtx.state = lexStateComment
				if lexCtx.depth == 0 {
					t, err := lexCtx.nextToken(true)
					if err != nil {
						return nil, err
					}
					tokens = append(tokens, t...)
				}
				lexCtx.depth += 1
			}
		case '*':
			n, err := lexCtx.peek(1)
			if err != nil {
				return nil, err
			}
			if n == '/' { // */ is end of multiline comments
				if lexCtx.depth == 1 {
					lexCtx.state = lexStateNormal
					t, err := lexCtx.nextToken(true)
					if err != nil {
						return nil, err
					}
					tokens = append(tokens, t...)
				}
				lexCtx.depth--
			}
		case '<':
			n, err := lexCtx.peek(1)
			if err != nil {
				return nil, err
			}
			if n == '<' { // << is starting multiline strings
				t, err := lexCtx.nextToken(true)
				if err != nil {
					return nil, err
				}
				tokens = append(tokens, t...)
			}
		case '\n':
			t, err := lexCtx.nextToken(false)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, t...)
		case '\t':
			lexCtx.discard()
		}
	}

	return tokens, nil
}

type decodeCtx struct {
	tokens []*token
	vars   map[string]any
	pos    int
}

func (d *decodeCtx) unmarshal(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Interface && rv.NumMethod() == 0 {
		root := make(map[string]any)
		err := d.unmarshalObject(root)
		if err != nil {
			return err
		}
		rv.Set(reflect.ValueOf(root))
	} else {
		return errors.New("unmarshal only supports any object")
	}
	return nil
}

func (d *decodeCtx) unmarshalObject(parent map[string]any) error {
	key := ""
	symbol := 0
	for ; d.pos < len(d.tokens); d.pos++ {
		t := d.tokens[d.pos]
		switch t.Type {
		case tokenTypeLiteral:
			if key == "" {
				key = t.Value
				continue
			}
			if symbol != 0 && d.tokens[symbol].Value == t.Value {
				val := ""
				for _, v := range d.tokens[symbol+2 : d.pos-1] {
					val += v.Value
				}
				d.setObjectValue(parent, key, val)
				key = ""
				symbol = 0
				continue
			}

			if symbol == 0 {
				child := make(map[string]any)
				err := d.unmarshalObject(child)
				if err != nil {
					return err
				}
				d.setObjectValue(parent, key, child)
				key = ""
			}
		case tokenTypeEqual, tokenTypeColon:
			if d.tokens[d.pos+1].Type == tokenTypeLiteral {
				d.setObjectValue(parent, key, d.parseValue(d.tokens[d.pos+1].Value))
				key = ""
				d.pos++
			}
		case tokenTypeLeftCurly:
			d.pos++
			child := make(map[string]any)
			err := d.unmarshalObject(child)
			if err != nil {
				return err
			}
			d.setObjectValue(parent, key, child)
			key = ""
		case tokenTypeRightCurly:
			return nil
		case tokenTypeSymbol:
			if symbol == 0 && d.tokens[d.pos+1].Type == tokenTypeLiteral {
				symbol = d.pos + 1
				d.pos++
			}
		}
	}
	return nil
}

func (*decodeCtx) setObjectValue(obj map[string]any, key string, value any) {
	if v, ok := obj[key]; ok {
		switch t := v.(type) {
		case []any:
			obj[key] = append(t, value)
		default:
			obj[key] = []any{v, value}
		}
	} else {
		obj[key] = value
	}
}

func (*decodeCtx) parseValue(in string) any {
	// Double quoted
	if len(in) >= 2 && in[0] == '"' && in[len(in)-1] == '"' {
		return in[1 : len(in)-1]
	}

	switch in {
	case "true", "yes", "on":
		return true
	case "false", "no", "off":
		return false
	}

	// Prefix (10 base multipliers)
	if len(in) >= 2 && isIntegerValue(in[:len(in)-1]) {
		prefix := int64(1)
		switch in[len(in)-1] {
		case 'k', 'K':
			prefix = 1000
		case 'm', 'M':
			prefix = 1000_000
		case 'g', 'G':
			prefix = 1000_000_000
		}
		if prefix != 1 {
			v, _ := strconv.ParseInt(in[:len(in)-1], 10, 64)
			return v * prefix
		}
	}
	// 2 power multipliers
	if len(in) >= 3 && in[len(in)-1] == 'b' && isIntegerValue(in[:len(in)-2]) {
		shift := int64(0)
		switch in[len(in)-2] {
		case 'k', 'K':
			shift = 10
		case 'm', 'M':
			shift = 20
		case 'g', 'G':
			shift = 30
		}
		if shift != 0 {
			v, _ := strconv.ParseInt(in[:len(in)-2], 10, 64)
			return v << shift
		}
	}

	if isIntegerValue(in) {
		v, _ := strconv.ParseInt(in, 10, 64)
		return v
	}
	return in
}

func isIntegerValue(in string) bool {
	isInteger := true
	for i, v := range in {
		if (v < '0' || v > '9') && !(i == 0 && v == '-') { // integer
			isInteger = false
		}
	}
	return isInteger
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
		next.Value = "\n"
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
