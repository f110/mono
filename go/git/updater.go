package git

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v85/github"
	"go.f110.dev/go-memcached/client"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/ctxutil"
	githubutil "go.f110.dev/mono/go/github"
	"go.f110.dev/mono/go/logger"
)

const (
	upstreamRemoteName = "origin"
)

type RepositoryConfig struct {
	Name   string
	URL    string
	Prefix string

	mu    sync.Mutex
	goGit *git.Repository
}

// Open opens the repository from object storage, cloning from the configured URL if it does not yet exist.
// Open does nothing if the repository is already opened. It is safe to call concurrently.
func (r *RepositoryConfig) Open(ctx context.Context, stClient ObjectStorageInterface, cachePool *client.SinglePool, packCache *PackfileCache, tokenProvider *githubutil.TokenProvider, timeout time.Duration, disableInflatePackFile bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.goGit != nil {
		return nil
	}

	storer := NewObjectStorageStorer(stClient, r.Prefix, cachePool, packCache)

	if ok, err := storer.Exist(); !ok && err == nil {
		initCtx, cancel := ctxutil.WithTimeout(ctx, timeout)

		logger.Log.Info("Init repository", slog.String("name", r.Name), slog.String("url", r.URL), slog.String("prefix", r.Prefix))
		var auth *gitHttp.BasicAuth
		if tokenProvider != nil {
			if v, err := tokenProvider.Token(initCtx); err == nil {
				auth = &gitHttp.BasicAuth{Username: "octocat", Password: v}
			}
		}
		if _, err := InitObjectStorageRepository(initCtx, stClient, r.URL, r.Prefix, auth); err != nil {
			cancel()
			return err
		}
		cancel()
	} else if err != nil {
		return err
	}

	gitRepo, err := git.Open(storer, nil)
	if err != nil {
		return xerrors.WithStack(err)
	}
	r.goGit = gitRepo

	if !disableInflatePackFile && storer.IncludePackFile(ctx) {
		logger.Log.Info("Inflate packfile", slog.String("name", r.Name))
		if err := InflatePackFile(ctx, stClient, r.Prefix, gitRepo); err != nil {
			return err
		}
	}

	return nil
}

// RepositoryProvider provides the repositories which are refreshed by Updater.
// Updater opens the returned repositories if they are not opened yet.
type RepositoryProvider interface {
	Repositories(ctx context.Context) ([]*RepositoryConfig, error)
}

// StaticRepositoryProvider provides the fixed list of the repositories.
type StaticRepositoryProvider struct {
	repos []*RepositoryConfig
}

var _ RepositoryProvider = &StaticRepositoryProvider{}

func NewStaticRepositoryProvider(repos []*RepositoryConfig) *StaticRepositoryProvider {
	return &StaticRepositoryProvider{repos: repos}
}

func (p *StaticRepositoryProvider) Repositories(_ context.Context) ([]*RepositoryConfig, error) {
	return p.repos, nil
}

// Updater periodically refreshes object-storage-backed repositories and
// handles incoming webhook events to trigger immediate updates. It implements
// http.Handler so the caller can wire it into their own HTTP server.
//
// Updater doesn't coordinate with other processes. If the repositories are
// shared by multiple processes, the caller must hold the lock before calling Run.
type Updater struct {
	repos    RepositoryProvider
	interval time.Duration
	timeout  time.Duration
	parallel int

	storageClient          ObjectStorageInterface
	cachePool              *client.SinglePool
	packCache              *PackfileCache
	tokenProvider          *githubutil.TokenProvider
	initTimeout            time.Duration
	disableInflatePackFile bool
	running                bool
}

func NewUpdater(stClient ObjectStorageInterface, tokenProvider *githubutil.TokenProvider, repos RepositoryProvider, workers int) *Updater {
	return &Updater{
		storageClient: stClient,
		repos:         repos,
		interval:      1 * time.Hour,
		timeout:       1 * time.Minute,
		initTimeout:   5 * time.Minute,
		parallel:      workers,
		tokenProvider: tokenProvider,
	}
}

func (u *Updater) SetInterval(d time.Duration) *Updater {
	if u.running != false {
		panic("cannot set interval. Updater is already running")
	}
	u.interval = d
	return u
}

func (u *Updater) SetTimeout(d time.Duration) *Updater {
	u.timeout = d
	return u
}

func (u *Updater) SetCachePool(c *client.SinglePool) *Updater {
	u.cachePool = c
	return u
}

func (u *Updater) SetPackCache(c *PackfileCache) *Updater {
	u.packCache = c
	return u
}

func (u *Updater) SetInitTimeout(d time.Duration) *Updater {
	u.initTimeout = d
	return u
}

func (u *Updater) SetDisableInflatePackFile(b bool) *Updater {
	u.disableInflatePackFile = b
	return u
}

// Run executes the periodic refresh loop. It blocks until ctx is cancelled.
func (u *Updater) Run(ctx context.Context) {
	logger.Log.Info("Start updater", slog.Duration("refresh_interval", u.interval), slog.Int("workers", u.parallel))
	u.running = true
	timer := time.NewTicker(u.interval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			u.update(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// ErrRepositoryNotTracked is returned by Sync when no configured repository
// matches the given URL.
var ErrRepositoryNotTracked = xerrors.Define("git: repository is not tracked")

// Sync fetches the repository whose URL matches the given value. It blocks
// until the fetch finishes and returns the fetch error, ErrRepositoryNotTracked
// if no matching repository is configured, or nil on success.
func (u *Updater) Sync(ctx context.Context, url string) error {
	repos, err := u.repos.Repositories(ctx)
	if err != nil {
		return err
	}
	for _, v := range repos {
		if v.URL == url {
			logger.Log.Info("Sync repository triggered", slog.String("repo", v.Name))
			return u.syncRepo(ctx, v)
		}
	}
	return ErrRepositoryNotTracked.WithStack()
}

func (u *Updater) syncRepo(ctx context.Context, repo *RepositoryConfig) error {
	if err := repo.Open(ctx, u.storageClient, u.cachePool, u.packCache, u.tokenProvider, u.initTimeout, u.disableInflatePackFile); err != nil {
		logger.Log.Warn("Failed to open repository", logger.E(err), slog.String("repo", repo.Name))
		return err
	}
	return u.updateRepo(ctx, repo.goGit)
}

// ServeHTTP handles GitHub webhook events. A push event triggers an
// immediate fetch of the matching repository.
func (u *Updater) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	payload, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Log.Warn("Failed to read request body", logger.E(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	e, err := github.ParseWebHook(github.WebHookType(req), payload)
	if err != nil {
		logger.Log.Warn("Failed to parse request", logger.E(err))
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	switch event := e.(type) {
	case *github.PushEvent:
		go func() {
			if err := u.Sync(context.Background(), event.Repo.GetGitURL()); err != nil && errors.Is(err, ErrRepositoryNotTracked) {
				_ = u.Sync(context.Background(), event.Repo.GetCloneURL())
			}
		}()
	}
}

func (u *Updater) update(ctx context.Context) {
	repos, err := u.repos.Repositories(ctx)
	if err != nil {
		logger.Log.Warn("Failed to get the repositories", logger.E(err))
		return
	}

	sem := make(chan struct{}, u.parallel)
	doneCh := make(chan struct{})
	for _, v := range repos {
		logger.Log.Info("Updating repository", slog.String("repo", v.Name))
		go func(repo *RepositoryConfig) {
			sem <- struct{}{}
			defer func() { <-sem }()

			_ = u.syncRepo(ctx, repo)

			doneCh <- struct{}{}
		}(v)
	}

	done := 0
	for range doneCh {
		done++
		if done == len(repos) {
			break
		}
	}
}

func (u *Updater) updateRepo(ctx context.Context, repo *git.Repository) error {
	timeoutCtx, stop := ctxutil.WithTimeout(ctx, u.timeout)
	defer stop()

	var auth *gitHttp.BasicAuth
	if u.tokenProvider != nil {
		if v, err := u.tokenProvider.Token(ctx); err == nil {
			auth = &gitHttp.BasicAuth{
				Username: "octocat",
				Password: v,
			}
		}
	}
	err := repo.FetchContext(timeoutCtx, &git.FetchOptions{
		Auth:       auth,
		RemoteName: upstreamRemoteName,
	})
	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return nil
		}
		logger.Log.Warn("Failed fetch repository", logger.E(err))
		return xerrors.WithStack(err)
	}

	iter, err := repo.References()
	if err != nil {
		logger.Log.Warn("Failed get references", logger.E(err))
		return xerrors.WithStack(err)
	}
	branches := make([]*plumbing.Reference, 0)
	for {
		ref, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Log.Warn("Failed get reference", logger.E(err))
			return xerrors.WithStack(err)
		}
		if ref.Name().IsRemote() {
			branches = append(branches, ref)
		}
		if ref.Name().IsBranch() {
			logger.Log.Debug("Remove reference", slog.String("ref", ref.Name().String()))
			if err := repo.Storer.RemoveReference(ref.Name()); err != nil {
				logger.Log.Warn("Failed remove reference", logger.E(err), slog.String("ref", ref.Name().String()))
				return xerrors.WithStack(err)
			}
		}
	}

	for _, ref := range branches {
		branchName := strings.TrimPrefix(ref.Name().String(), "refs/remotes/origin/")
		newRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName(branchName), ref.Hash())
		if err := repo.Storer.SetReference(newRef); err != nil {
			logger.Log.Warn("Failed create reference", logger.E(err))
			return xerrors.WithStack(err)
		}
	}
	return nil
}
