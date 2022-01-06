package repoindexer

import (
	"bytes"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gogitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/zoekt"
	"github.com/google/zoekt/build"
	"go.uber.org/zap"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/logger"
)

const (
	goProxy = "https://proxy.golang.org"
)

type Indexer struct {
	Indexes []*RepositoryIndex

	workDir        string
	ctags          string
	initRun        bool
	parallelism    int
	appId          int64
	installationId int64
	privateKeyFile string

	lister *RepositoryLister
}

func NewIndexer(
	rules *Config,
	workDir, token, ctags string,
	appId, installationId int64,
	privateKeyFile string,
	initRun bool,
	parallelism int,
) (*Indexer, error) {
	var listerOpts []RepositoryListerOpt
	if appId > 0 && installationId > 0 && privateKeyFile != "" {
		listerOpts = []RepositoryListerOpt{GitHubApp(appId, installationId, privateKeyFile)}
	} else if token != "" {
		listerOpts = []RepositoryListerOpt{GitHubToken(token)}
	} else {
		listerOpts = []RepositoryListerOpt{WithoutCredential()}
	}
	lister, err := NewRepositoryLister(rules.Rules, listerOpts...)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return &Indexer{
		workDir:        workDir,
		ctags:          ctags,
		initRun:        initRun,
		parallelism:    parallelism,
		appId:          appId,
		installationId: installationId,
		privateKeyFile: privateKeyFile,
		lister:         lister,
	}, nil
}

func (x *Indexer) Sync(ctx context.Context) error {
	repositories := x.lister.List(ctx)
	for _, v := range repositories {
		logger.Log.Debug("Found repository", zap.String("name", v.Name), zap.String("url", v.URL))

		if err := v.sync(ctx, x.workDir, x.appId, x.installationId, x.privateKeyFile, x.initRun); err != nil {
			logger.Log.Info("Failed sync repository", zap.Error(err), zap.String("url", v.URL))
			continue
		}
	}

	return nil
}

func (x *Indexer) BuildIndex(ctx context.Context) error {
	indexDir := filepath.Join(x.workDir, ".index")

	for _, v := range x.lister.List(ctx) {
		t1 := time.Now()
		m := newRepositoryMutator(v)
		branchRefs, err := m.Mutate(ctx, x.workDir, v.Refs)
		if err != nil {
			logger.Log.Info("Failed to mutate repository", zap.String("name", v.Name), zap.Error(err))
			continue
		}
		t2 := time.Now()

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
			IndexDir: indexDir,
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

		t3 := time.Now()
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
		doneCh := make(chan struct{})
		go func() {
			wg.Wait()
			close(doneCh)
		}()
		select {
		case <-doneCh:
		case <-ctx.Done():
			return ctx.Err()
		}

		logger.Log.Info("Total document",
			zap.String("name", v.Name),
			zap.Int32("count", docCount),
			zap.Duration("elapsed", time.Since(t1)),
			zap.Duration("mutating_elapsed", t2.Sub(t1)),
			zap.Duration("indexing_elapsed", time.Since(t3)),
		)
		if err := builder.Finish(); err != nil {
			return xerrors.Errorf(": %w", err)
		}

		if err := v.cleanup(x.workDir); err != nil {
			return xerrors.Errorf(": %w", err)
		}

		index, err := NewRepositoryIndex(v.Name, indexDir)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		x.Indexes = append(x.Indexes, index)
	}

	return nil
}

func (x *Indexer) Reset() {
	x.lister.ClearCache()
}

func (x *Indexer) worker(queue chan file, builder *build.Builder, repo *Repository, fileBranches map[file][]string, docCount *int32) {
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

func (x *Indexer) addDocument(builder *build.Builder, repo *Repository, f file, fileBranches map[file][]string) error {
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

func (x *Indexer) Cleanup(ctx context.Context) error {
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

	for _, v := range x.lister.List(ctx) {
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
	repo *Repository
}

func newRepositoryMutator(repo *Repository) *repositoryMutator {
	return &repositoryMutator{repo: repo}
}

func (m *repositoryMutator) Mutate(ctx context.Context, workDir string, refs []plumbing.ReferenceName) ([]plumbing.ReferenceName, error) {
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
			logger.Log.Debug("Vendoring", zap.String("name", m.repo.Name), zap.String("dir", dir))
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
					cmd := exec.CommandContext(ctx, "go", "mod", "vendor")
					cmd.Dir = filepath.Dir(path)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					cmd.Env = append(os.Environ(), fmt.Sprintf("GOPROXY=%s", goProxy))
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

		logger.Log.Debug("Commit", zap.String("name", m.repo.Name), zap.String("ref", refName.Short()))
		if err := m.repo.newCommit(); err != nil {
			logger.Log.Info("Failed create commit", zap.String("name", m.repo.Name), zap.Error(err))
			continue
		}

		logger.Log.Debug("Clean worktree", zap.String("name", m.repo.Name), zap.String("ref", refName.Short()))
		if err := m.repo.cleanWorktree(); err != nil {
			logger.Log.Info("Failed clean worktree", zap.String("name", m.repo.Name), zap.Error(err))
			continue
		}
	}

	return branchRefs, nil
}

func (x *Repository) sync(ctx context.Context, workDir string, appId, installationId int64, privateKeyFile string, initRun bool) error {
	var auth transport.AuthMethod
	if privateKeyFile != "" {
		t, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appId, installationId, privateKeyFile)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		token, err := t.Token(context.Background())
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		auth = &gogitHttp.BasicAuth{Username: "octocat", Password: token}
	}

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
		_, err = git.PlainCloneContext(ctx, dir, false, &git.CloneOptions{
			URL:        x.URL,
			NoCheckout: true,
			Auth:       auth,
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
	err = r.FetchContext(ctx, &git.FetchOptions{
		Progress: os.Stdout,
		Auth:     auth,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (x *Repository) checkout(workDir string, refName plumbing.ReferenceName) (plumbing.ReferenceName, error) {
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
		st, err := wt.Status()
		if err == nil {
			fmt.Printf("%v\n", st)
		}
		return "", xerrors.Errorf(": %w", err)
	}

	return branchRef.Name(), nil
}

func (x *Repository) resolveReference(refName plumbing.ReferenceName) (plumbing.Hash, error) {
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

func (x *Repository) files(refName plumbing.ReferenceName) (map[file]plumbing.Hash, error) {
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

func (x *Repository) newCommit() error {
	wt, err := x.repo.Worktree()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	err = wt.AddWithOptions(&git.AddOptions{All: true})
	if err != nil {
		return xerrors.Errorf(": %w", err)
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

func (x *Repository) cleanWorktree() error {
	wt, err := x.repo.Worktree()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	ref, err := x.repo.Head()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if err := wt.Reset(&git.ResetOptions{Commit: ref.Hash(), Mode: git.HardReset}); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if err := wt.Clean(&git.CleanOptions{Dir: true}); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (x *Repository) cleanup(workDir string) error {
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

type RepositoryIndex struct {
	Name  string
	Files []string
}

func NewRepositoryIndex(name, indexDir string) (*RepositoryIndex, error) {
	entry, err := os.ReadDir(indexDir)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	n := url.QueryEscape(name)
	if len(n) > 200 {
		h := sha1.New()
		io.WriteString(h, n)
		s := fmt.Sprintf("%x", h.Sum(nil))[:8]
		n = n[:200] + s
	}

	files := make([]string, 0)
	for _, v := range entry {
		if strings.HasPrefix(v.Name(), n) {
			files = append(files, filepath.Join(indexDir, v.Name()))
		}
	}

	return &RepositoryIndex{Name: name, Files: files}, nil
}
