package main

import (
	"context"
	"net/http"
	"os/exec"
	"testing"

	"github.com/google/go-github/v73/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.f110.dev/githubmock"
)

func TestJujutsuPRSubmitCommand(t *testing.T) {
	if _, err := exec.LookPath("jj"); err != nil {
		t.Skipf("Skip %s because jj is not found", t.Name())
	}

	t.Run("StateCreatePR", func(t *testing.T) {
		ghMock := githubmock.NewMock()
		ghMock.Repository("f110/mono")
		ghClient := github.NewClient(&http.Client{Transport: ghMock.Transport()})

		c := newSubmitCommand()
		c.ghClient = ghClient
		c.repositoryOwner, c.repositoryName = "f110", "mono"
		c.DefaultBranch = "master"
		c.RootDir = t.TempDir()
		c.stack = []*commit{
			{
				ChangeID: "ylsnsuvootnpnwoxvokynlptorzkmxwy", CommitID: "b947bd3ba890e5252f1a151014f72ade7ca03a03", Bookmarks: []*bookmark{{Name: "push-ylsnsuvootnp"}},
				Description: `util: Fix

This PR fixes the bug.`,
			},
			{
				ChangeID: "ulplmwrqqxyxszouwwopptsttrlsnnsk", CommitID: "a505cb91edb706ac06c6fb6667adeb4502f6c346", Bookmarks: []*bookmark{{Name: "push-ulplmwrqqxyx"}},
				Description: `math: Add

This PR improves math package.`,
			},
			{
				ChangeID: "wlkxotovqzqnpvsowvwknyzwvqokqlko", CommitID: "a505cb91edb706ac06c6fb6667adeb4502f6c346", Bookmarks: []*bookmark{{Name: "push-wlkxotovqzqn"}},
				Description: `crypto: Fix security issue

This PR contains fixing some security issues.`,
			},
		}

		nextState, err := c.createPR(context.Background())
		require.NoError(t, err)
		assert.Equal(t, c.stateUpdatePR, nextState)
		if pr, _, err := ghClient.PullRequests.Get(t.Context(), "f110", "mono", 1); assert.NoError(t, err) {
			assert.Equal(t, "push-wlkxotovqzqn", pr.GetHead().GetRef())
			assert.Equal(t, "master", pr.GetBase().GetRef())
			assert.Equal(t, "crypto: Fix security issue", pr.GetTitle())
		}
		if pr, _, err := ghClient.PullRequests.Get(t.Context(), "f110", "mono", 2); assert.NoError(t, err) {
			assert.Equal(t, "push-ulplmwrqqxyx", pr.GetHead().GetRef())
			assert.Equal(t, "push-wlkxotovqzqn", pr.GetBase().GetRef())
			assert.Equal(t, "math: Add", pr.GetTitle())
		}
		if pr, _, err := ghClient.PullRequests.Get(t.Context(), "f110", "mono", 3); assert.NoError(t, err) {
			assert.Equal(t, "push-ylsnsuvootnp", pr.GetHead().GetRef())
			assert.Equal(t, "push-ulplmwrqqxyx", pr.GetBase().GetRef())
			assert.Equal(t, "util: Fix", pr.GetTitle())
		}
	})

	t.Run("StateUpdatePR", func(t *testing.T) {
		ghMock := githubmock.NewMock()
		repo := ghMock.Repository("f110/mono")
		ghClient := github.NewClient(&http.Client{Transport: ghMock.Transport()})
		repo.PullRequests(
			githubmock.NewPullRequest().
				Number(1).
				Base("master").
				Head(nil, "push-wlkxotovqzqn", "").
				Title("crypto: Fix security issue").
				Body("This PR contains fixing some security issues."),
			githubmock.NewPullRequest().
				Number(2).
				Base("push-wlkxotovqzqn").
				Head(nil, "push-ulplmwrqqxyx", "").
				Title("math: Add").
				Body("This PR improves math package."),
			githubmock.NewPullRequest().
				Number(3).
				Base("push-ulplmwrqqxyx").
				Head(nil, "push-ylsnsuvootnp", "").
				Title("util: Fix").
				Body("This PR fixes the bug."),
		)

		pr1, _, err := ghClient.PullRequests.Get(t.Context(), "f110", "mono", 1)
		require.NoError(t, err)
		pr2, _, err := ghClient.PullRequests.Get(t.Context(), "f110", "mono", 2)
		require.NoError(t, err)
		pr3, _, err := ghClient.PullRequests.Get(t.Context(), "f110", "mono", 3)
		require.NoError(t, err)

		c := newSubmitCommand()
		c.ghClient = ghClient
		c.repositoryOwner, c.repositoryName = "f110", "mono"
		c.DefaultBranch = "master"
		c.stack = []*commit{
			{
				ChangeID: "ylsnsuvootnpnwoxvokynlptorzkmxwy", CommitID: "b947bd3ba890e5252f1a151014f72ade7ca03a03", Bookmarks: []*bookmark{{Name: "push-ylsnsuvootnp"}},
				Description: `util: Fix

This PR fixes the bug.`,
				PullRequest: newPullRequest(pr3),
			},
			{
				ChangeID: "ulplmwrqqxyxszouwwopptsttrlsnnsk", CommitID: "a505cb91edb706ac06c6fb6667adeb4502f6c346", Bookmarks: []*bookmark{{Name: "push-ulplmwrqqxyx"}},
				Description: `math: Add

This PR improves math package.`,
				PullRequest: newPullRequest(pr2),
			},
			{
				ChangeID: "wlkxotovqzqnpvsowvwknyzwvqokqlko", CommitID: "a505cb91edb706ac06c6fb6667adeb4502f6c346", Bookmarks: []*bookmark{{Name: "push-wlkxotovqzqn"}},
				Description: `crypto: Fix security issue

This PR contains fixing some security issues.`,
				PullRequest: newPullRequest(pr1),
			},
		}

		nextState, err := c.updatePR(context.Background())
		require.NoError(t, err)
		assert.Equal(t, c.stateClose, nextState)
	})
}
