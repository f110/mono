package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"google.golang.org/grpc/status"

	"go.f110.dev/mono/go/pkg/docutil"
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

//go:embed doc.tmpl directory.tmpl index.tmpl
var templateFiles embed.FS

var (
	pageTemplate = template.Must(template.New("").ParseFS(templateFiles, "*.tmpl"))
)

type httpHandler struct {
	gitData   git.GitDataClient
	docSearch docutil.DocSearchClient
	static    http.Handler
	markdown  *markdownParser

	toCMaxDepth   int
	title         string
	enabledSearch bool
}

var _ http.Handler = &httpHandler{}

func newHttpHandler(
	gitData git.GitDataClient,
	docSearch docutil.DocSearchClient,
	title, staticDir string,
	toCMaxDepth int,
	enabledSearch bool,
) *httpHandler {
	var static http.Handler
	if staticDir != "" {
		static = http.FileServer(http.Dir(staticDir))
	} else {
		static = http.FileServer(http.FS(staticContent))
	}
	return &httpHandler{
		gitData:       gitData,
		docSearch:     docSearch,
		static:        static,
		title:         title,
		toCMaxDepth:   toCMaxDepth,
		enabledSearch: enabledSearch,
		markdown:      newMarkdownParser(),
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
		if req.URL.Path == "/" {
			h.serveRepositoryIndex(w, req)
			return
		}
		h.static.ServeHTTP(w, req)
		return
	}

	repo, rawRef, blobPath := h.parsePath(req)
	ref := rawRef
	if !gitHash.MatchString(rawRef) {
		ref = plumbing.NewBranchReferenceName(rawRef).String()
	}

	requestFilePath := blobPath
	if len(requestFilePath) > 0 && requestFilePath[len(requestFilePath)-1] == '/' {
		requestFilePath = requestFilePath[:len(requestFilePath)-1]
	}
	if requestFilePath == "" {
		requestFilePath = "/"
	}
	logger.Log.Debug("GetFile", zap.String("repo", repo), zap.String("ref", ref), zap.String("path", requestFilePath))
	file, err := h.gitData.GetFile(req.Context(), &git.RequestGetFile{Repo: repo, Ref: ref, Path: requestFilePath})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Message() == "path is directory" {
			h.serveDirectoryIndex(req.Context(), w, req, repo, ref, rawRef, requestFilePath)
			return
		}

		logger.Log.Error("Failed to get file", logger.Error(err))
		http.Error(w, "Failed to get file", http.StatusInternalServerError)
		return
	}

	h.serveDocumentFile(req.Context(), w, file, repo, rawRef, blobPath)
}

func (h *httpHandler) serveRepositoryIndex(w http.ResponseWriter, req *http.Request) {
	repositories, err := h.gitData.ListRepositories(req.Context(), &git.RequestListRepositories{})
	if err != nil {
		logger.Log.Error("Failed to get repositories", logger.Error(err))
		http.Error(w, "Failed to get repositories", http.StatusInternalServerError)
		return
	}

	err = pageTemplate.ExecuteTemplate(w, "index.tmpl", struct {
		Title         string
		EnabledSearch bool
		Repositories  []*git.Repository
	}{
		Title:         h.title,
		EnabledSearch: h.enabledSearch,
		Repositories:  repositories.Repositories,
	})
	if err != nil {
		logger.Log.Error("Failed to render page", logger.Error(err))
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
		return
	}
}

func (h *httpHandler) serveDocumentFile(ctx context.Context, w http.ResponseWriter, file *git.ResponseGetFile, repo, rawRef, blobPath string) {
	logger.Log.Debug("PageLink", zap.String("repo", repo), zap.String("sha", blobPath))
	pageLink, err := h.docSearch.PageLink(ctx, &docutil.RequestPageLink{Repo: repo, Sha: blobPath})
	log.Print(pageLink)
	log.Print(err)

	var doc *document
	switch filepath.Ext(blobPath) {
	case ".md":
		d, err := h.makeDocument(file, blobPath)
		if err != nil {
			logger.Log.Error("Failed to convert document", logger.Error(err))
			http.Error(w, "Failed to convert document", http.StatusInternalServerError)
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

	breadcrumb := makeBreadcrumb(repo, rawRef, blobPath, false)
	err = pageTemplate.ExecuteTemplate(w, "doc.tmpl", struct {
		Title               string
		PageTitle           string
		EnabledSearch       bool
		Content             template.HTML
		Breadcrumb          []*breadcrumbNode
		BreadcrumbLastIndex int
		TableOfContent      []*templateToC
		RawURL              string
		EditURL             string
	}{
		Title:               h.title,
		PageTitle:           doc.Title,
		EnabledSearch:       h.enabledSearch,
		Content:             template.HTML(doc.Content),
		Breadcrumb:          breadcrumb,
		BreadcrumbLastIndex: len(breadcrumb) - 1,
		TableOfContent:      toTemplateToC(h.toCMaxDepth, doc.TableOfContents),
		RawURL:              file.RawUrl,
		EditURL:             file.EditUrl,
	})
	if err != nil {
		logger.Log.Error("Failed to render page", logger.Error(err))
		http.Error(w, "Failed render page", http.StatusInternalServerError)
		return
	}
}

func (h *httpHandler) makeDocument(file *git.ResponseGetFile, blobPath string) (*document, error) {
	switch filepath.Ext(blobPath) {
	case ".md":
		d, err := h.markdown.Parse(file.Content)
		if err != nil {
			return nil, err
		}
		return d, nil
	}

	return nil, xerrors.New("not implemented")
}

type directoryEntry struct {
	Name  string
	Path  string
	IsDir bool
}

func (h *httpHandler) serveDirectoryIndex(ctx context.Context, w http.ResponseWriter, req *http.Request, repo, ref, rawRef, dirPath string) {
	logger.Log.Debug("GetTree", zap.String("repo", repo), zap.String("ref", ref), zap.String("path", dirPath))
	tree, err := h.gitData.GetTree(ctx, &git.RequestGetTree{
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
	foundIndexFile := ""
	for i, v := range tree.Tree {
		rootDir := dirPath
		if rootDir == "/" {
			rootDir = ""
		}
		switch v.Path {
		case "README.md":
			foundIndexFile = v.Path
		}
		p := path.Join(rootDir, v.Path)
		if v.Mode == filemode.Dir.String() {
			p += "/"
		}
		entry[i] = &directoryEntry{
			Name:  v.Path,
			Path:  p,
			IsDir: v.Mode == filemode.Dir.String(),
		}
	}

	content := ""
	if foundIndexFile != "" {
		indexFilePath := path.Join(dirPath, foundIndexFile)
		if dirPath == "/" {
			indexFilePath = foundIndexFile
		}
		indexFile, err := h.gitData.GetFile(ctx, &git.RequestGetFile{Repo: repo, Ref: ref, Path: indexFilePath})
		if err != nil {
			logger.Log.Error("Failed to get index file", zap.Error(err), zap.String("path", indexFilePath))
			http.Error(w, "Failed to get index file", http.StatusInternalServerError)
			return
		}
		d, err := h.makeDocument(indexFile, path.Join(dirPath, foundIndexFile))
		if err != nil {
			logger.Log.Error("Failed to convert to document", logger.Error(err))
			http.Error(w, "Failed to convert to markdown", http.StatusInternalServerError)
			return
		}
		content = d.Content
	}

	breadcrumb := makeBreadcrumb(repo, rawRef, dirPath, true)
	err = pageTemplate.ExecuteTemplate(w, "directory.tmpl", struct {
		Title               string
		PageTitle           string
		EnabledSearch       bool
		Breadcrumb          []*breadcrumbNode
		BreadcrumbLastIndex int
		Content             template.HTML
		Repo                string
		Ref                 string
		Path                string
		Entry               []*directoryEntry
	}{
		Title:               h.title,
		PageTitle:           dirPath,
		EnabledSearch:       h.enabledSearch,
		Breadcrumb:          breadcrumb,
		BreadcrumbLastIndex: len(breadcrumb) - 1,
		Content:             template.HTML(content),
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

func makeBreadcrumb(repo, ref, blobPath string, isDir bool) []*breadcrumbNode {
	breadcrumb := []*breadcrumbNode{{Name: repo, Link: fmt.Sprintf("/%s/%s/-/", repo, ref)}}
	if blobPath == "/" {
		return breadcrumb
	}
	s := strings.Split(blobPath, "/")
	for i, v := range s[:len(s)-1] {
		breadcrumb = append(breadcrumb, &breadcrumbNode{
			Name: v,
			Link: fmt.Sprintf("/%s/%s/-/%s/", repo, ref, strings.Join(s[:i+1], "/")),
		})
	}
	if isDir {
		breadcrumb = append(breadcrumb, &breadcrumbNode{
			Name: s[len(s)-1],
			Link: fmt.Sprintf("/%s/%s/-/%s/", repo, ref, strings.Join(s, "/")),
		})
	} else {
		breadcrumb = append(breadcrumb, &breadcrumbNode{
			Name: s[len(s)-1],
			Link: fmt.Sprintf("/%s/%s/-/%s", repo, ref, strings.Join(s, "/")),
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
