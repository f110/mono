package main

import (
	"bytes"

	"github.com/abhinav/goldmark-mermaid"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark-highlighting"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.f110.dev/xerrors"
)

type document struct {
	Title           string
	Content         string
	TableOfContents *tableOfContent
}

type tableOfContent struct {
	Title  string
	Level  int
	Child  []*tableOfContent
	Parent *tableOfContent
}

type markdownParser struct {
	rp goldmark.Markdown
}

func newMarkdownParser() *markdownParser {
	htmlRender := html.NewRenderer().(*html.Renderer)

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(highlighting.WithStyle("monokai")),
			&mermaid.Extender{},
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRenderer(
			renderer.NewRenderer(renderer.WithNodeRenderers(util.Prioritized(htmlRender, 1000))),
		),
	)
	md.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(newMarkdownExtendedRenderer(htmlRender), 1)))
	return &markdownParser{rp: md}
}

func (m *markdownParser) Parse(in []byte) (*document, error) {
	node := m.rp.Parser().Parse(text.NewReader(in))

	// Add class attr
	err := ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if n.Kind() == east.KindTable {
				n.SetAttributeString("class", []byte("ui striped table"))
			}
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	buf := new(bytes.Buffer)
	if err := m.rp.Renderer().Render(buf, in, node); err != nil {
		return nil, xerrors.WithStack(err)
	}

	doc := &document{Content: buf.String(), TableOfContents: &tableOfContent{}}
	// Find document title
	child := node.FirstChild()
	for child != nil {
		if v, ok := child.(*ast.Heading); !ok {
			child = child.NextSibling()
			continue
		} else {
			if v.Level == 1 {
				doc.Title = string(v.Text(in))
				break
			}
		}
	}

	if err := m.makeTableOfContent(node, doc.TableOfContents, in); err != nil {
		return nil, err
	}
	return doc, nil
}

func (m *markdownParser) makeTableOfContent(node ast.Node, toc *tableOfContent, in []byte) error {
	prevToC := toc
	currentLevel := 0
	err := ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		heading, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		if !entering {
			return ast.WalkContinue, nil
		}
		// Level 1 is a document title
		if heading.Level < 2 {
			return ast.WalkContinue, nil
		}

		if currentLevel < heading.Level {
			n := &tableOfContent{Title: string(n.Text(in)), Parent: prevToC, Level: heading.Level - 1}
			prevToC.Child = append(prevToC.Child, n)
			prevToC = n
			currentLevel = heading.Level
		} else if currentLevel > heading.Level {
			parent := prevToC.Parent
			for {
				if parent.Level < heading.Level-1 {
					break
				}
				parent = parent.Parent
			}

			n := &tableOfContent{Title: string(n.Text(in)), Parent: parent, Level: heading.Level - 1}
			parent.Child = append(parent.Child, n)
			prevToC = n
			currentLevel = heading.Level
		} else {
			n := &tableOfContent{Title: string(n.Text(in)), Parent: prevToC.Parent, Level: heading.Level - 1}
			prevToC.Parent.Child = append(prevToC.Parent.Child, n)
			prevToC = n
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

type markdownExtendedRenderer struct {
	htmlRenderer *html.Renderer
}

var _ renderer.NodeRenderer = &markdownExtendedRenderer{}

func newMarkdownExtendedRenderer(r *html.Renderer) *markdownExtendedRenderer {
	return &markdownExtendedRenderer{htmlRenderer: r}
}

func (r *markdownExtendedRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindImage, r.renderImage)
}

func (r *markdownExtendedRenderer) renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Image)
	_, _ = w.WriteString("<img src=\"")
	if r.htmlRenderer.Unsafe || !html.IsDangerousURL(n.Destination) {
		_, _ = w.Write(util.EscapeHTML(util.URLEscape(n.Destination, true)))
	}
	_, _ = w.WriteString(`" alt="`)
	_, _ = w.Write(util.EscapeHTML(n.Text(source)))
	_ = w.WriteByte('"')
	if n.Title != nil {
		_, _ = w.WriteString(` title="`)
		r.htmlRenderer.Writer.Write(w, n.Title)
		_ = w.WriteByte('"')
	}
	if n.Attributes() != nil {
		html.RenderAttributes(w, n, html.ImageAttributeFilter)
	}
	if r.htmlRenderer.XHTML {
		_, _ = w.WriteString(" />")
	} else {
		_, _ = w.WriteString(">")
	}
	return ast.WalkSkipChildren, nil
}
