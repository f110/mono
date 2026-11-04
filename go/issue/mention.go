package issue

import (
	"regexp"
	"slices"
	"strconv"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// mentionPattern is the same as the pattern which the Web UI replaces with the link to the ticket.
var mentionPattern = regexp.MustCompile(`#(\d+)\b`)

var markdown = goldmark.New(goldmark.WithExtensions(extension.GFM))

// parseMentions returns the numbers of the tickets mentioned by "#123" in body in the order of the first appearance.
// The mentions in the code, the links, the images and the HTML blocks are ignored as the Web UI does not make the link
// of them.
func parseMentions(body string) []int32 {
	src := []byte(body)
	var res []int32
	add := func(s []byte) {
		for _, m := range mentionPattern.FindAllSubmatch(s, -1) {
			n, err := strconv.ParseInt(string(m[1]), 10, 32)
			if err != nil {
				continue
			}
			if !slices.Contains(res, int32(n)) {
				res = append(res, int32(n))
			}
		}
	}
	walkMentionText(markdown.Parser().Parse(text.NewReader(src)), src, add)
	return res
}

// walkMentionText calls fn with the text of the adjacent text nodes joined because goldmark splits the text at the
// characters which may start the inline elements.
func walkMentionText(n ast.Node, src []byte, fn func([]byte)) {
	var buf []byte
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			buf = append(buf, t.Segment.Value(src)...)
			if t.SoftLineBreak() || t.HardLineBreak() {
				buf = append(buf, '\n')
			}
			continue
		}
		fn(buf)
		buf = nil
		switch c.Kind() {
		case ast.KindLink, ast.KindAutoLink, ast.KindImage, ast.KindCodeSpan, ast.KindCodeBlock, ast.KindFencedCodeBlock, ast.KindHTMLBlock, ast.KindRawHTML:
			continue
		}
		walkMentionText(c, src, fn)
	}
	fn(buf)
}
