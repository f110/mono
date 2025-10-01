package main

import (
	"context"
	"net/http"
	"os/exec"
	"testing"

	"github.com/google/go-github/v73/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/githubutil"
	"go.f110.dev/mono/go/varptr"
)

func TestJujutsuPRSubmitCommand(t *testing.T) {
	if _, err := exec.LookPath("jj"); err != nil {
		t.Skipf("Skip %s because jj is not found", t.Name())
	}

	t.Run("StateCreatePR", func(t *testing.T) {
		ghMock := githubutil.NewMock()
		repo := ghMock.Repository("f110/mono")
		ghClient := github.NewClient(&http.Client{Transport: ghMock.RegisteredTransport()})

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
		if pr := repo.AssertPullRequest(t, 1); pr != nil {
			assert.Equal(t, "push-wlkxotovqzqn", pr.Head.GetRef())
			assert.Equal(t, "master", pr.Base.GetRef())
			assert.Equal(t, "crypto: Fix security issue", pr.GetTitle())
		}
		if pr := repo.AssertPullRequest(t, 2); pr != nil {
			assert.Equal(t, "push-ulplmwrqqxyx", pr.Head.GetRef())
			assert.Equal(t, "push-wlkxotovqzqn", pr.Base.GetRef())
			assert.Equal(t, "math: Add", pr.GetTitle())
		}
		if pr := repo.AssertPullRequest(t, 3); pr != nil {
			assert.Equal(t, "push-ylsnsuvootnp", pr.Head.GetRef())
			assert.Equal(t, "push-ulplmwrqqxyx", pr.Base.GetRef())
			assert.Equal(t, "util: Fix", pr.GetTitle())
		}
	})

	t.Run("StateUpdatePR", func(t *testing.T) {
		ghMock := githubutil.NewMock()
		repo := ghMock.Repository("f110/mono")
		ghClient := ghMock.Client()
		repo.PullRequests(
			&github.PullRequest{
				Number: varptr.Ptr(1),
				Base:   &github.PullRequestBranch{Ref: varptr.Ptr("master")},
				Head:   &github.PullRequestBranch{Ref: varptr.Ptr("push-wlkxotovqzqn")},
				Title:  varptr.Ptr("crypto: Fix security issue"),
				Body:   varptr.Ptr("This PR contains fixing some security issues."),
			},
			&github.PullRequest{
				Number: varptr.Ptr(2),
				Base:   &github.PullRequestBranch{Ref: varptr.Ptr("push-wlkxotovqzqn")},
				Head:   &github.PullRequestBranch{Ref: varptr.Ptr("push-ulplmwrqqxyx")},
				Title:  varptr.Ptr("math: Add"),
				Body:   varptr.Ptr("This PR improves math package."),
			},
			&github.PullRequest{
				Number: varptr.Ptr(3),
				Base:   &github.PullRequestBranch{Ref: varptr.Ptr("push-ulplmwrqqxyx")},
				Head:   &github.PullRequestBranch{Ref: varptr.Ptr("push-ylsnsuvootnp")},
				Title:  varptr.Ptr("util: Fix"),
				Body:   varptr.Ptr("This PR fixes the bug."),
			},
		)

		c := newSubmitCommand()
		c.ghClient = ghClient
		c.repositoryOwner, c.repositoryName = "f110", "mono"
		c.DefaultBranch = "master"
		c.stack = []*commit{
			{
				ChangeID: "ylsnsuvootnpnwoxvokynlptorzkmxwy", CommitID: "b947bd3ba890e5252f1a151014f72ade7ca03a03", Bookmarks: []*bookmark{{Name: "push-ylsnsuvootnp"}},
				Description: `util: Fix

This PR fixes the bug.`,
				PullRequest: newPullRequest(&repo.GetPullRequest(3).PullRequest),
			},
			{
				ChangeID: "ulplmwrqqxyxszouwwopptsttrlsnnsk", CommitID: "a505cb91edb706ac06c6fb6667adeb4502f6c346", Bookmarks: []*bookmark{{Name: "push-ulplmwrqqxyx"}},
				Description: `math: Add

This PR improves math package.`,
				PullRequest: newPullRequest(&repo.GetPullRequest(2).PullRequest),
			},
			{
				ChangeID: "wlkxotovqzqnpvsowvwknyzwvqokqlko", CommitID: "a505cb91edb706ac06c6fb6667adeb4502f6c346", Bookmarks: []*bookmark{{Name: "push-wlkxotovqzqn"}},
				Description: `crypto: Fix security issue

This PR contains fixing some security issues.`,
				PullRequest: newPullRequest(&repo.GetPullRequest(1).PullRequest),
			},
		}

		nextState, err := c.updatePR(context.Background())
		require.NoError(t, err)
		assert.Equal(t, c.stateClose, nextState)
	})
}
