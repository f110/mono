package ucl

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenize(t *testing.T) {
	cases := []struct {
		In     string
		Tokens []token
	}{
		{ // key and value
			In: `port = 80;`,
			Tokens: []token{
				{Pos: 0, Value: "port"},
				{Pos: 5, Type: tokenTypeEqual, Value: "="},
				{Pos: 7, Value: "80"},
				{Pos: 9, Type: tokenTypeSemiColon, Value: ";"},
			},
		},
		{
			In: `port=80`,
			Tokens: []token{
				{Pos: 0, Value: "port"},
				{Pos: 4, Type: tokenTypeEqual, Value: "="},
				{Pos: 5, Value: "80"},
			},
		},
		{
			In: `port: 80`,
			Tokens: []token{
				{Pos: 0, Value: "port"},
				{Pos: 4, Type: tokenTypeColon, Value: ":"},
				{Pos: 6, Value: "80"},
			},
		},
		{
			In: `
port = 80`,
			Tokens: []token{
				{Pos: 0, Type: tokenTypeNewline},
				{Pos: 1, Value: "port"},
				{Pos: 6, Type: tokenTypeEqual, Value: "="},
				{Pos: 8, Value: "80"},
			},
		},
		{
			In: `port = '80'`,
			Tokens: []token{
				{Pos: 0, Value: "port"},
				{Pos: 5, Type: tokenTypeEqual, Value: "="},
				{Pos: 7, Value: "'80'"},
			},
		},
		{ // section
			In: `port = 80;
foo {
	name = bar;
}`,
			Tokens: []token{
				{Pos: 0, Value: "port"},
				{Pos: 5, Type: tokenTypeEqual, Value: "="},
				{Pos: 7, Value: "80"},
				{Pos: 9, Type: tokenTypeSemiColon, Value: ";"},
				{Pos: 10, Type: tokenTypeNewline},
				{Pos: 11, Value: "foo"},
				{Pos: 15, Type: tokenTypeLeftCurly, Value: "{"},
				{Pos: 16, Type: tokenTypeNewline},
				{Pos: 18, Value: "name"},
				{Pos: 23, Type: tokenTypeEqual, Value: "="},
				{Pos: 25, Value: "bar"},
				{Pos: 28, Type: tokenTypeSemiColon, Value: ";"},
				{Pos: 29, Type: tokenTypeNewline},
				{Pos: 30, Type: tokenTypeRightCurly, Value: "}"},
			},
		},
		{ // named section
			In: `foo bar {
	baz = cuz;
}`,
			Tokens: []token{
				{Pos: 0, Value: "foo"},
				{Pos: 4, Value: "bar"},
				{Pos: 8, Type: tokenTypeLeftCurly, Value: "{"},
				{Pos: 9, Type: tokenTypeNewline},
				{Pos: 11, Value: "baz"},
				{Pos: 15, Type: tokenTypeEqual, Value: "="},
				{Pos: 17, Value: "cuz"},
				{Pos: 20, Type: tokenTypeSemiColon, Value: ";"},
				{Pos: 21, Type: tokenTypeNewline},
				{Pos: 22, Type: tokenTypeRightCurly, Value: "}"},
			},
		},
		{ // macro
			In: `.include "local.conf"`,
			Tokens: []token{
				{Pos: 0, Type: tokenTypeDot, Value: "."},
				{Pos: 1, Value: "include"},
				{Pos: 9, Value: "\"local.conf\""},
			},
		},
		{ // single line comment
			In: `# simple
port = 80;`,
			Tokens: []token{
				{Pos: 0, Type: tokenTypeComment, Value: "# simple"},
				{Pos: 8, Type: tokenTypeNewline},
				{Pos: 9, Value: "port"},
				{Pos: 14, Type: tokenTypeEqual, Value: "="},
				{Pos: 16, Value: "80"},
				{Pos: 18, Type: tokenTypeSemiColon, Value: ";"},
			},
		},
		{ // multiline comments
			In: `/*
  foo
*/
port = 80;`,
			Tokens: []token{
				{Pos: 0, Type: tokenTypeComment, Value: "/*"},
				{Pos: 2, Type: tokenTypeNewline},
				{Pos: 3, Value: "  foo"},
				{Pos: 8, Type: tokenTypeNewline},
				{Pos: 9, Type: tokenTypeComment, Value: "*/"},
				{Pos: 11, Type: tokenTypeNewline},
				{Pos: 12, Value: "port"},
				{Pos: 17, Type: tokenTypeEqual, Value: "="},
				{Pos: 19, Value: "80"},
				{Pos: 21, Type: tokenTypeSemiColon, Value: ";"},
			},
		},
		{ // Nested multiline comments
			In: `/*
  foo
  /* bar */
*/
port = 80`,
			Tokens: []token{
				{Pos: 0, Type: tokenTypeComment, Value: "/*"},
				{Pos: 2, Type: tokenTypeNewline},
				{Pos: 3, Value: "  foo"},
				{Pos: 8, Type: tokenTypeNewline},
				{Pos: 9, Value: "  /* bar */"},
				{Pos: 20, Type: tokenTypeNewline},
				{Pos: 21, Type: tokenTypeComment, Value: "*/"},
				{Pos: 23, Type: tokenTypeNewline},
				{Pos: 24, Value: "port"},
				{Pos: 29, Type: tokenTypeEqual, Value: "="},
				{Pos: 31, Value: "80"},
			},
		},
		{ // multiline strings
			In: `key = <<EOD
foo
bar
EOD`,
			Tokens: []token{
				{Pos: 0, Value: "key"},
				{Pos: 4, Type: tokenTypeEqual, Value: "="},
				{Pos: 6, Type: tokenTypeSymbol, Value: "<<"},
				{Pos: 8, Value: `EOD`},
				{Pos: 11, Type: tokenTypeNewline},
				{Pos: 12, Value: "foo"},
				{Pos: 15, Type: tokenTypeNewline},
				{Pos: 16, Value: "bar"},
				{Pos: 19, Type: tokenTypeNewline},
				{Pos: 20, Value: "EOD"},
			},
		},
		{
			In: `.macro_name(param=value) "something";`,
			Tokens: []token{
				{Pos: 0, Type: tokenTypeDot, Value: "."},
				{Pos: 1, Value: "macro_name"},
				{Pos: 11, Type: tokenTypeLeftParen, Value: "("},
				{Pos: 12, Value: "param"},
				{Pos: 17, Type: tokenTypeEqual, Value: "="},
				{Pos: 18, Value: "value"},
				{Pos: 23, Type: tokenTypeRightParen, Value: ")"},
				{Pos: 25, Value: "\"something\""},
				{Pos: 36, Type: tokenTypeSemiColon, Value: ";"},
			},
		},
		{
			In: `.macro_name(param={key=value}) "something";`,
			Tokens: []token{
				{Pos: 0, Type: tokenTypeDot, Value: "."},
				{Pos: 1, Value: "macro_name"},
				{Pos: 11, Type: tokenTypeLeftParen, Value: "("},
				{Pos: 12, Value: "param"},
				{Pos: 17, Type: tokenTypeEqual, Value: "="},
				{Pos: 18, Type: tokenTypeLeftCurly, Value: "{"},
				{Pos: 19, Value: "key"},
				{Pos: 22, Type: tokenTypeEqual, Value: "="},
				{Pos: 23, Value: "value"},
				{Pos: 28, Type: tokenTypeRightCurly, Value: "}"},
				{Pos: 29, Type: tokenTypeRightParen, Value: ")"},
				{Pos: 31, Value: "\"something\""},
				{Pos: 42, Type: tokenTypeSemiColon, Value: ";"},
			},
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			d := NewDecoder(strings.NewReader(tc.In))
			tokens := d.tokenize()
			assert.Equal(t, len(tc.Tokens), len(tokens))
			for i, v := range tokens {
				assert.Equal(t, tc.Tokens[i], *v)
			}
		})
	}
}
