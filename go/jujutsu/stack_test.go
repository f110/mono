package jujutsu

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-github/v85/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.f110.dev/githubmock"

	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestMain(m *testing.M) {
	logger.Init()
	m.Run()
}

func TestFindPullRequestTemplate(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".github/PULL_REQUEST_TEMPLATE"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".github/PULL_REQUEST_TEMPLATE.md"), nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".github/PULL_REQUEST_TEMPLATE/change.md"), nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".github/CODEOWNERS"), nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "docs/pull_request_template.md"), nil, 0644))

	templates, err := PullRequestTemplates(dir)
	require.NoError(t, err)
	assert.Len(t, templates, 3)
}

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

		c := NewPullRequestSubmitter()
		c.GitHubClient = ghClient
		c.repositoryOwner, c.repositoryName = "f110", "mono"
		c.DefaultBranch = "master"
		c.RootDir = t.TempDir()
		c.stack = []*StackCommit{
			{
				ChangeID: "ylsnsuvootnpnwoxvokynlptorzkmxwy", CommitID: "b947bd3ba890e5252f1a151014f72ade7ca03a03", Bookmarks: []*StackBookmark{{Name: "push-ylsnsuvootnp"}},
				Description: `util: Fix

This PR fixes the bug.`,
			},
			{
				ChangeID: "ulplmwrqqxyxszouwwopptsttrlsnnsk", CommitID: "a505cb91edb706ac06c6fb6667adeb4502f6c346", Bookmarks: []*StackBookmark{{Name: "push-ulplmwrqqxyx"}},
				Description: `math: Add

This PR improves math package.`,
			},
			{
				ChangeID: "wlkxotovqzqnpvsowvwknyzwvqokqlko", CommitID: "a505cb91edb706ac06c6fb6667adeb4502f6c346", Bookmarks: []*StackBookmark{{Name: "push-wlkxotovqzqn"}},
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

	t.Run("StateCreatePRSkipsExistingPR", func(t *testing.T) {
		ghMock := githubmock.NewMock()
		repo := ghMock.Repository("f110/mono")
		repo.PullRequests(
			githubmock.NewPullRequest().
				Number(1).
				State(githubmock.PullRequestStateOpen).
				Base("master").
				Head(nil, "push-a", "").
				Title("a: Fix").
				Body("Body of A."),
		)
		ghClient := github.NewClient(&http.Client{Transport: ghMock.Transport()})

		c := NewPullRequestSubmitter()
		c.GitHubClient = ghClient
		c.repositoryOwner, c.repositoryName = "f110", "mono"
		c.DefaultBranch = "master"
		c.RootDir = t.TempDir()
		c.stack = []*StackCommit{
			{
				ChangeID:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Description: "b: Add\n\nBody of B.",
				Bookmarks:   []*StackBookmark{{Name: "push-b"}},
			},
			{
				ChangeID:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Description: "a: Fix\n\nBody of A.",
				Bookmarks:   []*StackBookmark{{Name: "push-a"}},
			},
		}

		nextState, err := c.createPR(context.Background())
		require.NoError(t, err)
		assert.Equal(t, c.stateUpdatePR, nextState)

		require.NotNil(t, c.stack[1].pullRequest)
		assert.Equal(t, 1, c.stack[1].pullRequest.ID)

		require.NotNil(t, c.stack[0].pullRequest)
		assert.Equal(t, 2, c.stack[0].pullRequest.ID)
		assert.Equal(t, "push-b", c.stack[0].pullRequest.Head)
		assert.Equal(t, "push-a", c.stack[0].pullRequest.Base)
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

		c := NewPullRequestSubmitter()
		c.GitHubClient = ghClient
		c.repositoryOwner, c.repositoryName = "f110", "mono"
		c.DefaultBranch = "master"
		c.stack = []*StackCommit{
			{
				ChangeID: "ylsnsuvootnpnwoxvokynlptorzkmxwy", CommitID: "b947bd3ba890e5252f1a151014f72ade7ca03a03", Bookmarks: []*StackBookmark{{Name: "push-ylsnsuvootnp"}},
				Description: `util: Fix

This PR fixes the bug.`,
				pullRequest: newPullRequest(pr3),
			},
			{
				ChangeID: "ulplmwrqqxyxszouwwopptsttrlsnnsk", CommitID: "a505cb91edb706ac06c6fb6667adeb4502f6c346", Bookmarks: []*StackBookmark{{Name: "push-ulplmwrqqxyx"}},
				Description: `math: Add

This PR improves math package.`,
				pullRequest: newPullRequest(pr2),
			},
			{
				ChangeID: "wlkxotovqzqnpvsowvwknyzwvqokqlko", CommitID: "a505cb91edb706ac06c6fb6667adeb4502f6c346", Bookmarks: []*StackBookmark{{Name: "push-wlkxotovqzqn"}},
				Description: `crypto: Fix security issue

This PR contains fixing some security issues.`,
				pullRequest: newPullRequest(pr1),
			},
		}

		nextState, err := c.updatePR(context.Background())
		require.NoError(t, err)
		assert.Equal(t, c.stateSyncStack, nextState)
	})
}

func TestJujutsuPRSubmitCommand_SyncStack(t *testing.T) {
	// stackFromBottom builds c.stack from an ordered list of pull request
	// numbers given from the bottom (closest to the trunk) to the top. c.stack
	// itself is stored from the top to the bottom.
	stackFromBottom := func(prNumbers ...int) []*StackCommit {
		stack := make([]*StackCommit, len(prNumbers))
		for i, n := range prNumbers {
			stack[len(prNumbers)-1-i] = &StackCommit{pullRequest: &pullRequest{ID: n}}
		}
		return stack
	}
	// stackPRNumbers returns the pull request numbers of a remote stack.
	stackPRNumbers := func(s *githubmock.Stack) []int {
		prs := s.GetPullRequests()
		out := make([]int, len(prs))
		for i, pr := range prs {
			out[i] = pr.GetNumber()
		}
		return out
	}

	t.Run("AppendOntoExistingStack", func(t *testing.T) {
		// A part of the local stack (#1, #2) is already stacked on GitHub, and
		// #3 is newly stacked on top of them.
		ghMock := githubmock.NewMock()
		repo := ghMock.Repository("f110/mono")
		pr1 := githubmock.NewPullRequest().Number(1)
		pr2 := githubmock.NewPullRequest().Number(2)
		pr3 := githubmock.NewPullRequest().Number(3)
		repo.PullRequests(pr1, pr2, pr3)
		repo.Stacks(githubmock.NewStack().Number(1).Base("master").PullRequests(pr1, pr2))
		ghClient := github.NewClient(&http.Client{Transport: ghMock.Transport()})

		c := NewPullRequestSubmitter()
		c.GitHubClient = ghClient
		c.repositoryOwner, c.repositoryName = "f110", "mono"
		c.DefaultBranch = "master"
		c.stack = stackFromBottom(1, 2, 3)

		nextState, err := c.syncStack(context.Background())
		require.NoError(t, err)
		assert.Equal(t, c.stateClose, nextState)

		stacks := repo.GetStacks()
		require.Len(t, stacks, 1)
		assert.Equal(t, []int{1, 2, 3}, stackPRNumbers(stacks[0]))
	})

	t.Run("ReorderSubmittedStack", func(t *testing.T) {
		// The whole stack (#1, #2, #3) is already stacked on GitHub, and the
		// order is changed locally to #1, #3, #2. The stack has to be dissolved
		// and recreated because the Stacks API cannot express a reorder.
		ghMock := githubmock.NewMock()
		repo := ghMock.Repository("f110/mono")
		pr1 := githubmock.NewPullRequest().Number(1)
		pr2 := githubmock.NewPullRequest().Number(2)
		pr3 := githubmock.NewPullRequest().Number(3)
		repo.PullRequests(pr1, pr2, pr3)
		repo.Stacks(githubmock.NewStack().Number(1).Base("master").PullRequests(pr1, pr2, pr3))
		ghClient := github.NewClient(&http.Client{Transport: ghMock.Transport()})

		c := NewPullRequestSubmitter()
		c.GitHubClient = ghClient
		c.repositoryOwner, c.repositoryName = "f110", "mono"
		c.DefaultBranch = "master"
		c.stack = stackFromBottom(1, 3, 2)

		nextState, err := c.syncStack(context.Background())
		require.NoError(t, err)
		assert.Equal(t, c.stateClose, nextState)

		stacks := repo.GetStacks()
		require.Len(t, stacks, 1)
		assert.Equal(t, []int{1, 3, 2}, stackPRNumbers(stacks[0]))
	})
}

func TestJujutsuPRSubmitCommand_UpdatePRDoesNotWriteChain(t *testing.T) {
	// The pull request chain is now provided by the Stacks API, so updatePR must
	// not write it into the description.
	ghMock := githubmock.NewMock()
	repo := ghMock.Repository("f110/mono")
	repo.PullRequests(
		githubmock.NewPullRequest().
			Number(1).
			Base("master").
			Head(nil, "push-a", "").
			Title("a: Fix").
			Body("Body of A."),
		githubmock.NewPullRequest().
			Number(2).
			Base("push-a").
			Head(nil, "push-b", "").
			Title("b: Add").
			Body("Body of B."),
	)
	ghClient := github.NewClient(&http.Client{Transport: ghMock.Transport()})

	pr1, _, err := ghClient.PullRequests.Get(t.Context(), "f110", "mono", 1)
	require.NoError(t, err)
	pr2, _, err := ghClient.PullRequests.Get(t.Context(), "f110", "mono", 2)
	require.NoError(t, err)

	c := NewPullRequestSubmitter()
	c.GitHubClient = ghClient
	c.repositoryOwner, c.repositoryName = "f110", "mono"
	c.DefaultBranch = "master"
	c.stack = []*StackCommit{
		{
			Description: "b: Add\n\nBody of B.",
			Bookmarks:   []*StackBookmark{{Name: "push-b"}},
			pullRequest: newPullRequest(pr2),
		},
		{
			Description: "a: Fix\n\nBody of A.",
			Bookmarks:   []*StackBookmark{{Name: "push-a"}},
			pullRequest: newPullRequest(pr1),
		},
	}

	_, err = c.updatePR(context.Background())
	require.NoError(t, err)

	got1, _, err := ghClient.PullRequests.Get(t.Context(), "f110", "mono", 1)
	require.NoError(t, err)
	got2, _, err := ghClient.PullRequests.Get(t.Context(), "f110", "mono", 2)
	require.NoError(t, err)
	assert.Equal(t, "Body of A.", got1.GetBody())
	assert.Equal(t, "Body of B.", got2.GetBody())
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

func TestPullRequestSubmitterSubmit(t *testing.T) {
	if *jjBinaryPath == "" {
		t.Skipf("Skip %s because -test.jj-binary is not set", t.Name())
	}

	t.Run("Stack", func(t *testing.T) {
		repo, remote, changes := newStackRepository(t)
		ghMock, ghClient := newGitHubMock(t)

		c := NewPullRequestSubmitter()
		c.Dir, c.Repository, c.DefaultBranch, c.Revision = repo, "f110/mono", "master", changes[1]
		c.GitHubClient = ghClient
		res, err := c.Submit(t.Context())
		assertion.MustNoError(t, err)

		assertion.Equal(t, res.Pushed, []string{changes[1], changes[0]})
		assertion.MustLen(t, res.PullRequests, 2)
		assertion.Equal(t, res.PullRequests[0].ChangeID, changes[0])
		assertion.Equal(t, res.PullRequests[0].Action, PullRequestCreated)
		assertion.Equal(t, res.PullRequests[1].ChangeID, changes[1])
		assertion.Equal(t, res.PullRequests[1].Action, PullRequestCreated)

		assertion.MustLen(t, ghMock.LookupRepository("f110/mono").GetPullRequests(), 2)
		pr1, pr2 := getPullRequest(t, ghClient, 1), getPullRequest(t, ghClient, 2)
		assertion.Equal(t, pr1.GetTitle(), "a: Add")
		assertion.Equal(t, pr1.GetHead().GetRef(), "push-"+changes[0][:12])
		assertion.Equal(t, pr1.GetBase().GetRef(), "master")
		assertion.Equal(t, pr2.GetTitle(), "b: Add")
		assertion.Equal(t, pr2.GetBody(), "Body of B.\n")
		assertion.Equal(t, pr2.GetHead().GetRef(), "push-"+changes[1][:12])
		assertion.Equal(t, pr2.GetBase().GetRef(), "push-"+changes[0][:12])
		branches := []string{"master", "push-" + changes[0][:12], "push-" + changes[1][:12]}
		slices.Sort(branches)
		assertion.Equal(t, remoteBranches(t, remote), branches)
	})

	t.Run("SinglePR", func(t *testing.T) {
		repo, remote, changes := newStackRepository(t)
		ghMock, ghClient := newGitHubMock(t)

		c := NewPullRequestSubmitter()
		c.Dir, c.Repository, c.DefaultBranch, c.Revision, c.SinglePR = repo, "f110/mono", "master", changes[1], true
		c.GitHubClient = ghClient
		res, err := c.Submit(t.Context())
		assertion.MustNoError(t, err)

		assertion.Equal(t, res.Pushed, []string{changes[1]})
		assertion.MustLen(t, res.PullRequests, 1)
		assertion.Equal(t, res.PullRequests[0].ChangeID, changes[1])

		assertion.MustLen(t, ghMock.LookupRepository("f110/mono").GetPullRequests(), 1)
		pr := getPullRequest(t, ghClient, 1)
		assertion.Equal(t, pr.GetHead().GetRef(), "push-"+changes[1][:12])
		assertion.Equal(t, pr.GetBase().GetRef(), "master")
		assertion.Equal(t, remoteBranches(t, remote), []string{"master", "push-" + changes[1][:12]})
	})

	t.Run("SelectTemplate", func(t *testing.T) {
		repo, _, changes := newStackRepository(t)
		assertion.MustNoError(t, os.MkdirAll(filepath.Join(repo, ".github/PULL_REQUEST_TEMPLATE"), 0755))
		assertion.MustNoError(t, os.WriteFile(filepath.Join(repo, ".github/PULL_REQUEST_TEMPLATE/bug.md"), []byte("bug template"), 0644))
		assertion.MustNoError(t, os.WriteFile(filepath.Join(repo, ".github/PULL_REQUEST_TEMPLATE/feature.md"), []byte("feature template"), 0644))
		ghMock, ghClient := newGitHubMock(t)

		var candidates []string
		c := NewPullRequestSubmitter()
		c.Dir, c.Repository, c.DefaultBranch, c.Revision, c.SinglePR = repo, "f110/mono", "master", changes[1], true
		c.GitHubClient = ghClient
		c.SelectTemplate = func(templates []string) (string, error) {
			candidates = templates
			return templates[1], nil
		}
		_, err := c.Submit(t.Context())
		assertion.MustNoError(t, err)

		assertion.Equal(t, candidates, []string{
			filepath.Join(repo, ".github/PULL_REQUEST_TEMPLATE/bug.md"),
			filepath.Join(repo, ".github/PULL_REQUEST_TEMPLATE/feature.md"),
		})
		assertion.MustLen(t, ghMock.LookupRepository("f110/mono").GetPullRequests(), 1)
		assertion.Equal(t, getPullRequest(t, ghClient, 1).GetBody(), "Body of B.\nfeature template")
		last, err := os.ReadFile(filepath.Join(repo, ".jj", lastPickedTemplateFile))
		assertion.MustNoError(t, err)
		assertion.Equal(t, string(last), "/.github/PULL_REQUEST_TEMPLATE/feature.md")
	})
}

// newStackRepository returns the repository which has two commits on master@origin, the bare repository of origin and
// the change IDs of the commits from the oldest.
func newStackRepository(t *testing.T) (string, string, []string) {
	t.Helper()

	jjBinary, err := filepath.Abs(*jjBinaryPath)
	assertion.MustNoError(t, err)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	assertion.MustNoError(t, err)
	t.Setenv("PATH", filepath.Dir(jjBinary)+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HOME", dir)
	t.Setenv("JJ_CONFIG", filepath.Join(dir, "config.toml"))
	assertion.MustNoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[user]\nname = \"test\"\nemail = \"test@example.com\"\n"), 0644))

	remote := filepath.Join(dir, "remote.git")
	repo := filepath.Join(dir, "repo")
	runCommand(t, dir, "git", "init", "--quiet", "--bare", "--initial-branch=master", remote)
	runCommand(t, dir, "jj", "git", "init", repo)
	runCommand(t, repo, "jj", "git", "remote", "add", "origin", remote)
	assertion.MustNoError(t, os.WriteFile(filepath.Join(repo, "README"), []byte("init"), 0644))
	runCommand(t, repo, "jj", "commit", "--message=init")
	runCommand(t, repo, "jj", "bookmark", "create", "master", "--revision=@-")
	runCommand(t, repo, "jj", "bookmark", "track", "master", "--remote=origin")
	runCommand(t, repo, "jj", "git", "push", "--bookmark=master")

	var changes []string
	for _, v := range []string{"a", "b"} {
		assertion.MustNoError(t, os.WriteFile(filepath.Join(repo, v), []byte(v), 0644))
		runCommand(t, repo, "jj", "commit", fmt.Sprintf("--message=%s: Add\n\nBody of %s.\n\nRefs: #1\n", v, strings.ToUpper(v)))
		changes = append(changes, strings.TrimSpace(runCommand(t, repo, "jj", "log", "--no-graph", "--revisions=@-", "--template=change_id")))
	}
	return repo, remote, changes
}

// newGitHubMock returns the mock of GitHub which has f110/mono. The mock doesn't support listing the branches, so it
// returns no branches.
func newGitHubMock(t *testing.T) (*githubmock.Mock, *github.Client) {
	t.Helper()

	ghMock := githubmock.NewMock()
	ghMock.Repository("f110/mono")
	mux := http.NewServeMux()
	ghMock.RegisterHandler(mux)
	mux.HandleFunc("GET /repos/{owner}/{repo}/branches", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	})
	return ghMock, github.NewClient(&http.Client{Transport: handlerTransport{mux}})
}

func remoteBranches(t *testing.T, remote string) []string {
	t.Helper()

	out := runCommand(t, remote, "git", "for-each-ref", "--format=%(refname:short)", "refs/heads")
	return strings.Fields(out)
}

func runCommand(t *testing.T, dir, name string, args ...string) string {
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out)
}

type handlerTransport struct {
	http.Handler
}

func (h handlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result(), nil
}

func getPullRequest(t *testing.T, client *github.Client, number int) *github.PullRequest {
	t.Helper()

	pr, _, err := client.PullRequests.Get(t.Context(), "f110", "mono", number)
	assertion.MustNoError(t, err)
	return pr
}
