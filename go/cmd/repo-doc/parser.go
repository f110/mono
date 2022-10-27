package main

import (
	"bytes"
	"container/list"
	"fmt"

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
	Path            string
	Content         string
	TableOfContents *tableOfContent
}

type tableOfContent struct {
	Title  string
	Anchor string
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
		goldmark.WithParserOptions(),
		goldmark.WithRenderer(
			renderer.NewRenderer(renderer.WithNodeRenderers(util.Prioritized(htmlRender, 1000))),
		),
	)
	md.Parser().AddOptions(parser.WithASTTransformers(util.Prioritized(newMarkdownAutoHeadingIDTransformer(), 1)))
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
			child = child.NextSibling()
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

		var anchor string
		if v, ok := n.Attribute([]byte("id")); ok {
			anchor = string(v.([]uint8))
		}
		if currentLevel < heading.Level {
			n := &tableOfContent{Title: string(n.Text(in)), Anchor: anchor, Parent: prevToC, Level: heading.Level - 1}
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

			n := &tableOfContent{Title: string(n.Text(in)), Anchor: anchor, Parent: parent, Level: heading.Level - 1}
			parent.Child = append(parent.Child, n)
			prevToC = n
			currentLevel = heading.Level
		} else {
			n := &tableOfContent{Title: string(n.Text(in)), Anchor: anchor, Parent: prevToC.Parent, Level: heading.Level - 1}
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

type markdownAutoHeadingIDTransformer struct{}

var _ parser.ASTTransformer = &markdownAutoHeadingIDTransformer{}

func newMarkdownAutoHeadingIDTransformer() *markdownAutoHeadingIDTransformer {
	return &markdownAutoHeadingIDTransformer{}
}

func (m *markdownAutoHeadingIDTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	q := list.New()
	marked := make(map[ast.Node]struct{})
	marked[node] = struct{}{}
	q.PushBack(ast.Node(node))
	for q.Len() > 0 {
		e := q.Front()
		q.Remove(e)
		n := e.Value.(ast.Node)

		if n.Kind() == ast.KindHeading {
			heading := n.(*ast.Heading)
			if heading.Level > 1 {
				_, ok := n.AttributeString("id")
				if !ok {
					t := n.Text(reader.Source())
					t = bytes.Replace(t, []byte(" "), []byte("-"), -1)
					n.SetAttribute([]byte("id"), []byte(fmt.Sprintf("user-content-%s", t)))
				}
			}
		}

		next := n.FirstChild()
		for next != nil {
			if _, ok := marked[next]; !ok {
				marked[next] = struct{}{}
				q.PushBack(next)
			}
			next = next.NextSibling()
		}
	}
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
