package repoindexer

import (
	"bytes"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/go-github/v32/github"
	"github.com/google/zoekt"
	"github.com/google/zoekt/build"
	"github.com/shurcooL/githubv4"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/logger"
)

type Indexer struct {
	config      *Config
	workDir     string
	token       string
	ctags       string
	initRun     bool
	parallelism int

	repositories []*repository

	githubClient  *github.Client
	graphQLClient *githubv4.Client
}

func NewIndexer(rules *Config, workDir, token, ctags string, initRun bool, parallelism int) *Indexer {
	return &Indexer{config: rules, workDir: workDir, token: token, ctags: ctags, initRun: initRun, parallelism: parallelism}
}

func (x *Indexer) Sync() error {
	repositories := x.listRepositories()
	for _, v := range repositories {
		logger.Log.Debug("Found repository", zap.String("name", v.Name), zap.String("url", v.URL))

		if err := v.sync(x.workDir, x.initRun); err != nil {
			logger.Log.Info("Failed sync repository", zap.Error(err), zap.String("url", v.URL))
			continue
		}
	}

	return nil
}

func (x *Indexer) BuildIndex() error {
	for _, v := range x.listRepositories() {
		t1 := time.Now()
		m := newRepositoryMutator(v)
		branchRefs, err := m.Mutate(x.workDir, v.Refs)
		if err != nil {
			logger.Log.Info("Failed to mutate repository", zap.String("name", v.Name), zap.Error(err))
			continue
		}

		files := make(map[file]map[plumbing.Hash]struct{})
		fileBranches := make(map[file][]string)
		for _, refName := range branchRefs {
			f, err := v.files(refName)
			if err != nil {
				logger.Log.Info("Failed traverse the tree", zap.String("name", v.Name), zap.String("ref", refName.String()), zap.Error(err))
				continue
			}
			for k, v := range f {
				if _, ok := files[k]; !ok {
					files[k] = make(map[plumbing.Hash]struct{})
				}
				files[k][v] = struct{}{}
				fileBranches[k] = append(fileBranches[k], refName.Short())
			}
		}

		branches := make([]zoekt.RepositoryBranch, 0)
		for _, v := range v.Refs {
			branches = append(branches, zoekt.RepositoryBranch{Name: strings.TrimPrefix(v.Short(), "origin/")})
		}
		opt := build.Options{
			IndexDir: filepath.Join(x.workDir, ".index"),
			RepositoryDescription: zoekt.Repository{
				Name:     v.Name,
				Branches: branches,
			},
			CTags:       x.ctags,
			Parallelism: x.parallelism,
		}
		opt.SetDefaults()
		builder, err := build.NewBuilder(opt)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		t2 := time.Now()
		queue := make(chan file, x.parallelism)
		var docCount int32
		var wg sync.WaitGroup
		for i := 0; i < x.parallelism; i++ {
			wg.Add(1)
			go func() {
				x.worker(queue, builder, v, fileBranches, &docCount)
				wg.Done()
			}()
		}
		for f := range files {
			queue <- f
		}
		close(queue)
		wg.Wait()

		logger.Log.Info("Total document",
			zap.String("name", v.Name),
			zap.Int32("count", docCount),
			zap.Duration("elapsed", time.Since(t1)),
			zap.Duration("indexing_elapsed", time.Since(t2)),
		)
		if err := builder.Finish(); err != nil {
			return xerrors.Errorf(": %w", err)
		}

		if err := v.cleanup(x.workDir); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func (x *Indexer) worker(queue chan file, builder *build.Builder, repo *repository, fileBranches map[file][]string, docCount *int32) {
	for {
		f, ok := <-queue
		if !ok {
			return
		}
		if err := x.addDocument(builder, repo, f, fileBranches); err != nil {
			logger.Log.Info("Failed to add document", zap.String("name", repo.Name), zap.String("path", f.path), zap.Error(err))
		} else {
			atomic.AddInt32(docCount, 1)
		}
	}
}

func (x *Indexer) addDocument(builder *build.Builder, repo *repository, f file, fileBranches map[file][]string) error {
	t := time.Now()
	blob, err := repo.repo.BlobObject(f.hash)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	buf := new(bytes.Buffer)
	r, err := blob.Reader()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	buf.Grow(int(blob.Size))
	_, err = buf.ReadFrom(r)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	r.Close()

	brs := fileBranches[f]
	if err := builder.Add(zoekt.Document{
		Name:     f.path,
		Content:  buf.Bytes(),
		Branches: brs,
	}); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	logger.Log.Debug("Add document",
		zap.String("name", f.path),
		zap.Strings("branches", brs),
		zap.Duration("elapsed", time.Since(t)),
	)

	return nil
}

func (x *Indexer) Cleanup() error {
	indexDir := filepath.Join(x.workDir, ".index")
	entry, err := os.ReadDir(indexDir)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	files := make(map[string]struct{}, 0)
	for _, v := range entry {
		b := filepath.Base(v.Name())
		files[b] = struct{}{}
	}

	for _, v := range x.listRepositories() {
		n := url.QueryEscape(v.Name)
		if len(n) > 200 {
			h := sha1.New()
			io.WriteString(h, n)
			s := fmt.Sprintf("%x", h.Sum(nil))[:8]
			n = n[:200] + s
		}
		for f := range files {
			if strings.HasPrefix(f, n) {
				delete(files, f)
			}
		}
	}
	for f := range files {
		logger.Log.Debug("Delete index", zap.String("filename", f))
		if err := os.Remove(filepath.Join(indexDir, f)); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

type repositoryMutator struct {
	repo *repository
}

func newRepositoryMutator(repo *repository) *repositoryMutator {
	return &repositoryMutator{repo: repo}
}

func (m *repositoryMutator) Mutate(workDir string, refs []plumbing.ReferenceName) ([]plumbing.ReferenceName, error) {
	branchRefs := make([]plumbing.ReferenceName, 0)

	for _, refName := range refs {
		logger.Log.Debug("Prepare", zap.String("name", m.repo.Name), zap.String("ref", refName.Short()))
		dir := filepath.Join(workDir, m.repo.Name)
		if branchRef, err := m.repo.checkout(workDir, refName); err != nil {
			logger.Log.Info("Failed checkout repository", zap.Error(err), zap.String("name", m.repo.Name))
			continue
		} else {
			branchRefs = append(branchRefs, branchRef)
		}

		if !m.repo.DisableVendoring {
			logger.Log.Debug("Vendoring", zap.String("name", m.repo.Name))
			err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}
				if info.Name() == "go.mod" {
					if strings.Contains(strings.TrimPrefix(path, dir), "vendor/") {
						return nil
					}
					if strings.Contains(strings.TrimPrefix(path, dir), "testdata/") {
						return nil
					}

					if _, err := os.Stat(filepath.Join(filepath.Dir(path), "vendor")); !os.IsNotExist(err) {
						return nil
					}

					logger.Log.Info("Run go mod vendor", zap.String("go.mod", path))
					cmd := exec.Command("go", "mod", "vendor")
					cmd.Dir = filepath.Dir(path)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					if err := cmd.Run(); err != nil {
						return err
					}
				}

				return nil
			})
			if err != nil {
				logger.Log.Info("Failed go vendoring", zap.String("name", m.repo.Name), zap.Error(err))
				continue
			}
		}

		if err := m.repo.newCommit(); err != nil {
			logger.Log.Info("Failed create commit", zap.String("name", m.repo.Name), zap.Error(err))
			continue
		}

		if err := m.repo.cleanWorktree(); err != nil {
			logger.Log.Info("Failed clean worktree", zap.String("name", m.repo.Name), zap.Error(err))
			continue
		}
	}

	return branchRefs, nil
}

type repository struct {
	Name             string
	URL              string
	DisableVendoring bool
	Refs             []plumbing.ReferenceName

	repo *git.Repository
}

func (x *repository) sync(workDir string, initRun bool) error {
	// Clean up a directory for bare repository
	bareDir := filepath.Join(workDir, ".bare", x.Name)
	if _, err := os.Stat(bareDir); !os.IsNotExist(err) {
		logger.Log.Info("Remove bare repository", zap.String("dir", bareDir))
		if err := os.RemoveAll(bareDir); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	dir := filepath.Join(workDir, x.Name)
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		logger.Log.Info("Remove old directory", zap.String("dir", dir))
		// Old style directory If .git directory not exists.
		if err := os.RemoveAll(dir); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		logger.Log.Debug("Clone", zap.String("name", x.Name), zap.String("url", x.URL))
		_, err = git.PlainCloneContext(context.TODO(), dir, false, &git.CloneOptions{
			URL:        x.URL,
			NoCheckout: true,
		})
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}
	if initRun {
		return nil
	}

	r, err := git.PlainOpen(dir)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	logger.Log.Debug("Fetch", zap.String("name", x.Name))
	err = r.Fetch(&git.FetchOptions{
		Progress: os.Stdout,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (x *repository) checkout(workDir string, refName plumbing.ReferenceName) (plumbing.ReferenceName, error) {
	dir := filepath.Join(workDir, x.Name)
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return "", xerrors.Errorf(": %w", err)
	}
	x.repo = repo
	hash, err := x.resolveReference(refName)
	if err != nil {
		return "", xerrors.Errorf(": %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return "", xerrors.Errorf(": %w", err)
	}
	branchRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName(strings.TrimPrefix(refName.Short(), "origin/")), hash)
	if ref, err := repo.Reference(branchRef.Name(), true); err == plumbing.ErrReferenceNotFound {
		// Skip
	} else if err != nil {
		return "", xerrors.Errorf(": %w", err)
	} else {
		logger.Log.Debug("Remove branch(reference)", zap.String("name", ref.Name().String()), zap.String("name", x.Name))
		if err := repo.Storer.RemoveReference(ref.Name()); err != nil {
			return "", xerrors.Errorf(": %w", err)
		}
	}
	logger.Log.Debug("Set reference", zap.String("ref", branchRef.Name().String()), zap.String("hash", branchRef.Hash().String()), zap.String("name", x.Name))
	if err := repo.Storer.SetReference(branchRef); err != nil {
		return "", xerrors.Errorf(": %w", err)
	}
	logger.Log.Debug("Checkout", zap.String("branch", branchRef.Name().String()))
	err = wt.Checkout(&git.CheckoutOptions{
		Branch: branchRef.Name(),
	})
	if err != nil {
		return "", xerrors.Errorf(": %w", err)
	}
	x.repo = repo

	return branchRef.Name(), nil
}

func (x *repository) resolveReference(refName plumbing.ReferenceName) (plumbing.Hash, error) {
	hash := plumbing.ZeroHash
	ref, err := x.repo.Reference(refName, false)
	if err != nil {
		return hash, xerrors.Errorf(": %w", err)
	}
	obj, err := x.repo.Object(plumbing.AnyObject, ref.Hash())
	if err != nil {
		return hash, xerrors.Errorf(": %w", err)
	}
	switch v := obj.(type) {
	case *object.Tag:
		hash = v.Target
	case *object.Commit:
		hash = v.Hash
	}

	return hash, nil
}

type file struct {
	path string
	hash plumbing.Hash
}

func (f file) String() string {
	return fmt.Sprintf("%s: %s", f.path, f.hash.String())
}

func (x *repository) files(refName plumbing.ReferenceName) (map[file]plumbing.Hash, error) {
	ref, err := x.repo.Reference(refName, true)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	commit, err := x.repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	files := make(map[file]plumbing.Hash)
	w := object.NewTreeWalker(tree, true, make(map[plumbing.Hash]bool))
	for {
		name, entry, err := w.Next()
		if err == io.EOF {
			break
		}
		if entry.Mode.IsFile() {
			files[file{path: name, hash: entry.Hash}] = entry.Hash
		}
	}

	return files, nil
}

func (x *repository) newCommit() error {
	wt, err := x.repo.Worktree()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	st, err := wt.Status()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	for path := range st {
		if _, err := wt.Add(path); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	if _, err := wt.Commit("new commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "indexer",
			Email: "example@example.com",
		},
	}); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (x *repository) cleanWorktree() error {
	wt, err := x.repo.Worktree()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if err := wt.Clean(&git.CleanOptions{Dir: true}); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (x *repository) cleanup(workDir string) error {
	dir := filepath.Join(workDir, x.Name)
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	iter, err := repo.Branches()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	for {
		ref, err := iter.Next()
		if err != io.EOF {
			break
		}
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		logger.Log.Debug("Branch", zap.String("name", x.Name), zap.String("branch", ref.String()))
	}

	return nil
}

func (x *Indexer) listRepositories() []*repository {
	if x.repositories != nil {
		return x.repositories
	}

	repos := make([]*repository, 0)

	for _, rule := range x.config.Rules {
		if rule.Owner != "" && rule.Name != "" {
			repo, _, err := x.githubRESTClient().Repositories.Get(context.Background(), rule.Owner, rule.Name)
			if err != nil {
				logger.Log.Info("Repository is not found", zap.String("owner", rule.Owner), zap.String("name", rule.Name))
				continue
			}
			repos = append(repos, &repository{
				Name:             fmt.Sprintf("%s/%s", rule.Owner, rule.Name),
				URL:              repo.GetGitURL(),
				Refs:             x.refSpecs(rule.Branches, rule.Tags),
				DisableVendoring: rule.DisableVendoring,
			})
		}

		if rule.Query != "" {
			vars := map[string]interface{}{
				"searchQuery": githubv4.String(rule.Query),
				"cursor":      (*githubv4.String)(nil),
			}
			err := x.githubGraphQLClient().Query(context.Background(), &listRepositoriesQuery, vars)
			if err != nil {
				logger.Log.Info("Failed execute query", zap.Error(err))
				continue
			}
			for _, v := range listRepositoriesQuery.Search.Nodes {
				if v.Type != "Repository" {
					continue
				}
				if v.Repository.IsArchived {
					continue
				}
				repos = append(repos, &repository{
					Name:             fmt.Sprintf("%s/%s", v.Repository.Owner.Login, v.Repository.Name),
					URL:              v.Repository.URL.String(),
					Refs:             x.refSpecs(rule.Branches, rule.Tags),
					DisableVendoring: rule.DisableVendoring,
				})
			}
			if !listRepositoriesQuery.Search.PageInfo.HasNextPage {
				break
			}
			vars["cursor"] = listRepositoriesQuery.Search.PageInfo.EndCursor
		}
	}

	x.repositories = repos
	return repos
}

func (*Indexer) refSpecs(branches, tags []string) []plumbing.ReferenceName {
	refs := make([]plumbing.ReferenceName, 0, len(branches)+len(tags))
	for _, v := range branches {
		refs = append(refs, plumbing.NewRemoteReferenceName("origin", v))
	}
	for _, v := range tags {
		refs = append(refs, plumbing.NewTagReferenceName(v))
	}

	return refs
}

func (x *Indexer) githubRESTClient() *github.Client {
	if x.githubClient != nil {
		return x.githubClient
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: x.token})
	tc := oauth2.NewClient(context.Background(), ts)

	x.githubClient = github.NewClient(tc)
	return x.githubClient
}

func (x *Indexer) githubGraphQLClient() *githubv4.Client {
	if x.graphQLClient != nil {
		return x.graphQLClient
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: x.token})
	tc := oauth2.NewClient(context.Background(), ts)
	x.graphQLClient = githubv4.NewClient(tc)
	return x.graphQLClient
}

var listRepositoriesQuery struct {
	Search struct {
		PageInfo struct {
			EndCursor   githubv4.String
			HasNextPage bool
		}
		Nodes []struct {
			Type       string           `graphql:"__typename"`
			Repository RepositorySchema `graphql:"... on Repository"`
		}
	} `graphql:"search(query: $searchQuery type: REPOSITORY first: 100 after: $cursor)"`
}

type RepositorySchema struct {
	Name  string
	Owner struct {
		Login string
	}
	URL        githubv4.URI
	IsArchived bool
}
