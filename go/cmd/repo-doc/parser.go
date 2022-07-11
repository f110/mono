package main

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
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
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(),
	)
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
				if parent.Level == heading.Level-1 {
					break
				}
				parent = parent.Parent
			}

			n := &tableOfContent{Title: string(n.Text(in)), Parent: parent.Parent, Level: heading.Level - 1}
			parent.Parent.Child = append(parent.Parent.Child, n)
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
