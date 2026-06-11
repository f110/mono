package git

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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
	"go.f110.dev/mono/go/githubutil"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/storage"
)

const (
	upstreamRemoteName = "origin"
)

type RepositoryConfig struct {
	Name   string
	URL    string
	Prefix string

	goGit *git.Repository
}

// Open opens the repository from object storage, cloning from the configured URL if it does not yet exist.
func (r *RepositoryConfig) Open(ctx context.Context, stClient *storage.S3, cachePool *client.SinglePool, tokenProvider *githubutil.TokenProvider, timeout time.Duration, disableInflatePackFile bool) error {
	storer := NewObjectStorageStorer(stClient, r.Prefix, cachePool)

	if ok, err := storer.Exist(); !ok && err == nil {
		initCtx, cancel := ctxutil.WithTimeout(ctx, timeout)

		slogger.Log.Info("Init repository", slog.String("name", r.Name), slog.String("url", r.URL), slog.String("prefix", r.Prefix))
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
		slogger.Log.Info("Inflate packfile", slog.String("name", r.Name))
		if err := InflatePackFile(ctx, stClient, r.Prefix, gitRepo); err != nil {
			return err
		}
	}

	return nil
}

// Updater periodically refreshes object-storage-backed repositories and
// handles incoming webhook events to trigger immediate updates. It implements
// http.Handler so the caller can wire it into their own HTTP server.
type Updater struct {
	mu       sync.Mutex
	repo     []*RepositoryConfig
	interval time.Duration
	timeout  time.Duration
	parallel int

	id                     string
	storageClient          *storage.S3
	cachePool              *client.SinglePool
	lockFilePath           string
	tokenProvider          *githubutil.TokenProvider
	initTimeout            time.Duration
	disableInflatePackFile bool
	dataService            *DataService
	running                bool
}

func NewUpdater(stClient *storage.S3, tokenProvider *githubutil.TokenProvider, repos []*RepositoryConfig, lockFilePath string, workers int) (*Updater, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return nil, xerrors.WithStack(err)
	}
	return &Updater{
		id:            hex.EncodeToString(buf),
		storageClient: stClient,
		repo:          repos,
		interval:      1 * time.Hour,
		timeout:       1 * time.Minute,
		initTimeout:   5 * time.Minute,
		lockFilePath:  lockFilePath,
		parallel:      workers,
		tokenProvider: tokenProvider,
	}, nil
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

func (u *Updater) SetInitTimeout(d time.Duration) *Updater {
	u.initTimeout = d
	return u
}

func (u *Updater) SetDisableInflatePackFile(b bool) *Updater {
	u.disableInflatePackFile = b
	return u
}

// SetDataService links a DataService so newly added repositories become
// available to gRPC requests in addition to being refreshed periodically.
func (u *Updater) SetDataService(s *DataService) *Updater {
	u.dataService = s
	return u
}

// Run executes the periodic refresh loop. It blocks until ctx is cancelled.
func (u *Updater) Run(ctx context.Context) {
	if err := u.acquireLock(ctx); err != nil {
		slogger.Log.Error("Failed to get the lock", slogger.E(err))
		return
	}

	slogger.Log.Info("Start updater", slog.Duration("refresh_interval", u.interval), slog.Int("workers", u.parallel))
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

// AddRepo opens the repository from object storage (cloning upstream if it
// does not yet exist), registers it for periodic refresh, and exposes it to
// the linked DataService if any. Triggers an immediate fetch in the
// background.
func (u *Updater) AddRepo(ctx context.Context, repo *RepositoryConfig) error {
	if err := repo.Open(ctx, u.storageClient, u.cachePool, u.tokenProvider, u.initTimeout, u.disableInflatePackFile); err != nil {
		return err
	}
	u.mu.Lock()
	u.repo = append(u.repo, repo)
	u.mu.Unlock()
	if u.dataService != nil {
		u.dataService.AddRepo(repo.Name, repo.goGit)
	}
	go u.updateRepo(context.Background(), repo.goGit)
	return nil
}

// ErrRepositoryNotTracked is returned by Sync when no configured repository
// matches the given URL.
var ErrRepositoryNotTracked = xerrors.Define("git: repository is not tracked")

// Sync fetches the repository whose URL matches the given value. It blocks
// until the fetch finishes and returns the fetch error, ErrRepositoryNotTracked
// if no matching repository is configured, or nil on success.
func (u *Updater) Sync(ctx context.Context, url string) error {
	u.mu.Lock()
	repos := append([]*RepositoryConfig(nil), u.repo...)
	u.mu.Unlock()
	for _, v := range repos {
		if v.URL == url {
			slogger.Log.Info("Sync repository triggered", slog.String("repo", v.Name))
			return u.updateRepo(ctx, v.goGit)
		}
	}
	return ErrRepositoryNotTracked.WithStack()
}

// ServeHTTP handles GitHub webhook events. A push event triggers an
// immediate fetch of the matching repository.
func (u *Updater) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	payload, err := io.ReadAll(req.Body)
	if err != nil {
		slogger.Log.Warn("Failed to read request body", slogger.E(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	e, err := github.ParseWebHook(github.WebHookType(req), payload)
	if err != nil {
		slogger.Log.Warn("Failed to parse request", slogger.E(err))
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

func (u *Updater) acquireLock(ctx context.Context) error {
	if u.lockFilePath == "" {
		return nil
	}

	slogger.Log.Info("Acquiring the lock...", slog.String("id", u.id))
	lock, err := u.getLock(ctx)
	if errors.Is(err, storage.ErrObjectNotFound) {
		if err := u.setLock(ctx); err != nil {
			return err
		}
		return nil
	}
	if time.Now().After(lock.Expire) {
		if err := u.setLock(ctx); err != nil {
			return err
		}
	}
	slogger.Log.Debug("Other process is running", slog.String("id", u.id), slog.Time("expire", lock.Expire))

	t := time.NewTicker(9 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			lock, err := u.getLock(ctx)
			if err != nil {
				continue
			}
			if time.Now().After(lock.Expire) {
				if err := u.setLock(ctx); err != nil {
					return err
				}
			}
		}
	}
}

type updaterLock struct {
	Id     string
	Expire time.Time
}

func (u *Updater) getLock(ctx context.Context) (*updaterLock, error) {
	lock := &updaterLock{}
	lockFileReader, err := u.storageClient.Get(ctx, u.lockFilePath)
	if err != nil {
		return nil, err
	}
	if err := json.NewDecoder(lockFileReader.Body).Decode(&lock); err != nil {
		return nil, err
	}

	return lock, nil
}

func (u *Updater) setLock(ctx context.Context) error {
	lock := updaterLock{Id: u.id, Expire: time.Now().Add(10 * time.Minute)}
	buf, err := json.Marshal(lock)
	if err != nil {
		return err
	}
	if err := u.storageClient.Put(ctx, u.lockFilePath, buf); err != nil {
		return err
	}
	slogger.Log.Info("Got lock", slog.String("id", u.id))

	go func() {
		t := time.NewTicker(9 * time.Minute)
		defer t.Stop()

		select {
		case <-t.C:
			if err := u.setLock(ctx); err != nil {
				return
			}
		}
	}()

	return nil
}

func (u *Updater) update(ctx context.Context) {
	u.mu.Lock()
	repos := append([]*RepositoryConfig(nil), u.repo...)
	u.mu.Unlock()

	sem := make(chan struct{}, u.parallel)
	doneCh := make(chan struct{})
	for _, v := range repos {
		slogger.Log.Info("Updating repository", slog.String("repo", v.Name))
		go func(repo *git.Repository) {
			sem <- struct{}{}
			defer func() { <-sem }()

			_ = u.updateRepo(ctx, repo)

			doneCh <- struct{}{}
		}(v.goGit)
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
		slogger.Log.Warn("Failed fetch repository", slogger.E(err))
		return xerrors.WithStack(err)
	}

	iter, err := repo.References()
	if err != nil {
		slogger.Log.Warn("Failed get references", slogger.E(err))
		return xerrors.WithStack(err)
	}
	branches := make([]*plumbing.Reference, 0)
	for {
		ref, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			slogger.Log.Warn("Failed get reference", slogger.E(err))
			return xerrors.WithStack(err)
		}
		if ref.Name().IsRemote() {
			branches = append(branches, ref)
		}
		if ref.Name().IsBranch() {
			slogger.Log.Debug("Remove reference", slog.String("ref", ref.Name().String()))
			if err := repo.Storer.RemoveReference(ref.Name()); err != nil {
				slogger.Log.Warn("Failed remove reference", slogger.E(err), slog.String("ref", ref.Name().String()))
				return xerrors.WithStack(err)
			}
		}
	}

	for _, ref := range branches {
		branchName := strings.TrimPrefix(ref.Name().String(), "refs/remotes/origin/")
		newRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName(branchName), ref.Hash())
		if err := repo.Storer.SetReference(newRef); err != nil {
			slogger.Log.Warn("Failed create reference", slogger.E(err))
			return xerrors.WithStack(err)
		}
	}
	return nil
}
