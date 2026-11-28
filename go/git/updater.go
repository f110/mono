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
	"go.f110.dev/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

// Open opens the repository from the backend, cloning from the configured URL if the backend doesn't have it yet.
// Open does nothing if the repository is already opened. It is safe to call concurrently.
func (r *RepositoryConfig) Open(ctx context.Context, backend Backend, tokenProvider *githubutil.TokenProvider, timeout time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.goGit != nil {
		return nil
	}

	gitRepo, err := backend.Open(ctx, r, tokenProvider, timeout)
	if gitRepo != nil {
		r.goGit = gitRepo
	}
	return err
}

// Backend is the storage of the repositories.
type Backend interface {
	// Open opens the repository. If the backend doesn't have the repository, Open clones it from the URL of repo within
	// initTimeout. If Open returns both the repository and an error, the repository is usable and the error happened
	// after opening it.
	Open(ctx context.Context, repo *RepositoryConfig, tokenProvider *githubutil.TokenProvider, initTimeout time.Duration) (*git.Repository, error)
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

	backend       Backend
	tokenProvider *githubutil.TokenProvider
	initTimeout   time.Duration
	running       bool
	// repackInterval is the interval of the repack of the repositories whose storer has Repack. Zero disables the repack.
	repackInterval time.Duration
}

func NewUpdater(backend Backend, tokenProvider *githubutil.TokenProvider, repos RepositoryProvider, workers int) *Updater {
	return &Updater{
		backend:       backend,
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

func (u *Updater) SetInitTimeout(d time.Duration) *Updater {
	u.initTimeout = d
	return u
}

func (u *Updater) SetRepackInterval(d time.Duration) *Updater {
	if u.running {
		panic("cannot set repack interval. Updater is already running")
	}
	u.repackInterval = d
	return u
}

// Run executes the periodic refresh loop and the periodic repack. It blocks until ctx is cancelled.
func (u *Updater) Run(ctx context.Context) {
	logger.Log.Info("Start updater", slog.Duration("refresh_interval", u.interval), slog.Duration("repack_interval", u.repackInterval),
		slog.Int("workers", u.parallel))
	u.running = true
	timer := time.NewTicker(u.interval)
	defer timer.Stop()
	var repackCh <-chan time.Time
	if u.repackInterval > 0 {
		repackTimer := time.NewTicker(u.repackInterval)
		defer repackTimer.Stop()
		repackCh = repackTimer.C
	}
	for {
		select {
		case <-timer.C:
			u.update(ctx)
		case <-repackCh:
			u.repack(ctx)
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
	if err := repo.Open(ctx, u.backend, u.tokenProvider, u.initTimeout); err != nil {
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

// repack repacks the repositories whose storer has Repack (e.g. DatabaseStorer).
func (u *Updater) repack(ctx context.Context) {
	repos, err := u.repos.Repositories(ctx)
	if err != nil {
		logger.Log.Warn("Failed to get the repositories", logger.E(err))
		return
	}
	for _, repo := range repos {
		if err := repo.Open(ctx, u.backend, u.tokenProvider, u.initTimeout); err != nil {
			logger.Log.Warn("Failed to open repository", logger.E(err), slog.String("repo", repo.Name))
			continue
		}
		r, ok := repo.goGit.Storer.(interface{ Repack(context.Context) error })
		if !ok {
			continue
		}
		start := time.Now()
		if err := r.Repack(ctx); err != nil {
			logger.Log.Warn("Failed to repack repository", logger.E(err), slog.String("repo", repo.Name))
			continue
		}
		logger.Log.Info("Repacked repository", slog.String("repo", repo.Name), slog.Duration("elapsed", time.Since(start)))
	}
}

func (u *Updater) updateRepo(ctx context.Context, repo *git.Repository) error {
	timeoutCtx, stop := ctxutil.WithTimeout(ctx, u.timeout)
	defer stop()

	err := repo.FetchContext(timeoutCtx, &git.FetchOptions{
		Auth:       basicAuth(ctx, u.tokenProvider),
		RemoteName: upstreamRemoteName,
	})
	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return nil
		}
		logger.Log.Warn("Failed fetch repository", logger.E(err))
		return xerrors.WithStack(err)
	}

	return updateBranches(repo)
}

// UpdaterService serves Sync of Updater via gRPC.
type UpdaterService struct {
	updater *Updater
}

var _ GitDataUpdaterServer = &UpdaterService{}

func NewUpdaterService(updater *Updater) *UpdaterService {
	return &UpdaterService{updater: updater}
}

func (s *UpdaterService) Sync(ctx context.Context, req *RequestSync) (*ResponseSync, error) {
	err := s.updater.Sync(ctx, req.Url)
	switch {
	case errors.Is(err, ErrRepositoryNotTracked):
		return nil, status.Error(codes.NotFound, err.Error())
	case err != nil:
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &ResponseSync{}, nil
}

// UpdaterClient calls Sync of Updater which is served by UpdaterService in the other process.
type UpdaterClient struct {
	client GitDataUpdaterClient
}

func NewUpdaterClient(conn grpc.ClientConnInterface) *UpdaterClient {
	return &UpdaterClient{client: NewGitDataUpdaterClient(conn)}
}

// Sync has the same semantics as Updater.Sync.
func (c *UpdaterClient) Sync(ctx context.Context, url string) error {
	_, err := c.client.Sync(ctx, &RequestSync{Url: url})
	if status.Code(err) == codes.NotFound {
		return ErrRepositoryNotTracked.WithStack()
	}
	return xerrors.WithStack(err)
}

// basicAuth returns the credential for GitHub. It returns nil if tokenProvider is nil or fails to get the token.
func basicAuth(ctx context.Context, tokenProvider *githubutil.TokenProvider) *gitHttp.BasicAuth {
	if tokenProvider == nil {
		return nil
	}
	v, err := tokenProvider.Token(ctx)
	if err != nil {
		return nil
	}
	return &gitHttp.BasicAuth{Username: "octocat", Password: v}
}

// updateBranches replaces the branches of repo with the remote branches of origin.
func updateBranches(repo *git.Repository) error {
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
