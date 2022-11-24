package httpserver

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
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

type SinglePageApplicationHandler struct {
	root http.FileSystem
}

var _ http.Handler = &SinglePageApplicationHandler{}

func SinglePageApplication(root string) *SinglePageApplicationHandler {
	return &SinglePageApplicationHandler{root: SinglePageApplicationFileSystem(root)}
}

func (h *SinglePageApplicationHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	urlPath := req.URL.Path
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
		req.URL.Path = urlPath
	}
	p := path.Clean(req.URL.Path)

	f, err := h.root.Open(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "404 page not found", http.StatusNotFound)
		}
		if errors.Is(err, fs.ErrPermission) {
			http.Error(w, "403 Forbidden", http.StatusForbidden)
		}
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
		return
	}

	http.ServeContent(w, req, stat.Name(), stat.ModTime(), f)
}
