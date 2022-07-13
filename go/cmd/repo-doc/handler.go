package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"go.uber.org/zap"
	"google.golang.org/grpc/status"

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

//go:embed directory.tmpl
var directoryIndexTemplate string

var (
	documentPage       = template.Must(template.New("doc").Parse(docTemplate))
	directoryIndexPage = template.Must(template.New("index").Parse(directoryIndexTemplate))
)

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

	requestFilePath := blobPath
	if requestFilePath == "" {
		requestFilePath = "/"
	}
	logger.Log.Debug("GetFile", zap.String("repo", repo), zap.String("ref", ref), zap.String("path", requestFilePath))
	file, err := h.client.GetFile(req.Context(), &git.RequestGetFile{Repo: repo, Ref: ref, Path: requestFilePath})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Message() == "path is directory" {
			h.serveDirectoryIndex(req.Context(), w, repo, ref, rawRef, requestFilePath)
			return
		}

		logger.Log.Error("Failed to get file", logger.Error(err))
		http.Error(w, "Failed to get file", http.StatusInternalServerError)
		return
	}

	h.serveDocumentFile(w, file, repo, rawRef, blobPath)
}

func (h *httpHandler) serveDocumentFile(w http.ResponseWriter, file *git.ResponseGetFile, repo, rawRef, blobPath string) {
	var doc *document
	switch filepath.Ext(blobPath) {
	case ".md":
		d, err := h.markdown.Parse(file.Content)
		if err != nil {
			logger.Log.Warn("Failed to convert to markdown", logger.Error(err))
			http.Error(w, "Failed to convert to markdown", http.StatusInternalServerError)
			return
		}
		doc = d
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

	breadcrumb := makeBreadcrumb(repo, rawRef, blobPath)
	err := documentPage.Execute(w, struct {
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
		TableOfContent:      toTemplateToC(h.toCMaxDepth, doc.TableOfContents),
	})
	if err != nil {
		logger.Log.Error("Failed to render page", logger.Error(err))
		http.Error(w, "Failed render page", http.StatusInternalServerError)
		return
	}
}

type directoryEntry struct {
	Name  string
	Path  string
	IsDir bool
}

func (h *httpHandler) serveDirectoryIndex(ctx context.Context, w http.ResponseWriter, repo, ref, rawRef, dirPath string) {
	logger.Log.Debug("GetTree", zap.String("repo", repo), zap.String("ref", ref), zap.String("path", dirPath))
	tree, err := h.client.GetTree(ctx, &git.RequestGetTree{
		Repo:      repo,
		Ref:       ref,
		Recursive: false,
		Path:      dirPath,
	})
	if err != nil {
		logger.Log.Error("Failed to get tree", logger.Error(err))
		http.Error(w, "Failed to get tree", http.StatusInternalServerError)
		return
	}

	sort.Slice(tree.Tree, func(i, j int) bool {
		if tree.Tree[i].Mode != tree.Tree[j].Mode {
			return tree.Tree[i].Mode < tree.Tree[j].Mode
		}
		return tree.Tree[i].Path < tree.Tree[j].Path
	})
	entry := make([]*directoryEntry, len(tree.Tree))
	for i, v := range tree.Tree {
		rootDir := dirPath
		if rootDir == "/" {
			rootDir = ""
		}
		entry[i] = &directoryEntry{
			Name:  v.Path,
			Path:  path.Join(rootDir, v.Path),
			IsDir: v.Mode == filemode.Dir.String(),
		}
	}
	breadcrumb := makeBreadcrumb(repo, rawRef, dirPath)
	err = directoryIndexPage.Execute(w, struct {
		Title               string
		PageTitle           string
		Breadcrumb          []*breadcrumbNode
		BreadcrumbLastIndex int
		Repo                string
		Ref                 string
		Path                string
		Entry               []*directoryEntry
	}{
		Title:               h.title,
		PageTitle:           dirPath,
		Breadcrumb:          breadcrumb,
		BreadcrumbLastIndex: len(breadcrumb) - 1,
		Repo:                repo,
		Ref:                 rawRef,
		Path:                dirPath,
		Entry:               entry,
	})
	if err != nil {
		logger.Log.Error("Failed to render page", logger.Error(err))
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

func makeBreadcrumb(repo, ref, blobPath string) []*breadcrumbNode {
	breadcrumb := []*breadcrumbNode{{Name: repo, Link: fmt.Sprintf("/%s/%s/-/", repo, ref)}}
	if blobPath == "/" {
		return breadcrumb
	}
	s := strings.Split(blobPath, "/")
	for i, v := range s {
		breadcrumb = append(breadcrumb, &breadcrumbNode{
			Name: v,
			Link: fmt.Sprintf("/%s/%s/-/%s", repo, ref, strings.Join(s[:i+1], "/")),
		})
	}
	return breadcrumb
}

func toTemplateToC(maxDepth int, in *tableOfContent) []*templateToC {
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
			child = append(child, toTemplateToC(maxDepth, v)...)
		}
		child[len(child)-1].Up = true

		res = append(res, child...)
	}

	return res
}
