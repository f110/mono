package docutil

import (
	"context"
	"errors"
	"log"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.f110.dev/xerrors"
	"gopkg.in/src-d/go-git.v4/plumbing"

	"go.f110.dev/mono/go/pkg/git"
)

// repositoryPageLinks represents links that had the page.
// The key of map is a file path.
type repositoryPageLinks map[string]*page

type page struct {
	LinkIn  []*PageLink
	LinkOut []*PageLink
}

type DocSearchService struct {
	client         git.GitDataClient
	markdownParser parser.Parser

	repositories []*git.Repository
	// pageLink is a cache data. The key of map is a name of repository.
	pageLink map[string]repositoryPageLinks
}

func NewDocSearchService(client git.GitDataClient) *DocSearchService {
	g := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
		),
	)
	markdownParser := g.Parser()
	return &DocSearchService{client: client, markdownParser: markdownParser, pageLink: make(map[string]repositoryPageLinks)}
}

var _ DocSearchServer = &DocSearchService{}

func (d *DocSearchService) AvailableFeatures(_ context.Context, _ *RequestAvailableFeatures) (*ResponseAvailableFeatures, error) {
	return &ResponseAvailableFeatures{PageLink: true}, nil
}

func (d *DocSearchService) GetPage(ctx context.Context, req *RequestGetPage) (*ResponseGetPage, error) {
	//TODO implement me
	panic("implement me")
}

func (d *DocSearchService) PageLink(_ context.Context, req *RequestPageLink) (*ResponsePageLink, error) {
	links, ok := d.pageLink[req.Repo]
	if !ok {
		return nil, errors.New("repository not found")
	}
	page, ok := links[req.Sha]
	if !ok {
		return nil, errors.New("path is not found")
	}
	return &ResponsePageLink{In: page.LinkIn, Out: page.LinkOut}, nil
}

func (d *DocSearchService) Initialize(ctx context.Context) error {
	if err := d.scanRepositories(ctx); err != nil {
		return err
	}

	d.interpolateLinks()
	return nil
}

func (d *DocSearchService) interpolateLinks() {
	for sourceRepoName, pageLinks := range d.pageLink {
		for sourcePath, page := range pageLinks {
			for _, link := range page.LinkOut {
				switch link.Type {
				case LinkType_LINK_TYPE_IN_REPOSITORY:
					if _, ok := pageLinks[link.Destination]; !ok {
						log.Print(link.Destination)
					} else {
						pageLinks[link.Destination].LinkIn = append(pageLinks[link.Destination].LinkIn, &PageLink{
							Type:   LinkType_LINK_TYPE_IN_REPOSITORY,
							Source: sourcePath,
						})
					}
				case LinkType_LINK_TYPE_NEIGHBOR_REPOSITORY:
					if _, ok := d.pageLink[link.Repository][link.Destination]; !ok {
						log.Print(link.Destination)
					} else {
						destPage := d.pageLink[link.Repository][link.Destination]
						destPage.LinkIn = append(destPage.LinkIn, &PageLink{
							Type:       LinkType_LINK_TYPE_NEIGHBOR_REPOSITORY,
							Source:     sourcePath,
							Repository: sourceRepoName,
						})
					}
				}
			}
		}
	}
}

func (d *DocSearchService) scanRepositories(ctx context.Context) error {
	repos, err := d.client.ListRepositories(ctx, &git.RequestListRepositories{})
	if err != nil {
		return xerrors.WithMessage(err, "Failed to get list of repository")
	}
	d.repositories = repos.Repositories

	for _, v := range repos.Repositories {
		if err := d.scanRepository(ctx, v); err != nil {
			return xerrors.WithMessagef(err, "Failed to scan the repository: %s", v.Name)
		}
	}

	return nil
}

func (d *DocSearchService) scanRepository(ctx context.Context, repo *git.Repository) error {
	tree, err := d.client.GetTree(ctx, &git.RequestGetTree{
		Repo:      repo.Name,
		Ref:       plumbing.NewBranchReferenceName(repo.DefaultBranch).String(),
		Path:      "/",
		Recursive: true,
	})
	if err != nil {
		return xerrors.WithStack(err)
	}

	pageLinks := make(repositoryPageLinks)
	d.pageLink[repo.Name] = pageLinks
	for _, v := range tree.Tree {
		switch filepath.Ext(v.Path) {
		case ".md":
			blob, err := d.client.GetBlob(ctx, &git.RequestGetBlob{Repo: repo.Name, Sha: v.Sha})
			if err != nil {
				return xerrors.WithMessagef(err, "Failed to get blob: %s", repo.Name)
			}
			rootNode := d.markdownParser.Parse(text.NewReader(blob.Content))
			page, err := d.makePageFromMarkdownAST(rootNode, repo, blob.Content)
			if err != nil {
				return err
			}

			pageLinks[v.Path] = page
		}
	}

	return nil
}

func (d *DocSearchService) makePageFromMarkdownAST(rootNode ast.Node, repo *git.Repository, raw []byte) (*page, error) {
	page := &page{}

	var linkOut []*PageLink
	err := ast.Walk(rootNode, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch v := n.(type) {
		case *ast.Link:
			u, err := url.Parse(string(v.Destination))
			if err != nil {
				return ast.WalkContinue, nil
			}
			if u.Scheme == "" {
				destination := string(v.Destination)
				if destination[0] == '#' {
					return ast.WalkContinue, nil
				}
				linkOut = append(linkOut, &PageLink{
					Type:        LinkType_LINK_TYPE_IN_REPOSITORY,
					Destination: destination,
				})
			} else {
				destination := string(v.Destination)
				linkType := LinkType_LINK_TYPE_EXTERNAL
				repoName := ""

				if strings.HasPrefix(destination, repo.Url) {
					linkType = LinkType_LINK_TYPE_IN_REPOSITORY
					destination = strings.TrimPrefix(destination, repo.Url+"/")
				} else {
					for _, repo := range d.repositories {
						if strings.HasPrefix(destination, repo.Url) {
							linkType = LinkType_LINK_TYPE_NEIGHBOR_REPOSITORY
							repoName = repo.Name
							break
						}
					}
				}

				linkOut = append(linkOut, &PageLink{
					Type:        linkType,
					Destination: destination,
					Repository:  repoName,
				})
			}
		case *ast.AutoLink:
			destination := string(v.URL(raw))
			linkType := LinkType_LINK_TYPE_EXTERNAL
			repoName := ""
			for _, repo := range d.repositories {
				if strings.HasPrefix(destination, repo.Url) {
					linkType = LinkType_LINK_TYPE_NEIGHBOR_REPOSITORY
					repoName = repo.Name
					break
				}
			}

			linkOut = append(linkOut, &PageLink{
				Type:        linkType,
				Destination: destination,
				Repository:  repoName,
			})
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}

	seen := make(map[seenKey]struct{})
	for _, v := range linkOut {
		seen[seenKey{v.Type, v.Destination, v.Repository}] = struct{}{}
	}
	for v := range seen {
		page.LinkOut = append(page.LinkOut, &PageLink{Type: v.Type, Destination: v.Destination, Repository: v.Repository})
	}
	return page, nil
}

type seenKey struct {
	Type        LinkType
	Destination string
	Repository  string
}
