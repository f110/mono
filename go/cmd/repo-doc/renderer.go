package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"

	"go.f110.dev/mono/go/docutil"
	"go.f110.dev/mono/go/pkg/git"
	"go.f110.dev/mono/go/pkg/logger"
)

//go:embed doc.tmpl directory.tmpl index.tmpl
var templateFiles embed.FS

var (
	pageTemplate = template.Must(template.New("").ParseFS(templateFiles, "*.tmpl"))
)

type Renderer struct {
	Title       string
	ToCMaxDepth int
	docSearch   docutil.DocSearchClient

	enabledSearch                   bool
	metadataAvailableFileExtensions map[string]struct{}
}

func NewRenderer(docSearch docutil.DocSearchClient, title string, toCMaxDepth int, availableFeatures *docutil.ResponseAvailableFeatures) *Renderer {
	extensions := make(map[string]struct{})
	for _, v := range availableFeatures.SupportedFileType {
		switch v {
		case docutil.FileType_FILE_TYPE_MARKDOWN:
			extensions["md"] = struct{}{}
		}
	}

	return &Renderer{
		Title:                           title,
		ToCMaxDepth:                     toCMaxDepth,
		docSearch:                       docSearch,
		enabledSearch:                   availableFeatures.FullTextSearch,
		metadataAvailableFileExtensions: extensions,
	}
}

func (r *Renderer) RenderRepositories(w http.ResponseWriter, repos []*git.Repository) {
	err := pageTemplate.ExecuteTemplate(w, "index.tmpl", struct {
		Title         string
		EnabledSearch bool
		Repositories  []*git.Repository
	}{
		Title:         r.Title,
		EnabledSearch: r.enabledSearch,
		Repositories:  repos,
	})
	if err != nil {
		logger.Log.Error("Failed to render page", logger.Error(err))
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
		return
	}
}

func (r *Renderer) RenderFile(ctx context.Context, w http.ResponseWriter, repo *docutil.Repository, file *git.ResponseGetFile, doc *document, commit *git.Commit) {
	var references, cited []*docutil.PageLink
	if _, ok := r.metadataAvailableFileExtensions[filepath.Ext(doc.Path)]; ok {
		pageLink, err := r.docSearch.PageLink(ctx, &docutil.RequestPageLink{Repo: repo.Name, Sha: doc.Path})
		if err != nil {
			logger.Log.Error("Failed to get page link", logger.Error(err))
			http.Error(w, "Failed to get page link", http.StatusInternalServerError)
			return
		}
		references = pageLink.Out
		cited = pageLink.In
	}

	breadcrumb := makeBreadcrumb(repo.Name, doc.Path, false)
	err := pageTemplate.ExecuteTemplate(w, "doc.tmpl", &pageTemplateVar{
		Title:               r.Title,
		PageTitle:           doc.Title,
		EnabledSearch:       r.enabledSearch,
		Repo:                repo.Name,
		Commit:              commit,
		Content:             template.HTML(doc.Content),
		Breadcrumb:          breadcrumb,
		BreadcrumbLastIndex: len(breadcrumb) - 1,
		TableOfContent:      toTemplateToC(r.ToCMaxDepth, doc.TableOfContents),
		RawURL:              file.RawUrl,
		EditURL:             file.EditUrl,
		References:          references,
		Cited:               cited,
	})
	if err != nil {
		logger.Log.Error("Failed to render page", logger.Error(err))
		http.Error(w, "Failed render page", http.StatusInternalServerError)
		return
	}
}

func (r *Renderer) RenderPage(w http.ResponseWriter, repo *docutil.Repository, page *docutil.ResponseGetPage, doc *document, commit *git.Commit) {
	breadcrumb := makeBreadcrumb(repo.Name, doc.Path, false)
	err := pageTemplate.ExecuteTemplate(w, "doc.tmpl", &pageTemplateVar{
		Title:               r.Title,
		PageTitle:           doc.Title,
		EnabledSearch:       r.enabledSearch,
		Repo:                repo.Name,
		Commit:              commit,
		Content:             template.HTML(doc.Content),
		Breadcrumb:          breadcrumb,
		BreadcrumbLastIndex: len(breadcrumb) - 1,
		TableOfContent:      toTemplateToC(r.ToCMaxDepth, doc.TableOfContents),
		RawURL:              page.RawUrl,
		EditURL:             page.EditUrl,
		References:          page.Out,
		Cited:               page.In,
	})
	if err != nil {
		logger.Log.Error("Failed to render page", logger.Error(err))
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
		return
	}
}

func (r *Renderer) RenderDirectoryIndex(w http.ResponseWriter, repo *docutil.Repository, dirPath string, commit *git.Commit, entry []*directoryEntry, content string) {
	breadcrumb := makeBreadcrumb(repo.Name, dirPath, true)
	err := pageTemplate.ExecuteTemplate(w, "directory.tmpl", struct {
		Title               string
		PageTitle           string
		EnabledSearch       bool
		Breadcrumb          []*breadcrumbNode
		BreadcrumbLastIndex int
		Commit              *git.Commit
		Content             template.HTML
		Repo                string
		Path                string
		Entry               []*directoryEntry
	}{
		Title:               r.Title,
		PageTitle:           dirPath,
		EnabledSearch:       r.enabledSearch,
		Breadcrumb:          breadcrumb,
		BreadcrumbLastIndex: len(breadcrumb) - 1,
		Commit:              commit,
		Content:             template.HTML(content),
		Repo:                repo.Name,
		Path:                dirPath,
		Entry:               entry,
	})
	if err != nil {
		logger.Log.Error("Failed to render page", logger.Error(err))
		http.Error(w, "Failed render page", http.StatusInternalServerError)
		return
	}
}

type pageTemplateVar struct {
	Title               string
	PageTitle           string
	EnabledSearch       bool
	Repo                string
	Commit              *git.Commit
	Content             template.HTML
	Breadcrumb          []*breadcrumbNode
	BreadcrumbLastIndex int
	TableOfContent      []*templateToC
	RawURL              string
	EditURL             string
	References          []*docutil.PageLink
	Cited               []*docutil.PageLink
}

type breadcrumbNode struct {
	Name string
	Link string
}

func makeBreadcrumb(repo, blobPath string, isDir bool) []*breadcrumbNode {
	breadcrumb := []*breadcrumbNode{{Name: repo, Link: fmt.Sprintf("/%s%s", repo, pathSeparator)}}
	if blobPath == "/" {
		return breadcrumb
	}
	s := strings.Split(blobPath, "/")
	for i, v := range s[:len(s)-1] {
		breadcrumb = append(breadcrumb, &breadcrumbNode{
			Name: v,
			Link: fmt.Sprintf("/%s%s%s/", repo, pathSeparator, strings.Join(s[:i+1], "/")),
		})
	}
	if isDir {
		breadcrumb = append(breadcrumb, &breadcrumbNode{
			Name: s[len(s)-1],
			Link: fmt.Sprintf("/%s%s%s/", repo, pathSeparator, strings.Join(s, "/")),
		})
	} else {
		breadcrumb = append(breadcrumb, &breadcrumbNode{
			Name: s[len(s)-1],
			Link: fmt.Sprintf("/%s%s%s", repo, pathSeparator, strings.Join(s, "/")),
		})
	}
	return breadcrumb
}

type templateToC struct {
	Title  string
	Anchor string
	Down   bool
	Up     bool
}

func toTemplateToC(maxDepth int, in *tableOfContent) []*templateToC {
	var res []*templateToC

	if in.Title != "" {
		res = append(res, &templateToC{Title: in.Title, Anchor: in.Anchor})
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
