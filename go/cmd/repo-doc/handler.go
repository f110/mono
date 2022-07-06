package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"go.uber.org/zap"

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

	var commit string
	if gitHash.MatchString(ref) {
		commit = ref
	} else {
		ref = plumbing.NewBranchReferenceName(ref).String()
		ref, err := h.client.GetReference(req.Context(), &git.RequestGetReference{Repo: repo, Ref: ref})
		if err != nil {
			logger.Log.Error("Failed to get reference", logger.Error(err))
			http.Error(w, fmt.Sprintf("Reference %s is not found", ref), http.StatusBadRequest)
			return
		}
		commit = ref.Ref.Hash
	}

	tree, err := h.client.GetTree(req.Context(), &git.RequestGetTree{Repo: repo, Sha: commit})
	if err != nil {
		logger.Log.Error("Failed to get tree", logger.Error(err))
		http.Error(w, "Failed to get the tree", http.StatusInternalServerError)
		return
	}
	var blobHash string
	for _, entry := range tree.Tree {
		if entry.Path == blobPath {
			blobHash = entry.Sha
			break
		}
	}
	if blobHash == "" {
		logger.Log.Info("Blob is not found in the tree", zap.String("path", blobPath))
		http.NotFound(w, req)
		return
	}
	blob, err := h.client.GetBlob(req.Context(), &git.RequestGetBlob{Repo: repo, Sha: blobHash})
	if err != nil {
		logger.Log.Error("Failed to get blob", logger.Error(err))
		http.Error(w, "Failed to get blob", http.StatusInternalServerError)
		return
	}

	buf := new(bytes.Buffer)
	switch filepath.Ext(blobPath) {
	case ".md":
		if err := md.Convert(blob.Content, buf); err != nil {
			logger.Log.Warn("Failed to convert to markdown", logger.Error(err))
			http.Error(w, "Failed to convert to markdown", http.StatusInternalServerError)
			return
		}
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
