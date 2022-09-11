package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/v32/github"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/pkg/logger"
)

const (
	upstreamRemoteName = "origin"
)

type repositoryUpdater struct {
	repo     []*Repository
	interval time.Duration
	timeout  time.Duration
	parallel int

	s *http.Server
}

func newRepositoryUpdater(repo []*Repository, interval, timeout time.Duration, parallel int) (*repositoryUpdater, error) {
	if parallel == 0 {
		parallel = 1
	}
	if timeout > interval {
		return nil, xerrors.New("timeout is longer than interval")
	}
	return &repositoryUpdater{repo: repo, interval: interval, timeout: timeout, parallel: parallel}, nil
}

func (u *repositoryUpdater) Run(ctx context.Context) {
	timer := time.NewTicker(u.interval)
	for {
		select {
		case <-timer.C:
			u.update(ctx)
		case <-ctx.Done():
			return
		}
	}
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
	sem := make(chan struct{}, u.interval)
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
	ctx, stop := context.WithTimeout(ctx, u.timeout)
	defer stop()

	err := repo.FetchContext(ctx, &git.FetchOptions{RemoteName: upstreamRemoteName})
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
