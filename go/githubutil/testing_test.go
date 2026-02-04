package githubutil

import (
	"context"
	"testing"

	"github.com/google/go-github/v73/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/testing/assertion"
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

	t.Run("GitService", func(t *testing.T) {
		m := NewMock()
		repo := m.Repository("f110/gh-test")
		repo.Files(File{Name: ".github/CODEOWNERS"}, File{Name: "/docs/sample/README.md"})
		repo.Files(File{Name: ".build/mirror.cue"}, File{Name: ".build/test.cue"})
		repo.Files(File{Name: "README.md", Body: []byte("README")})

		ghClient := m.Client()

		t.Run("GetCommit", func(t *testing.T) {
			commit, _, err := ghClient.Git.GetCommit(t.Context(), "f110", "gh-test", "HEAD")
			assertion.MustNoError(t, err)
			assertion.NotEmpty(t, commit.GetTree().GetSHA())
		})

		t.Run("GetTree", func(t *testing.T) {
			commit, _, err := ghClient.Git.GetCommit(t.Context(), "f110", "gh-test", "HEAD")
			assertion.MustNoError(t, err)

			tree, _, err := ghClient.Git.GetTree(t.Context(), "f110", "gh-test", commit.GetTree().GetSHA(), false)
			assertion.MustNoError(t, err)
			docsSHA := ""
			buildSHA := ""
			for _, v := range tree.Entries {
				switch *v.Path {
				case "docs":
					docsSHA = *v.SHA
				case ".build":
					buildSHA = *v.SHA
				}
			}
			assertion.MustNotEmpty(t, docsSHA)
			assertion.MustNotEmpty(t, buildSHA)

			tree, _, err = ghClient.Git.GetTree(t.Context(), "f110", "gh-test", docsSHA, false)
			assertion.MustNoError(t, err)
			assertion.Len(t, tree.Entries, 1)

			tree, _, err = ghClient.Git.GetTree(t.Context(), "f110", "gh-test", buildSHA, false)
			assertion.MustNoError(t, err)
			assertion.Len(t, tree.Entries, 2)
		})

		t.Run("GetBlobRaw", func(t *testing.T) {
			commit, _, err := ghClient.Git.GetCommit(t.Context(), "f110", "gh-test", "HEAD")
			assertion.MustNoError(t, err)
			tree, _, err := ghClient.Git.GetTree(t.Context(), "f110", "gh-test", commit.GetTree().GetSHA(), false)
			assertion.MustNoError(t, err)
			sha := ""
			for _, v := range tree.Entries {
				if v.GetPath() == "README.md" {
					sha = v.GetSHA()
					break
				}
			}
			assertion.MustNotEmpty(t, sha)
			blob, _, err := ghClient.Git.GetBlobRaw(t.Context(), "f110", "gh-test", sha)
			assertion.MustNoError(t, err)
			assertion.Equal(t, "README", string(blob))
		})
	})

	t.Run("RepositoriesService", func(t *testing.T) {
		m := NewMock()
		m.Repository("f110/gh-test")

		ghClient := m.Client()

		t.Run("GetCommit", func(t *testing.T) {
			repoCommit, _, err := ghClient.Repositories.GetCommit(t.Context(), "f110", "gh-test", "HEAD", &github.ListOptions{})
			assertion.MustNoError(t, err)
			assertion.NotEmpty(t, repoCommit.GetSHA())
			assertion.NotEmpty(t, repoCommit.GetCommit().GetTree().GetSHA())
		})
	})
}
