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
vendor/auto.go !linguist-generated
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
		specified bool
	}{
		{path: "api.pb.go", generated: true, specified: true},
		{path: "go/api/api.pb.go", generated: true, specified: true},
		{path: "go/api/api.go"},
		{path: "vendor/a/b.go", generated: true, specified: true},
		{path: "go/vendor/a.go"},
		{path: "vendor/keep.go", specified: true},
		{path: "vendor/auto.go"},
		{path: "docs/a.gen.md", generated: true, specified: true},
		{path: "docs/a/b/c.gen.md", generated: true, specified: true},
		{path: "gen/a.go"},
		{path: "src/a.js", generated: true, specified: true},
		{path: "src/c.js"},
		{path: "a.txt"},
		{path: "lock.json", generated: true, specified: true},
		{path: "a/lock.json", generated: true, specified: true},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			generated, specified := f.Generated(tc.path)
			assertion.Equal(t, generated, tc.generated)
			assertion.Equal(t, specified, tc.specified)
		})
	}
}

func TestAttributesGeneratedEmpty(t *testing.T) {
	generated, specified := ParseAttributes(nil).Generated("a.go")
	assertion.False(t, generated)
	assertion.False(t, specified)
}
