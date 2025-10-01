package githubutil

import (
	"context"
	"testing"

	"github.com/google/go-github/v73/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/varptr"
)

func TestMock(t *testing.T) {
	t.Run("PullRequestService", func(t *testing.T) {
		t.Run("Create", func(t *testing.T) {
			m := NewMock()
			m.Repository("f110/gh-test")
			ghClient := m.Client()

			pr, _, err := ghClient.PullRequests.Create(context.Background(), "f110", "gh-test", &github.NewPullRequest{})
			require.NoError(t, err)
			assert.Equal(t, 1, pr.GetNumber())
		})

		t.Run("Edit", func(t *testing.T) {
			m := NewMock()
			repo := m.Repository("f110/gh-test")
			ghClient := m.Client()
			repo.PullRequests(
				&github.PullRequest{
					Number: varptr.Ptr(1),
					Title:  varptr.Ptr(t.Name()),
					Body:   varptr.Ptr("PR description"),
					Base:   &github.PullRequestBranch{Ref: varptr.Ptr("master")},
					Head:   &github.PullRequestBranch{Ref: varptr.Ptr("feature-1")},
				},
			)

			pr, _, err := ghClient.PullRequests.Edit(context.Background(), "f110", "gh-test", 1, &github.PullRequest{
				Base: &github.PullRequestBranch{Ref: varptr.Ptr("main")},
			})
			require.NoError(t, err)
			assert.Equal(t, t.Name(), pr.GetTitle())
			assert.Equal(t, "main", pr.GetBase().GetRef())
		})

		t.Run("CreateComment", func(t *testing.T) {
			m := NewMock()
			repo := m.Repository("f110/gh-test")
			ghClient := m.Client()
			repo.PullRequests(
				&github.PullRequest{
					Number: varptr.Ptr(1),
					Title:  varptr.Ptr(t.Name()),
				},
			)

			comment, _, err := ghClient.PullRequests.CreateComment(context.Background(), "f110", "gh-test", 1, &github.PullRequestComment{
				Body: varptr.Ptr("Comment"),
			})
			require.NoError(t, err)
			assert.NotNil(t, comment)
			pr := repo.GetPullRequest(1)
			require.NotNil(t, pr)
			assert.Len(t, pr.Comments, 1)
		})
	})
}
