package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v32/github"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/githubutil"
	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/storage"
)

const (
	upstreamRemoteName = "origin"
)

type repositoryUpdater struct {
	repo     []*Repository
	timeout  time.Duration
	parallel int

	id            string
	storageClient *storage.S3
	lockFilePath  string
	s             *http.Server
	tokenProvider *githubutil.TokenProvider
}

func newRepositoryUpdater(storageClient *storage.S3, repo []*Repository, timeout time.Duration, lockFilePath string, tokenProvider *githubutil.TokenProvider, parallel int) (*repositoryUpdater, error) {
	if parallel == 0 {
		parallel = 1
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return nil, xerrors.WithStack(err)
	}
	return &repositoryUpdater{
		id:            hex.EncodeToString(buf),
		storageClient: storageClient,
		repo:          repo,
		timeout:       timeout,
		lockFilePath:  lockFilePath,
		parallel:      parallel,
		tokenProvider: tokenProvider,
	}, nil
}

func (u *repositoryUpdater) Run(ctx context.Context, interval time.Duration) {
	if err := u.acquireLock(ctx); err != nil {
		logger.Log.Error("Failed to get the lock", logger.Error(err))
		return
	}

	timer := time.NewTicker(interval)
	for {
		select {
		case <-timer.C:
			u.update(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (u *repositoryUpdater) acquireLock(ctx context.Context) error {
	if u.lockFilePath == "" {
		return nil
	}

	logger.Log.Info("Acquiring the lock...", zap.String("id", u.id))
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
	logger.Log.Debug("Other process is running", zap.String("id", u.id), zap.Time("expire", lock.Expire))

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

func (u *repositoryUpdater) getLock(ctx context.Context) (*updaterLock, error) {
	lock := &updaterLock{}
	lockFileReader, err := u.storageClient.Get(ctx, u.lockFilePath)
	if err != nil {
		return nil, err
	}
	if err := json.NewDecoder(lockFileReader).Decode(&lock); err != nil {
		return nil, err
	}

	return lock, nil
}

func (u *repositoryUpdater) setLock(ctx context.Context) error {
	lock := updaterLock{Id: u.id, Expire: time.Now().Add(10 * time.Minute)}
	buf, err := json.Marshal(lock)
	if err != nil {
		return err
	}
	if err := u.storageClient.Put(ctx, u.lockFilePath, buf); err != nil {
		return err
	}
	logger.Log.Info("Got lock", zap.String("id", u.id))

	// To update the lock thread
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

func (u *repositoryUpdater) ListenWebhookReceiver(addr string) {
	logger.Log.Info("Start webhook receiver", zap.String("addr", addr))
	u.s = &http.Server{
		Addr:    addr,
		Handler: http.HandlerFunc(u.handleWebhook),
	}

	if err := u.s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Log.Info("Stop webhook receiver", logger.Error(err))
	}
}

func (u *repositoryUpdater) Stop(ctx context.Context) {
	if u.s != nil {
		u.s.Shutdown(ctx)
	}
}

func (u *repositoryUpdater) handleWebhook(w http.ResponseWriter, req *http.Request) {
	payload, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Log.Warn("Failed to read request body", logger.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	e, err := github.ParseWebHook(github.WebHookType(req), payload)
	if err != nil {
		logger.Log.Warn("Failed to parse request", logger.Error(err))
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	switch event := e.(type) {
	case *github.PushEvent:
		for _, v := range u.repo {
			if v.URL == event.Repo.GetGitURL() || v.URL == event.Repo.GetCloneURL() {
				logger.Log.Info("Update repository triggered by webhook", zap.String("repo", v.Name))
				go u.updateRepo(context.Background(), v.GoGit)
				break
			}
		}
	}
}

func (u *repositoryUpdater) update(ctx context.Context) {
	sem := make(chan struct{}, u.parallel)
	doneCh := make(chan struct{})
	for _, v := range u.repo {
		go func(repo *git.Repository) {
			sem <- struct{}{}
			defer func() { <-sem }()

			u.updateRepo(ctx, repo)

			doneCh <- struct{}{}
		}(v.GoGit)
	}

	done := 0
	for range doneCh {
		done++
		if done == len(u.repo) {
			break
		}
	}
}

func (u *repositoryUpdater) updateRepo(ctx context.Context, repo *git.Repository) {
	timeoutCtx, stop := context.WithTimeout(ctx, u.timeout)
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
		if err != git.NoErrAlreadyUpToDate {
			logger.Log.Warn("Failed fetch repository", logger.Error(err))
		}
		return
	}

	// Make references
	iter, err := repo.References()
	if err != nil {
		logger.Log.Warn("Failed get references", logger.Error(err))
		return
	}
	branches := make([]*plumbing.Reference, 0)
	for {
		ref, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Log.Warn("Failed get reference", logger.Error(err))
			break
		}
		if ref.Name().IsRemote() {
			branches = append(branches, ref)
		}
		if ref.Name().IsBranch() {
			logger.Log.Debug("Remove reference", zap.String("ref", ref.Name().String()))
			if err := repo.Storer.RemoveReference(ref.Name()); err != nil {
				logger.Log.Warn("Failed remove reference", logger.Error(err), zap.String("ref", ref.Name().String()))
				break
			}
		}
	}

	for _, ref := range branches {
		branchName := strings.TrimPrefix(ref.Name().String(), "refs/remotes/origin/")
		newRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName(branchName), ref.Hash())
		if err := repo.Storer.SetReference(newRef); err != nil {
			logger.Log.Warn("Failed create reference", logger.Error(err))
		}
	}
}
