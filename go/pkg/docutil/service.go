package docutil

import (
	"context"
	"errors"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"gopkg.in/src-d/go-git.v4/plumbing"

	"go.f110.dev/mono/go/pkg/git"
	"go.f110.dev/mono/go/pkg/logger"
)

// pages represents links that had the page.
// The key of map is a file path.
type pages map[string]*page

type page struct {
	Title   string
	LinkIn  []*PageLink
	LinkOut []*PageLink
}

type DocSearchService struct {
	client         git.GitDataClient
	markdownParser parser.Parser

	repositories []*git.Repository
	// data is a cache data. The key of map is a name of the repository.
	data map[string]pages
}

func NewDocSearchService(client git.GitDataClient) *DocSearchService {
	g := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
		),
	)
	markdownParser := g.Parser()
	return &DocSearchService{client: client, markdownParser: markdownParser, data: make(map[string]pages)}
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
	links, ok := d.data[req.Repo]
	if !ok {
		return nil, errors.New("repository not found")
	}
	page, ok := links[req.Sha]
	if !ok {
		return nil, errors.New("path is not found")
	}
	return &ResponsePageLink{In: page.LinkIn, Out: page.LinkOut}, nil
}

func (d *DocSearchService) Initialize(ctx context.Context, workers int) error {
	if err := d.scanRepositories(ctx, workers); err != nil {
		return err
	}

	d.interpolateLinks()
	return nil
}

func (d *DocSearchService) interpolateLinks() {
	for sourceRepoName, pages := range d.data {
		for sourcePath, sourcePage := range pages {
			for _, link := range sourcePage.LinkOut {
				switch link.Type {
				case LinkType_LINK_TYPE_IN_REPOSITORY:
					if _, ok := pages[link.Destination]; !ok {
						//log.Print(link.Destination)
					} else {
						dest := link.Destination
						if dest[0] == '/' {
							dest = dest[1:]
						} else {
							dest = path.Clean(path.Join(path.Dir(sourcePath), dest))
						}
						pages[dest].LinkIn = append(pages[dest].LinkIn, &PageLink{
							Type:   LinkType_LINK_TYPE_IN_REPOSITORY,
							Source: sourcePath,
							Title:  sourcePage.Title,
						})
					}
				case LinkType_LINK_TYPE_NEIGHBOR_REPOSITORY:
					if _, ok := d.data[link.Repository][link.Destination]; !ok {
						//log.Print(link.Destination)
					} else {
						destPage := d.data[link.Repository][link.Destination]
						destPage.LinkIn = append(destPage.LinkIn, &PageLink{
							Type:       LinkType_LINK_TYPE_NEIGHBOR_REPOSITORY,
							Source:     sourcePath,
							Repository: sourceRepoName,
							Title:      sourcePage.Title,
						})
					}
				}
			}
		}
	}
}

func (d *DocSearchService) scanRepositories(ctx context.Context, workers int) error {
	t1 := time.Now()
	repos, err := d.client.ListRepositories(ctx, &git.RequestListRepositories{})
	if err != nil {
		return xerrors.WithMessage(err, "Failed to get list of repository")
	}
	d.repositories = repos.Repositories

	for _, v := range repos.Repositories {
		t1 := time.Now()
		if err := d.scanRepository(ctx, v, workers); err != nil {
			return xerrors.WithMessagef(err, "Failed to scan the repository: %s", v.Name)
		}
		logger.Log.Debug("ScanRepository", zap.String("repo", v.Name), zap.Duration("duration", time.Since(t1)))
	}

	logger.Log.Debug("ScanRepositories", zap.Duration("duration", time.Since(t1)), zap.Int("num", len(d.repositories)))
	return nil
}

func (d *DocSearchService) scanRepository(ctx context.Context, repo *git.Repository, workers int) error {
	var mu sync.Mutex
	pageLinks := make(pages)
	d.data[repo.Name] = pageLinks

	ch := make(chan *git.TreeEntry, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				entry, ok := <-ch
				if !ok {
					break
				}
				page, err := d.makePage(ctx, repo, entry.Sha)
				if err != nil {
					logger.Log.Error("Failed to make page", logger.Error(err))
				} else {
					mu.Lock()
					pageLinks[entry.Path] = page
					mu.Unlock()
				}
			}
		}()
	}

	tree, err := d.client.GetTree(ctx, &git.RequestGetTree{
		Repo:      repo.Name,
		Ref:       plumbing.NewBranchReferenceName(repo.DefaultBranch).String(),
		Path:      "/",
		Recursive: true,
	})
	if err != nil {
		return xerrors.WithStack(err)
	}

	for _, v := range tree.Tree {
		switch filepath.Ext(v.Path) {
		case ".md":
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			ch <- v
		}
	}
	close(ch)

	timeout := time.After(1 * time.Minute)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-timeout:
		return xerrors.New("timed out to scan the repository")
	case <-done:
		return nil
	}
}

func (d *DocSearchService) makePage(ctx context.Context, repo *git.Repository, sha string) (*page, error) {
	blob, err := d.client.GetBlob(ctx, &git.RequestGetBlob{Repo: repo.Name, Sha: sha})
	if err != nil {
		return nil, xerrors.WithMessage(err, "Failed to get blob")
	}

	rootNode := d.markdownParser.Parse(text.NewReader(blob.Content))
	page, err := d.makePageFromMarkdownAST(rootNode, repo, blob.Content)
	if err != nil {
		return nil, xerrors.WithMessage(err, "Failed to parse markdown")
	}

	return page, nil
}

func (d *DocSearchService) makePageFromMarkdownAST(rootNode ast.Node, repo *git.Repository, raw []byte) (*page, error) {
	page := &page{}

	var linkOut []*PageLink
	seen := make(map[seenKey]struct{})
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

				if _, ok := seen[seenKey{LinkType_LINK_TYPE_IN_REPOSITORY, destination, ""}]; !ok {
					seen[seenKey{LinkType_LINK_TYPE_IN_REPOSITORY, destination, ""}] = struct{}{}
					linkOut = append(linkOut, &PageLink{
						Type:        LinkType_LINK_TYPE_IN_REPOSITORY,
						Destination: destination,
					})
				}
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

				if _, ok := seen[seenKey{linkType, destination, repoName}]; !ok {
					seen[seenKey{linkType, destination, repoName}] = struct{}{}
					linkOut = append(linkOut, &PageLink{
						Type:        linkType,
						Destination: destination,
						Repository:  repoName,
					})
				}
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

			if _, ok := seen[seenKey{linkType, destination, repoName}]; !ok {
				seen[seenKey{linkType, destination, repoName}] = struct{}{}
				linkOut = append(linkOut, &PageLink{
					Type:        linkType,
					Destination: destination,
					Repository:  repoName,
				})
			}
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}
	page.LinkOut = linkOut

	// Find document title
	child := rootNode.FirstChild()
	for child != nil {
		if v, ok := child.(*ast.Heading); !ok {
			child = child.NextSibling()
			continue
		} else {
			if v.Level == 1 {
				page.Title = string(v.Text(raw))
			}
			break
		}
	}
	return page, nil
}

type seenKey struct {
	Type        LinkType
	Destination string
	Repository  string
}
