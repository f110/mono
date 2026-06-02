package main

import (
	"context"
	"flag"
	"net/http"
	"testing"

	"github.com/google/go-github/v85/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.f110.dev/githubmock"

	"go.f110.dev/mono/go/testing/assertion"
)

var jjBinaryPath *string

func init() {
	jjBinaryPath = flag.String("test.jj-binary", "", "")
}

func TestJujutsuPRSubmitCommand(t *testing.T) {
	if *jjBinaryPath == "" {
		t.Skipf("Skip %s because -test.jj-binary is not set", t.Name())
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

func TestSplitDescription(t *testing.T) {
	cases := []struct {
		name      string
		desc      string
		wantTitle string
		wantBody  string
	}{
		{
			name:      "subject only",
			desc:      "fix: nothing",
			wantTitle: "fix: nothing",
		},
		{
			name:      "subject and body, no trailers",
			desc:      "fix: bug\n\nThis fixes the bug.",
			wantTitle: "fix: bug",
			wantBody:  "This fixes the bug.",
		},
		{
			name:      "single trailer line is stripped",
			desc:      "fix: bug\n\nThis fixes the bug.\n\nFixes: #123",
			wantTitle: "fix: bug",
			wantBody:  "This fixes the bug.",
		},
		{
			name:      "paragraph containing a non-trailer line is not treated as a trailer",
			desc:      "fix: bug\n\nThis fixes the bug.\n\nNot: foo\nbar",
			wantTitle: "fix: bug",
			wantBody:  "This fixes the bug.\n\nNot: foo\nbar",
		},
		{
			name:      "multi-line trailer block is stripped",
			desc:      "fix: bug\n\nThis fixes the bug.\n\nFixes: #123\nCo-Authored-By: Foo <foo@example.com>",
			wantTitle: "fix: bug",
			wantBody:  "This fixes the bug.",
		},
		{
			name:      "body is empty when only trailers are present",
			desc:      "fix: bug\n\nFixes: #123",
			wantTitle: "fix: bug",
			wantBody:  "",
		},
		{
			name:      "everything after the first trailer paragraph is dropped",
			desc:      "fix: bug\n\nBody paragraph.\n\nFirst: trailer\n\nSecond: paragraph after trailer",
			wantTitle: "fix: bug",
			wantBody:  "Body paragraph.",
		},
		{
			name:      "multi-paragraph body without trailers is preserved",
			desc:      "fix: bug\n\nFirst paragraph.\n\nSecond paragraph.",
			wantTitle: "fix: bug",
			wantBody:  "First paragraph.\n\nSecond paragraph.",
		},
		{
			name:      "body line containing a URL is not mistaken for a trailer",
			desc:      "fix: bug\n\nSee https://example.com for details.",
			wantTitle: "fix: bug",
			wantBody:  "See https://example.com for details.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			title, body := splitDescription(tc.desc)
			assertion.Equal(t, title, tc.wantTitle)
			assertion.Equal(t, body, tc.wantBody)
		})
	}
}
