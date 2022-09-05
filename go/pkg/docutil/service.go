package docutil

import (
	"bytes"
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"go.f110.dev/mono/go/pkg/git"
	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/queue"
	"go.f110.dev/mono/go/pkg/storage"
)

type ObjectStorageInterface interface {
	PutReader(ctx context.Context, name string, data io.Reader) error
	Delete(ctx context.Context, name string) error
	Get(ctx context.Context, name string) (io.ReadCloser, error)
	List(ctx context.Context, prefix string) ([]*storage.Object, error)
}

// pages represents links that had the page.
// The key of map is a file path.
type pages map[string]*page

type docSet struct {
	Pages      pages
	Repository *git.Repository
	Ref        plumbing.Hash
}

type page struct {
	Title   string
	LinkIn  []*PageLink
	LinkOut []*PageLink
}

type DocSearchService struct {
	client         git.GitDataClient
	storage        ObjectStorageInterface
	markdownParser parser.Parser
	httpClient     *http.Client

	repositories []*git.Repository
	// data is a cache data. The key of map is a name of the repository.
	data map[string]*docSet

	mu          sync.Mutex
	titleCaches map[string]*titleCache
}

func NewDocSearchService(client git.GitDataClient, b ObjectStorageInterface) *DocSearchService {
	g := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
		),
	)
	markdownParser := g.Parser()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxConnsPerHost = 100

	return &DocSearchService{
		client:         client,
		storage:        b,
		markdownParser: markdownParser,
		data:           make(map[string]*docSet),
		httpClient:     &http.Client{Transport: transport},
		titleCaches:    make(map[string]*titleCache),
	}
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
	docs, ok := d.data[req.Repo]
	if !ok {
		return nil, errors.New("repository not found")
	}
	page, ok := docs.Pages[req.Sha]
	if !ok {
		return nil, errors.New("path is not found")
	}
	return &ResponsePageLink{In: page.LinkIn, Out: page.LinkOut}, nil
}

func (d *DocSearchService) Initialize(ctx context.Context, workers, maxConns int) error {
	q := queue.NewSimple()

	for i := 0; i < maxConns; i++ {
		logger.Log.Debug("Start fetching page title worker", zap.Int("thread", i+1))
		go func() {
			d.gettingExternalLinkTitleWorker(ctx, q)
		}()
	}
	go d.updateTitleCacheOnPeriodically()

	if err := d.scanRepositories(ctx, workers); err != nil {
		return err
	}

	d.interpolateCitedLinks()
	d.interpolateLinkTitle(ctx, q)
	return nil
}

func (d *DocSearchService) interpolateCitedLinks() {
	for sourceRepoName, docs := range d.data {
		for sourcePath, sourcePage := range docs.Pages {
			for _, link := range sourcePage.LinkOut {
				switch link.Type {
				case LinkType_LINK_TYPE_IN_REPOSITORY:
					if _, ok := docs.Pages[link.Destination]; !ok {
						//log.Print(link.Destination)
					} else {
						dest := link.Destination
						if dest[0] == '/' {
							dest = dest[1:]
						} else {
							dest = path.Clean(path.Join(path.Dir(sourcePath), dest))
						}
						docs.Pages[dest].LinkIn = append(docs.Pages[dest].LinkIn, &PageLink{
							Type:   LinkType_LINK_TYPE_IN_REPOSITORY,
							Source: sourcePath,
							Title:  sourcePage.Title,
						})
					}
				case LinkType_LINK_TYPE_NEIGHBOR_REPOSITORY:
					if _, ok := d.data[link.Repository].Pages[link.Destination]; !ok {
						//log.Print(link.Destination)
					} else {
						destPage := d.data[link.Repository].Pages[link.Destination]
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

func (d *DocSearchService) interpolateLinkTitle(ctx context.Context, q *queue.Simple) {
	for sourceRepoName, docs := range d.data {
		externalLinkTitleCache := newTitleCache(ctx, d.storage, docs)

		for sourcePath, sourcePage := range docs.Pages {
			for _, link := range sourcePage.LinkOut {
				switch link.Type {
				case LinkType_LINK_TYPE_IN_REPOSITORY, LinkType_LINK_TYPE_NEIGHBOR_REPOSITORY:
					d.interpolateLinkTitleUnderRepository(sourceRepoName, link, sourcePath)
				case LinkType_LINK_TYPE_EXTERNAL:
					u := link.Destination
					if i := strings.IndexRune(u, '#'); i > 0 {
						u = u[:i]
					}

					if !strings.HasPrefix(u, "http") {
						// The url is not a web page
						continue
					}
					if t, ok := externalLinkTitleCache.Get(u); ok {
						link.Title = t
						continue
					}
					q.Enqueue(&pageLinkItem{PageLink: link, docs: docs})
				default:
					continue
				}
			}
		}
	}
}

func (d *DocSearchService) getOrNewTitleCache(ctx context.Context, docs *docSet) *titleCache {
	key := fmt.Sprintf("%s%s", docs.Ref.String(), docs.Repository.Name)

	d.mu.Lock()
	c, ok := d.titleCaches[key]
	if ok {
		d.mu.Unlock()
		return c
	}

	c = newTitleCache(ctx, d.storage, docs)
	d.titleCaches[key] = c
	d.mu.Unlock()

	return c
}

type pageLinkItem struct {
	*PageLink
	docs *docSet
}

func (d *DocSearchService) gettingExternalLinkTitleWorker(ctx context.Context, q *queue.Simple) {
	var cached, remote, failed, skipped int
	for {
		v := q.Dequeue()
		if v == nil {
			break
		}

		link := v.(*pageLinkItem)
		u := link.Destination
		if i := strings.IndexRune(u, '#'); i > 0 {
			u = u[:i]
		}

		if !strings.HasPrefix(u, "http") {
			// The url is not a web page
			skipped++
			continue
		}

		titleCache := d.getOrNewTitleCache(ctx, link.docs)
		if t, ok := titleCache.Get(u); ok {
			link.Title = t
			cached++
			continue
		}

		title, err := d.fetchExternalPageTitle(ctx, u)
		if err == nil {
			title = strings.TrimSpace(title)
			link.Title = title
			titleCache.Set(u, title)
			remote++
		} else {
			switch err.Error() {
			case "page not found", "title is not found":
				titleCache.Set(u, "")
				remote++
			default:
				failed++
			}

			if !errors.Is(err, context.Canceled) {
				logger.Log.Info("Failed to fetch page title", logger.Error(err), zap.String("url", link.Destination))
			}
		}
	}
	logger.Log.Debug("Fetched external link title",
		zap.Int("cached", cached),
		zap.Int("remote", remote),
		zap.Int("failed", failed),
		zap.Int("skipped", skipped),
	)
}

func (d *DocSearchService) updateTitleCacheOnPeriodically() {
	logger.Log.Debug("Start updating title cache file on periodically")

	t := time.NewTicker(1 * time.Minute)
	for {
		select {
		case <-t.C:
			d.mu.Lock()
			for _, c := range d.titleCaches {
				if err := c.Save(); err != nil {
					logger.Log.Warn("Failed to save title cache", logger.Error(err), zap.String("repo", c.docs.Repository.Name))
				}
			}
			d.mu.Unlock()
		}
	}
}

func (d *DocSearchService) interpolateLinkTitleUnderRepository(repoName string, link *PageLink, sourcePath string) {
	if link.Type == LinkType_LINK_TYPE_NEIGHBOR_REPOSITORY {
		repoName = link.Repository
	}

	dest := link.Destination
	if strings.ContainsRune(dest, '#') {
		dest = dest[:strings.IndexRune(dest, '#')]
	}
	if dest[0] == '/' {
		dest = dest[1:]
	} else {
		dest = path.Clean(path.Join(path.Dir(sourcePath), dest))
	}
	if v, ok := d.data[repoName].Pages[dest]; ok {
		link.Title = v.Title
		if strings.ContainsRune(link.Destination, '#') {
			link.Title += link.Destination[strings.IndexRune(link.Destination, '#'):]
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
	docs := &docSet{Pages: make(pages), Repository: repo}
	d.data[repo.Name] = docs

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
					docs.Pages[entry.Path] = page
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
	docs.Ref = plumbing.NewHash(tree.Sha)

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

func (d *DocSearchService) fetchExternalPageTitle(ctx context.Context, u string) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u, nil)
	if err != nil {
		cancel()
		return "", xerrors.WithMessage(err, "failed to create request")
	}
	res, err := d.httpClient.Do(req)
	if err != nil {
		cancel()
		return "", xerrors.WithMessage(err, "failed to send request")
	}
	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		cancel()
		return "", xerrors.New("page not found")
	default:
		cancel()
		logger.Log.Warn("The web page doesn't returns status 200", zap.Int("status", res.StatusCode), zap.String("url", u))
		return "", xerrors.New("failed to fetch the url")
	}

	node, err := html.Parse(res.Body)
	if err != nil {
		cancel()
		return "", xerrors.WithMessage(err, "failed to parse response body")
	}
	// We must cancel the context after reading response body.
	cancel()

	// Find title
	var title string
	stack := list.New()
	stack.PushBack(node)
	for stack.Len() != 0 {
		e := stack.Back()
		stack.Remove(e)

		node := e.Value.(*html.Node)
		if node.DataAtom == atom.Title {
			if node.FirstChild != nil {
				title = node.FirstChild.Data
			}
			break
		}

		for c := node.FirstChild; c != nil; c = c.NextSibling {
			stack.PushBack(c)
		}
	}
	logger.Log.Debug("Fetch page title", zap.String("title", title), zap.String("url", u))

	if title == "" {
		return "", errors.New("title is not found")
	}
	return title, nil
}

type titleCache struct {
	cache   map[string]string
	mu      sync.Mutex
	changed bool

	docs    *docSet
	storage ObjectStorageInterface
}

func newTitleCache(ctx context.Context, storage ObjectStorageInterface, docs *docSet) *titleCache {
	externalLinkTitleCache := make(map[string]string)
	buf, err := storage.Get(
		ctx,
		fmt.Sprintf("external_links/%s/%s.json", docs.Repository.Name, docs.Ref.String()),
	)
	if err == nil {
		if err := json.NewDecoder(buf).Decode(&externalLinkTitleCache); err != nil {
			logger.Log.Error("Failed to decode external link cache file", logger.Error(err))
		}
		if err := buf.Close(); err != nil {
			logger.Log.Error("Failed to close buffer", logger.Error(err))
		}
	}

	return &titleCache{cache: externalLinkTitleCache, storage: storage, docs: docs}
}

func (c *titleCache) Set(u, title string) {
	c.mu.Lock()
	c.cache[u] = title
	c.changed = true
	c.mu.Unlock()
}

func (c *titleCache) Get(u string) (string, bool) {
	c.mu.Lock()
	t, ok := c.cache[u]
	c.mu.Unlock()
	return t, ok
}

func (c *titleCache) Close() {
	if err := c.Save(); err != nil {
		logger.Log.Error("Failed to put external link cache file", logger.Error(err))
	}
}

func (c *titleCache) Save() error {
	c.mu.Lock()
	if !c.changed {
		c.mu.Unlock()
		return nil
	}
	data := make(map[string]string)
	for k, v := range c.cache {
		data[k] = v
	}
	c.mu.Unlock()

	cacheBuf := new(bytes.Buffer)
	if err := json.NewEncoder(cacheBuf).Encode(data); err == nil {
		// We must create the new context because the context may be closed.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		key := fmt.Sprintf("external_links/%s/%s.json", c.docs.Repository.Name, c.docs.Ref.String())
		logger.Log.Debug("Update title cache file", zap.String("key", key))
		if err := c.storage.PutReader(ctx, key, cacheBuf); err != nil {
			return err
		}
	}
	return nil
}
