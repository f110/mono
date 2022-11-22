package httpserver

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
)

const (
	indexFileName = "index.html"
)

type SinglePageApplicationFileSystem string

var _ http.FileSystem = SinglePageApplicationFileSystem("")

func (s SinglePageApplicationFileSystem) Open(name string) (http.File, error) {
	dir := string(s)
	if dir == "" {
		dir = "."
	}
	full := filepath.Join(dir, filepath.FromSlash(path.Clean("/"+name)))
	st, err := os.Stat(full)
	if os.IsNotExist(err) {
		return os.Open(filepath.Join(dir, indexFileName))
	}
	if err != nil {
		return nil, err
	}

	if st.IsDir() {
		return os.Open(filepath.Join(dir, indexFileName))
	}
	return os.Open(full)
}
