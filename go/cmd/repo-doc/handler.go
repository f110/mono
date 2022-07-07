package main

import (
	"bytes"
	"html/template"
	"mime"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"

	"go.f110.dev/mono/go/pkg/git"
	"go.f110.dev/mono/go/pkg/logger"
)

const (
	pathSeparator = "/-/"
)

var (
	gitHash = regexp.MustCompile(`[[:alnum:]]{40}`)
)

type httpHandler struct {
	client git.GitDataClient
}

var _ http.Handler = &httpHandler{}

func newHttpHandler(client git.GitDataClient) *httpHandler {
	return &httpHandler{client: client}
}

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	goldmark.WithRendererOptions(),
)

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if strings.Index(req.URL.Path, pathSeparator) == -1 {
		http.NotFound(w, req)
		return
	}
	repo, ref, blobPath := h.parsePath(req)

	if !gitHash.MatchString(ref) {
		ref = plumbing.NewBranchReferenceName(ref).String()
	}

	file, err := h.client.GetFile(req.Context(), &git.RequestGetFile{Repo: repo, Ref: ref, Path: blobPath})
	if err != nil {
		logger.Log.Error("Failed to get file", logger.Error(err))
		http.Error(w, "Failed to get file", http.StatusInternalServerError)
		return
	}

	buf := new(bytes.Buffer)
	switch filepath.Ext(blobPath) {
	case ".md":
		if err := md.Convert(file.Content, buf); err != nil {
			logger.Log.Warn("Failed to convert to markdown", logger.Error(err))
			http.Error(w, "Failed to convert to markdown", http.StatusInternalServerError)
			return
		}

		err = documentPage.Execute(w, struct {
			Content template.HTML
		}{
			Content: template.HTML(buf.String()),
		})
		if err != nil {
			logger.Log.Warn("Failed to render page", logger.Error(err))
			http.Error(w, "Failed render page", http.StatusInternalServerError)
			return
		}
	default:
		if v := mime.TypeByExtension(filepath.Ext(blobPath)); v != "" {
			w.Header().Set("Content-Type", v)
		}
		if _, err := w.Write(file.Content); err != nil {
			logger.Log.Warn("Failed write content", logger.Error(err))
			http.Error(w, "Failed write content", http.StatusInternalServerError)
			return
		}
	}
}

func (h *httpHandler) parsePath(req *http.Request) (repo string, ref string, filepath string) {
	sep := strings.LastIndex(req.URL.Path, pathSeparator)
	repoAndRef := req.URL.Path[1:sep]
	filepath = req.URL.Path[sep+len(pathSeparator):]
	sep = strings.Index(repoAndRef, "/")
	repo, ref = repoAndRef[:sep], repoAndRef[sep+1:]

	return repo, ref, filepath
}

var documentPage = template.Must(template.New("doc").Parse(`
<html>
<head></head>
<body>
{{ .Content }}
</body>
</html>`))
