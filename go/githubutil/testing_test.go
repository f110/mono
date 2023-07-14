package githubutil

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/go-github/v49/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMock(t *testing.T) {
	t.Run("PullRequestService", func(t *testing.T) {
		t.Run("Create", func(t *testing.T) {
			m := NewMock()
			m.Repository("f110/gh-test")
			ghClient := github.NewClient(&http.Client{Transport: m.RegisteredTransport()})

			pr, _, err := ghClient.PullRequests.Create(context.Background(), "f110", "gh-test", &github.NewPullRequest{})
			require.NoError(t, err)
			assert.Equal(t, 1, pr.GetNumber())
		})

		t.Run("Edit", func(t *testing.T) {
			m := NewMock()
			repo := m.Repository("f110/gh-test")
			ghClient := github.NewClient(&http.Client{Transport: m.RegisteredTransport()})
			repo.PullRequests = []*github.PullRequest{
				{
					Number: github.Int(1),
					Title:  github.String(t.Name()),
					Body:   github.String("PR description"),
					Base:   &github.PullRequestBranch{Ref: github.String("master")},
					Head:   &github.PullRequestBranch{Ref: github.String("feature-1")},
				},
			}

			pr, _, err := ghClient.PullRequests.Edit(context.Background(), "f110", "gh-test", 1, &github.PullRequest{
				Base: &github.PullRequestBranch{Ref: github.String("main")},
			})
			require.NoError(t, err)
			assert.Equal(t, t.Name(), pr.GetTitle())
			assert.Equal(t, "main", pr.GetBase().GetRef())
		})
	})
}
