package ucl

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

func Unmarshal(b []byte, vars map[string]string, v any) error {
	return NewDecoder(bytes.NewReader(b)).Decode(vars, v)
}

func UnmarshalFile(f string, vars map[string]string, v any) error {
	d, err := NewFileDecoder(f)
	if err != nil {
		return err
	}
	return d.Decode(vars, v)
}

type Decoder struct {
	r     *bufio.Reader
	funcs map[string]any
}

func NewDecoder(in io.Reader) *Decoder {
	d := &Decoder{r: bufio.NewReader(in), funcs: make(map[string]any)}
	wd, _ := os.Getwd()
	return d.Funcs(map[string]any{"include": macroInclude(wd), "try_include": macroTryInclude(wd)})
}

func NewFileDecoder(f string) (*Decoder, error) {
	file, err := os.Open(f)
	if err != nil {
		return nil, err
	}
	d := &Decoder{r: bufio.NewReader(file), funcs: make(map[string]any)}
	wd := filepath.Dir(f)
	return d.Funcs(map[string]any{"include": macroInclude(wd), "try_include": macroTryInclude(wd)}), nil
}

func (d *Decoder) Funcs(funcs map[string]any) *Decoder {
	for k, v := range funcs {
		d.funcs[k] = v
	}
	return d
}

func (d *Decoder) Decode(vars map[string]string, v any) error {
	tokens, err := d.tokenize()
	if err != nil {
		return err
	}
	tokens, err = d.preProcess(tokens)
	if err != nil {
		return err
	}
	c := decodeCtx{tokens: tokens, vars: vars}
	return c.unmarshal(v)
}

func (d *Decoder) ToJSON(vars map[string]string) ([]byte, error) {
	var j any
	if err := d.Decode(vars, &j); err != nil {
		return nil, err
	}
	return json.Marshal(j)
}

func (d *Decoder) preProcess(tokens []*token) ([]*token, error) {
	for pos := 0; pos < len(tokens); pos++ {
		switch tokens[pos].Type {
		case tokenTypeLiteral:
			if pos != len(tokens)-1 && tokens[pos+1].Type == tokenTypeLiteral {
				endPos := pos
				depth := 0
			FindEndCurly:
				for ; endPos < len(tokens); endPos++ {
					switch tokens[endPos].Type {
					case tokenTypeLeftCurly:
						depth++
					case tokenTypeRightCurly:
						if depth == 1 {
							break FindEndCurly
						}
						depth--
					}
				}
				if endPos != pos {
					newTokens := append(tokens[:pos+1], append([]*token{{Type: tokenTypeLeftCurly, Value: "{"}}, tokens[pos+1:endPos]...)...)
					newTokens = append(newTokens, &token{Type: tokenTypeRightCurly, Value: "}"})
					newTokens = append(newTokens, tokens[endPos:]...)
					tokens = newTokens
				}
			}
		case tokenTypeDot:
			name := tokens[pos+1].Value
			f, ok := d.funcs[name]
			if !ok {
				return nil, fmt.Errorf("macro %s is not found", name)
			}
			var args string
			endPos := pos
		FindEnd:
			for ; endPos < len(tokens); endPos++ {
				switch tokens[endPos].Type {
				case tokenTypeLiteral:
					args = tokens[endPos].Value
				case tokenTypeSemiColon:
					break FindEnd
				case tokenTypeNewline:
					endPos--
					break FindEnd
				}
			}
			if args[0] == '"' && args[len(args)-1] == '"' {
				args = args[1 : len(args)-1]
			}

			switch fn := f.(type) {
			case func(any, map[string]any) (string, error):
				raw, err := fn(args, nil)
				if err != nil {
					return nil, err
				}
				addTokens, err := NewDecoder(strings.NewReader(raw)).tokenize()
				if err != nil {
					return nil, err
				}
				tokens = append(tokens[:pos], append(addTokens, tokens[endPos+1:]...)...)
			}
		}
	}

	return tokens, nil
}

func macroInclude(workDir string) func(any, map[string]any) (string, error) {
	return func(args any, kwargs map[string]any) (string, error) {
		buf, err := os.ReadFile(filepath.Join(workDir, args.(string)))
		if err != nil {
			return "", err
		}
		return string(buf), nil
	}
}

func macroTryInclude(workDir string) func(any, map[string]any) string {
	f := macroInclude(workDir)
	return func(args any, kwargs map[string]any) string {
		b, err := f(args, kwargs)
		if err != nil {
			return ""
		}
		return b
	}
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
	case tokenTypeDot:
		return "dot"
	case tokenTypeComment:
		return "comment"
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
			if lexCtx.state != lexStateQuote {
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
	vars   map[string]string
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
		return errors.New("unmarshal only supports \"any\" object")
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
				if t.Value[0] == '"' && t.Value[len(t.Value)-1] == '"' {
					key = t.Value[1 : len(t.Value)-1]
				} else {
					key = t.Value
				}
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

func (d *decodeCtx) parseValue(in string) any {
	// Double quoted
	if len(in) >= 2 && in[0] == '"' && in[len(in)-1] == '"' {
		return d.assignVars(in[1 : len(in)-1])
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
	return d.assignVars(in)
}

func (d *decodeCtx) assignVars(in string) string {
	if !strings.Contains(in, "$") {
		return in
	}

	var b strings.Builder
	b.Grow(len(in))
	startPos := -1
	for i, v := range in {
		if v == '$' {
			if i > 0 && in[i-1] != '$' && len(in) > i+1 && in[i+1] != '$' {
				startPos = i + 1
				continue
			}
			if i == 0 && len(in) > 1 && in[i+1] != '$' {
				startPos = 1
				continue
			}
			if len(in) > i+1 && in[i+1] == '$' {
				continue
			}
		}

		if i > 0 && v == '{' && in[i-1] == '$' {
			startPos = i
			continue
		}
		if startPos > 0 {
			if v == '}' {
				key := in[startPos+1 : i]
				startPos = -1
				b.WriteString(d.vars[key])
			} else if (v < '0' || '9' < v) && (v < 'A' || 'Z' < v) && (v < 'a' || 'z' < v) && v != '_' {
				key := in[startPos:i]
				startPos = -1
				b.WriteString(d.vars[key])
				b.WriteRune(v)
			}
		} else {
			b.WriteRune(v)
		}
	}

	if startPos > 0 {
		key := in[startPos:]
		b.WriteString(d.vars[key])
	}
	return b.String()
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
