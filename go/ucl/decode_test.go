package ucl

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToJSON(t *testing.T) {
	cases := []struct {
		In   string
		JSON any
	}{
		{
			In:   `port = 80;`,
			JSON: map[string]int{"port": 80},
		},
		{
			In:   `port: 80`,
			JSON: map[string]int{"port": 80},
		},
		{
			In:   `port = -80`,
			JSON: map[string]int{"port": -80},
		},
		{
			In:   `port = 80-80`,
			JSON: map[string]string{"port": "80-80"},
		},
		{
			In: `port = 80;
name = foo`,
			JSON: map[string]any{"port": 80, "name": "foo"},
		},
		{
			In: `foo {
  bar = baz;
}`,
			JSON: map[string]any{"foo": map[string]string{"bar": "baz"}},
		},
		{
			In: `foo {
  bar = baz;
}
baz {
  qux = alice;
}`,
			JSON: map[string]any{"foo": map[string]string{"bar": "baz"}, "baz": map[string]string{"qux": "alice"}},
		},
		{
			In: `foo bar {
  baz = qux;
}`,
			JSON: map[string]any{"foo": map[string]any{"bar": map[string]string{"baz": "qux"}}},
		},
		{
			In: `port = 80;
port = 443;`,
			JSON: map[string]any{"port": []any{80, 443}},
		},
		{
			In: `port = 80;
port = https;`,
			JSON: map[string]any{"port": []any{80, "https"}},
		},
		{
			In:   `port = "80";`,
			JSON: map[string]any{"port": "80"},
		},
		{
			In: `port = 1k;
port2 = 1K;`,
			JSON: map[string]any{"port": 1000, "port2": 1000},
		},
		{
			In: `port = 1m;
port2 = 1M;`,
			JSON: map[string]any{"port": 1000_000, "port2": 1000_000},
		},
		{
			In: `port = 1g;
port2 = 1G;`,
			JSON: map[string]any{"port": 1000_000_000, "port2": 1000_000_000},
		},
		{
			In: `port = 1kb;
port2 = 1Kb;`,
			JSON: map[string]any{"port": 1 << 10, "port2": 1 << 10},
		},
		{
			In: `port = 1mb;
port2 = 1Mb;`,
			JSON: map[string]any{"port": 1 << 20, "port2": 1 << 20},
		},
		{
			In: `port = 1gb;
port2 = 1Gb;`,
			JSON: map[string]any{"port": 1 << 30, "port2": 1 << 30},
		},
		{
			In: `flag = true;
flag2 = yes;
flag3 = on;`,
			JSON: map[string]any{"flag": true, "flag2": true, "flag3": true},
		},
		{
			In: `flag = false;
flag2 = no;
flag3 = off;`,
			JSON: map[string]any{"flag": false, "flag2": false, "flag3": false},
		},
		{
			In:   `v1 = "80"; v2 = "1k"; v3 = "1kb"; v4 = "true"; v5 = "false"`,
			JSON: map[string]any{"v1": "80", "v2": "1k", "v3": "1kb", "v4": "true", "v5": "false"},
		},
		{
			In: `desc = <<EOD
foo
bar
EOD`,
			JSON: map[string]any{"desc": "foo\nbar"},
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			j, err := NewDecoder(strings.NewReader(tc.In)).ToJSON(nil)
			require.NoError(t, err)
			b, err := json.Marshal(tc.JSON)
			require.NoError(t, err)
			assert.JSONEq(t, string(b), string(j))
			t.Log(string(j))
		})
	}
}

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
			In: `port = -80;`,
			Tokens: []token{
				{Pos: 0, Value: "port"},
				{Pos: 5, Type: tokenTypeEqual, Value: "="},
				{Pos: 7, Value: "-80"},
				{Pos: 10, Type: tokenTypeSemiColon, Value: ";"},
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
				{Pos: 0, Type: tokenTypeNewline, Value: "\n"},
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
				{Pos: 10, Type: tokenTypeNewline, Value: "\n"},
				{Pos: 11, Value: "foo"},
				{Pos: 15, Type: tokenTypeLeftCurly, Value: "{"},
				{Pos: 16, Type: tokenTypeNewline, Value: "\n"},
				{Pos: 18, Value: "name"},
				{Pos: 23, Type: tokenTypeEqual, Value: "="},
				{Pos: 25, Value: "bar"},
				{Pos: 28, Type: tokenTypeSemiColon, Value: ";"},
				{Pos: 29, Type: tokenTypeNewline, Value: "\n"},
				{Pos: 30, Type: tokenTypeRightCurly, Value: "}"},
			},
		},
		{ // named section
			In: `foo bar {
	baz = qux;
}`,
			Tokens: []token{
				{Pos: 0, Value: "foo"},
				{Pos: 4, Value: "bar"},
				{Pos: 8, Type: tokenTypeLeftCurly, Value: "{"},
				{Pos: 9, Type: tokenTypeNewline, Value: "\n"},
				{Pos: 11, Value: "baz"},
				{Pos: 15, Type: tokenTypeEqual, Value: "="},
				{Pos: 17, Value: "qux"},
				{Pos: 20, Type: tokenTypeSemiColon, Value: ";"},
				{Pos: 21, Type: tokenTypeNewline, Value: "\n"},
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
				{Pos: 8, Type: tokenTypeNewline, Value: "\n"},
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
				{Pos: 2, Type: tokenTypeNewline, Value: "\n"},
				{Pos: 3, Value: "  foo"},
				{Pos: 8, Type: tokenTypeNewline, Value: "\n"},
				{Pos: 9, Type: tokenTypeComment, Value: "*/"},
				{Pos: 11, Type: tokenTypeNewline, Value: "\n"},
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
				{Pos: 2, Type: tokenTypeNewline, Value: "\n"},
				{Pos: 3, Value: "  foo"},
				{Pos: 8, Type: tokenTypeNewline, Value: "\n"},
				{Pos: 9, Value: "  /* bar */"},
				{Pos: 20, Type: tokenTypeNewline, Value: "\n"},
				{Pos: 21, Type: tokenTypeComment, Value: "*/"},
				{Pos: 23, Type: tokenTypeNewline, Value: "\n"},
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
				{Pos: 11, Type: tokenTypeNewline, Value: "\n"},
				{Pos: 12, Value: "foo"},
				{Pos: 15, Type: tokenTypeNewline, Value: "\n"},
				{Pos: 16, Value: "bar"},
				{Pos: 19, Type: tokenTypeNewline, Value: "\n"},
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
			tokens, err := d.tokenize()
			require.NoError(t, err)
			assert.Equal(t, len(tc.Tokens), len(tokens))
			for i, v := range tokens {
				assert.Equal(t, tc.Tokens[i], *v)
			}
		})
	}
}
