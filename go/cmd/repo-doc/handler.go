package main

import (
	"context"
	"embed"
	"mime"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/docutil"
	"go.f110.dev/mono/go/pkg/git"
	"go.f110.dev/mono/go/pkg/logger"
)

const (
	pathSeparator = "/_/"
)

//go:embed style.css
var staticContent embed.FS

type httpHandler struct {
	gitData   git.GitDataClient
	docSearch docutil.DocSearchClient
	static    http.Handler
	markdown  *markdownParser
	renderer  *Renderer

	metadataAvailableFileExtensions map[string]struct{}
}

var _ http.Handler = &httpHandler{}

func newHttpHandler(
	ctx context.Context,
	gitData git.GitDataClient,
	docSearch docutil.DocSearchClient,
	title, staticDir string,
	toCMaxDepth int,
) (*httpHandler, error) {
	var static http.Handler
	if staticDir != "" {
		static = http.FileServer(http.Dir(staticDir))
	} else {
		static = http.FileServer(http.FS(staticContent))
	}
	availableFeatures, err := docSearch.AvailableFeatures(ctx, &docutil.RequestAvailableFeatures{})
	if err != nil {
		return nil, err
	}
	renderer := NewRenderer(docSearch, title, toCMaxDepth, availableFeatures)
	extensions := make(map[string]struct{})
	for _, v := range availableFeatures.SupportedFileType {
		switch v {
		case docutil.FileType_FILE_TYPE_MARKDOWN:
			extensions[".md"] = struct{}{}
		}
	}

	return &httpHandler{
		gitData:                         gitData,
		docSearch:                       docSearch,
		static:                          static,
		markdown:                        newMarkdownParser(),
		renderer:                        renderer,
		metadataAvailableFileExtensions: extensions,
	}, nil
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/_/readiness" {
		h.readiness(w, req)
		return
	}

	if strings.Index(req.URL.Path, pathSeparator) == -1 {
		if req.URL.Path == "/" {
			h.serveRepositoryIndex(w, req)
			return
		}

		h.static.ServeHTTP(w, req)
		return
	}

	repoName, blobPath := h.parsePath(req)
	repoRes, err := h.docSearch.GetRepository(req.Context(), &docutil.RequestGetRepository{Repo: repoName})
	if err != nil {
		http.Error(w, "Failed to get repository", http.StatusBadRequest)
		return
	}
	repo := repoRes.Repository

	var commit *git.Commit
	if v, err := h.gitData.GetCommit(req.Context(), &git.RequestGetCommit{Repo: repoName, Ref: plumbing.NewBranchReferenceName(repo.DefaultBranch).String()}); err != nil {
		return
	} else {
		commit = v.Commit
	}

	requestFilePath := blobPath
	if len(requestFilePath) > 0 && requestFilePath[len(requestFilePath)-1] == '/' {
		requestFilePath = requestFilePath[:len(requestFilePath)-1]
	}
	if requestFilePath == "" {
		requestFilePath = "/"
	}

	pathStat, err := h.gitData.Stat(req.Context(), &git.RequestStat{Repo: repoName, Ref: plumbing.NewBranchReferenceName(repo.DefaultBranch).String(), Path: requestFilePath})
	if err != nil {
		logger.Log.Error("Failed to get stat", logger.Error(err))
		http.Error(w, "Failed to get stat", http.StatusInternalServerError)
		return
	}

	if filemode.FileMode(pathStat.Mode)&filemode.Dir == filemode.Dir {
		h.serveDirectoryIndex(req.Context(), w, req, repo, repoName, commit, requestFilePath)
		return
	}

	if _, ok := h.metadataAvailableFileExtensions[filepath.Ext(requestFilePath)]; !ok {
		file, err := h.gitData.GetFile(req.Context(), &git.RequestGetFile{Repo: repoName, Ref: repo.DefaultBranch, Path: requestFilePath})
		if err != nil {
			logger.Log.Error("Failed to get file", logger.Error(err))
			http.Error(w, "Failed to get file", http.StatusInternalServerError)
			return
		}
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

	h.serveDocumentPage(req.Context(), w, repo, requestFilePath, commit)
}

func (h *httpHandler) readiness(w http.ResponseWriter, req *http.Request) {}

func (h *httpHandler) serveRepositoryIndex(w http.ResponseWriter, req *http.Request) {
	repositories, err := h.gitData.ListRepositories(req.Context(), &git.RequestListRepositories{})
	if err != nil {
		logger.Log.Error("Failed to get repositories", logger.Error(err))
		http.Error(w, "Failed to get repositories", http.StatusInternalServerError)
		return
	}

	h.renderer.RenderRepositories(w, repositories.Repositories)
}

func (h *httpHandler) serveDocumentFile(ctx context.Context, w http.ResponseWriter, file *git.ResponseGetFile, repo *docutil.Repository, repoName, rawRef string, commit *git.Commit, blobPath string) {
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

	h.renderer.RenderFile(ctx, w, repo, file, doc, commit)
}

func (h *httpHandler) serveDocumentPage(ctx context.Context, w http.ResponseWriter, repo *docutil.Repository, requestFilePath string, commit *git.Commit) {
	var page *docutil.ResponseGetPage
	if _, ok := h.metadataAvailableFileExtensions[filepath.Ext(requestFilePath)]; ok {
		p, err := h.docSearch.GetPage(ctx, &docutil.RequestGetPage{Repo: repo.Name, Path: requestFilePath})
		if err != nil {
			logger.Log.Error("Failed to get page from service", logger.Error(err))
			http.Error(w, "Failed to get page", http.StatusInternalServerError)
			return
		}
		page = p
	}

	var doc *document
	switch filepath.Ext(requestFilePath) {
	case ".md":
		d, err := h.markdown.Parse([]byte(page.Doc))
		if err != nil {
			logger.Log.Error("Failed to convert document body", logger.Error(err))
			http.Error(w, "Failed to convert document body", http.StatusInternalServerError)
			return
		}
		d.Path = requestFilePath
		doc = d
	}

	h.renderer.RenderPage(w, repo, page, doc, commit)
}

func (h *httpHandler) makeDocument(file *git.ResponseGetFile, blobPath string) (*document, error) {
	switch filepath.Ext(blobPath) {
	case ".md":
		d, err := h.markdown.Parse(file.Content)
		if err != nil {
			return nil, err
		}
		d.Path = blobPath
		return d, nil
	}

	return nil, xerrors.New("not implemented")
}

type directoryEntry struct {
	Name  string
	Path  string
	IsDir bool
}

func (h *httpHandler) serveDirectoryIndex(ctx context.Context, w http.ResponseWriter, _ *http.Request, repo *docutil.Repository, repoName string, commit *git.Commit, dirPath string) {
	var content string
	var foundIndexFile string
	tree, err := h.docSearch.GetDirectory(ctx, &docutil.RequestGetDirectory{
		Repo: repoName,
		Ref:  repo.DefaultBranch,
		Path: dirPath,
	})
	if err != nil {
		logger.Log.Error("Failed to get tree", logger.Error(err))
		http.Error(w, "Failed to get tree", http.StatusInternalServerError)
		return
	}

	sort.Slice(tree.Entries, func(i, j int) bool {
		return tree.Entries[i].Path < tree.Entries[j].Path
	})

	entry := make([]*directoryEntry, 0, len(tree.Entries))
	rootDir := dirPath
	if rootDir == "/" {
		rootDir = ""
	}
	for _, v := range tree.Entries {
		if !v.IsDir {
			continue
		}
		entry = append(entry, &directoryEntry{
			Name:  v.Name,
			Path:  v.Path + "/",
			IsDir: true,
		})
	}
	for _, v := range tree.Entries {
		if v.IsDir {
			continue
		}
		switch v.Name {
		case "README.md":
			foundIndexFile = v.Path
		}
		entry = append(entry, &directoryEntry{
			Name:  v.Name,
			Path:  v.Path,
			IsDir: v.IsDir,
		})
	}

	if foundIndexFile != "" {
		page, err := h.docSearch.GetPage(ctx, &docutil.RequestGetPage{
			Repo: repoName,
			Path: foundIndexFile,
		})
		if err != nil {
			logger.Log.Error("Failed to get index content", logger.Error(err))
			http.Error(w, "Failed to get index content", http.StatusInternalServerError)
			return
		}
		content = page.Doc

		switch filepath.Ext(foundIndexFile) {
		case ".md":
			d, err := h.markdown.Parse([]byte(page.Doc))
			if err != nil {
				logger.Log.Error("Failed to convert document body", logger.Error(err))
				http.Error(w, "Failed to convert document body", http.StatusInternalServerError)
				return
			}
			content = d.Content
		}
	}

	h.renderer.RenderDirectoryIndex(w, repo, dirPath, commit, entry, content)
}

func (h *httpHandler) parsePath(req *http.Request) (repo string, filepath string) {
	sep := strings.LastIndex(req.URL.Path, pathSeparator)
	if sep == -1 {
		return
	}
	repo = req.URL.Path[1:sep]
	filepath = req.URL.Path[sep+len(pathSeparator):]

	return repo, filepath
}
