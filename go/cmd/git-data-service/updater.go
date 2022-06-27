package main

import (
	"context"
	"time"

	"github.com/go-git/go-git/v5"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/pkg/logger"
)

const (
	upstreamRemoteName = "origin"
)

type repositoryUpdater struct {
	repo     []*git.Repository
	interval time.Duration
	timeout  time.Duration
	parallel int
}

func newRepositoryUpdater(repo []*git.Repository, interval, timeout time.Duration, parallel int) (*repositoryUpdater, error) {
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
			u.update()
		case <-ctx.Done():
			return
		}
	}
}

func (u *repositoryUpdater) update() {
	sem := make(chan struct{}, u.interval)
	doneCh := make(chan struct{})
	for _, v := range u.repo {
		go func(repo *git.Repository) {
			sem <- struct{}{}
			defer func() { <-sem }()

			u.updateRepo(repo)

			doneCh <- struct{}{}
		}(v)
	}

	done := 0
	for range doneCh {
		done++
		if done == len(u.repo) {
			break
		}
	}
}

func (u *repositoryUpdater) updateRepo(repo *git.Repository) {
	ctx, stop := context.WithTimeout(context.Background(), u.timeout)
	defer stop()

	err := repo.FetchContext(ctx, &git.FetchOptions{RemoteName: upstreamRemoteName})
	if err != nil {
		logger.Log.Warn("Failed fetch repository", zap.Error(err))
	}
}
