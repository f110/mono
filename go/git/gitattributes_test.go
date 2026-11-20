package git

import (
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestAttributesGenerated(t *testing.T) {
	f := ParseAttributes([]byte(`# comment
*.pb.go linguist-generated
/vendor/** linguist-generated=true
vendor/keep.go -linguist-generated
docs/**/*.gen.md linguist-generated
gen/ linguist-generated
src/[ab].js linguist-generated
*.txt text
lock.json linguist-generated=false
**/lock.json linguist-generated
`))

	cases := []struct {
		path      string
		generated bool
	}{
		{path: "api.pb.go", generated: true},
		{path: "go/api/api.pb.go", generated: true},
		{path: "go/api/api.go", generated: false},
		{path: "vendor/a/b.go", generated: true},
		{path: "go/vendor/a.go", generated: false},
		{path: "vendor/keep.go", generated: false},
		{path: "docs/a.gen.md", generated: true},
		{path: "docs/a/b/c.gen.md", generated: true},
		{path: "gen/a.go", generated: false},
		{path: "src/a.js", generated: true},
		{path: "src/c.js", generated: false},
		{path: "a.txt", generated: false},
		{path: "lock.json", generated: true},
		{path: "a/lock.json", generated: true},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			assertion.Equal(t, f.Generated(tc.path), tc.generated)
		})
	}
}

func TestAttributesGeneratedEmpty(t *testing.T) {
	assertion.False(t, ParseAttributes(nil).Generated("a.go"))
}
