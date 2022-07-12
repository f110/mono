package main

import (
	"embed"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"

	"go.f110.dev/mono/go/pkg/git"
	"go.f110.dev/mono/go/pkg/logger"
)

const (
	pathSeparator = "/-/"
)

var (
	gitHash = regexp.MustCompile(`[[:alnum:]]{40}`)
)

//go:embed style.css
var staticContent embed.FS

//go:embed doc.tmpl
var docTemplate string

var documentPage = template.Must(template.New("doc").Parse(docTemplate))

type httpHandler struct {
	client   git.GitDataClient
	static   http.Handler
	markdown *markdownParser

	toCMaxDepth int
	title       string
}

var _ http.Handler = &httpHandler{}

func newHttpHandler(client git.GitDataClient, title, staticDir string, toCMaxDepth int) *httpHandler {
	var static http.Handler
	if staticDir != "" {
		static = http.FileServer(http.Dir(staticDir))
	} else {
		static = http.FileServer(http.FS(staticContent))
	}
	return &httpHandler{
		client:      client,
		static:      static,
		title:       title,
		toCMaxDepth: toCMaxDepth,
		markdown:    newMarkdownParser(),
	}
}

type templateToC struct {
	Title  string
	Anchor string
	Down   bool
	Up     bool
}

type breadcrumbNode struct {
	Name string
	Link string
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if strings.Index(req.URL.Path, pathSeparator) == -1 {
		h.static.ServeHTTP(w, req)
		return
	}

	repo, rawRef, blobPath := h.parsePath(req)
	ref := rawRef
	if !gitHash.MatchString(rawRef) {
		ref = plumbing.NewBranchReferenceName(rawRef).String()
	}

	file, err := h.client.GetFile(req.Context(), &git.RequestGetFile{Repo: repo, Ref: ref, Path: blobPath})
	if err != nil {
		logger.Log.Error("Failed to get file", logger.Error(err))
		http.Error(w, "Failed to get file", http.StatusInternalServerError)
		return
	}

	var doc *document
	switch filepath.Ext(blobPath) {
	case ".md":
		doc, err = h.markdown.Parse(file.Content)
		if err != nil {
			logger.Log.Warn("Failed to convert to markdown", logger.Error(err))
			http.Error(w, "Failed to convert to markdown", http.StatusInternalServerError)
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
		return
	}

	breadcrumb := []*breadcrumbNode{{Name: repo, Link: fmt.Sprintf("/%s/%s/-/", repo, rawRef)}}
	s := strings.Split(blobPath, "/")
	for i, v := range s {
		breadcrumb = append(breadcrumb, &breadcrumbNode{
			Name: v,
			Link: fmt.Sprintf("/%s/%s/-/%s", repo, rawRef, strings.Join(s[:i+1], "/")),
		})
	}
	err = documentPage.Execute(w, struct {
		Title               string
		PageTitle           string
		Content             template.HTML
		Breadcrumb          []*breadcrumbNode
		BreadcrumbLastIndex int
		TableOfContent      []*templateToC
	}{
		Title:               h.title,
		PageTitle:           doc.Title,
		Content:             template.HTML(doc.Content),
		Breadcrumb:          breadcrumb,
		BreadcrumbLastIndex: len(breadcrumb) - 1,
		TableOfContent:      toTempalteToC(h.toCMaxDepth, doc.TableOfContents),
	})
	if err != nil {
		logger.Log.Warn("Failed to render page", logger.Error(err))
		http.Error(w, "Failed render page", http.StatusInternalServerError)
		return
	}
}

func (h *httpHandler) parsePath(req *http.Request) (repo string, ref string, filepath string) {
	sep := strings.LastIndex(req.URL.Path, pathSeparator)
	if sep == -1 {
		return
	}
	repoAndRef := req.URL.Path[1:sep]
	filepath = req.URL.Path[sep+len(pathSeparator):]
	sep = strings.Index(repoAndRef, "/")
	repo, ref = repoAndRef[:sep], repoAndRef[sep+1:]

	return repo, ref, filepath
}

func toTempalteToC(maxDepth int, in *tableOfContent) []*templateToC {
	var res []*templateToC

	if in.Title != "" {
		anchor := strings.ToLower(in.Title)
		anchor = strings.Replace(anchor, " ", "-", -1)
		res = append(res, &templateToC{Title: in.Title, Anchor: anchor})
	}

	if len(in.Child) > 0 && in.Level+1 <= maxDepth {
		if len(res) > 0 {
			res[len(res)-1].Down = true
		}

		var child []*templateToC
		for _, v := range in.Child {
			child = append(child, toTempalteToC(maxDepth, v)...)
		}
		child[len(child)-1].Up = true

		res = append(res, child...)
	}

	return res
}
