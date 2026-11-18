// Package static provides the frontend of Codexel embedded into the binary.
package static

import (
	"embed"
	"io/fs"
)

//go:embed dist_prod
var dist embed.FS

// FS returns the built frontend. index.html is at the root.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist_prod")
	if err != nil {
		panic(err)
	}
	return sub
}
