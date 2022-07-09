package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeTableOfContents(t *testing.T) {
	raw := `# document title
## first
## second
## third
### fourth
### fifth
## sixth
### seventh`
	p := newMarkdownParser()
	doc, err := p.Parse([]byte(raw))
	require.NoError(t, err)

	if assert.Len(t, doc.TableOfContents.Child, 4) {
		assert.Equal(t, "first", doc.TableOfContents.Child[0].Title)
		assert.Len(t, doc.TableOfContents.Child[0].Child, 0)

		assert.Equal(t, "second", doc.TableOfContents.Child[1].Title)
		assert.Len(t, doc.TableOfContents.Child[1].Child, 0)

		assert.Equal(t, "third", doc.TableOfContents.Child[2].Title)
		if assert.Len(t, doc.TableOfContents.Child[2].Child, 2) {
			assert.Equal(t, "fourth", doc.TableOfContents.Child[2].Child[0].Title)
			assert.Equal(t, "fifth", doc.TableOfContents.Child[2].Child[1].Title)
		}
		assert.Equal(t, "sixth", doc.TableOfContents.Child[3].Title)
		if assert.Len(t, doc.TableOfContents.Child[3].Child, 1) {
			assert.Equal(t, "seventh", doc.TableOfContents.Child[3].Child[0].Title)
		}
	}
}
