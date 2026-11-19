package server

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/go-github/v85/github"
	"go.f110.dev/githubmock"

	"go.f110.dev/mono/go/codexel/api"
	"go.f110.dev/mono/go/jujutsu"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/testing/assertion"
)

var jjBinaryPath = flag.String("test.jj-binary", "", "")

func TestMain(m *testing.M) {
	logger.Init()
	m.Run()
}

func TestIsWholeStack(t *testing.T) {
	stack := []*jujutsu.StackCommit{{CommitID: "cccccccccccccccc"}, {CommitID: "bbbbbbbbbbbbbbbb"}, {CommitID: "aaaaaaaaaaaaaaaa"}}

	assertion.True(t, isWholeStack(stack, []string{"aaaaaaaa", "cccccccc", "bbbbbbbb"}))
	assertion.False(t, isWholeStack(stack, []string{"cccccccc", "bbbbbbbb"}))
	assertion.False(t, isWholeStack(stack, []string{"cccccccc", "bbbbbbbb", "dddddddd"}))
}

func TestListPullRequestTemplates(t *testing.T) {
	root := t.TempDir()
	assertion.MustNoError(t, os.MkdirAll(filepath.Join(root, ".github/PULL_REQUEST_TEMPLATE"), 0755))
	assertion.MustNoError(t, os.WriteFile(filepath.Join(root, ".github/PULL_REQUEST_TEMPLATE/bug.md"), nil, 0644))
	assertion.MustNoError(t, os.WriteFile(filepath.Join(root, ".github/PULL_REQUEST_TEMPLATE/feature.md"), nil, 0644))
	h := NewPullRequestHandler(newConfig(t, root))

	res, err := h.ListTemplates(t.Context(), connect.NewRequest(api.RequestListPullRequestTemplates_builder{Path: new(root)}.Build()))
	assertion.MustNoError(t, err)
	assertion.Equal(t, res.Msg.GetTemplates(), []string{".github/PULL_REQUEST_TEMPLATE/bug.md", ".github/PULL_REQUEST_TEMPLATE/feature.md"})
}

func TestSubmitPullRequest(t *testing.T) {
	if *jjBinaryPath == "" {
		t.Skipf("Skip %s because -test.jj-binary is not set", t.Name())
	}

	repo, commits := newStackRepository(t, "a", "b", "c")
	ghMock := githubmock.NewMock()
	ghMock.Repository("f110/mono")
	mux := http.NewServeMux()
	ghMock.RegisterHandler(mux)
	mux.HandleFunc("GET /repos/{owner}/{repo}/branches", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	})
	client := github.NewClient(&http.Client{Transport: handlerTransport{mux}})
	h := NewPullRequestHandler(newConfig(t, repo))
	h.newGitHubClient = func(context.Context) (*github.Client, error) { return client, nil }

	validate := func(revisions ...string) string {
		res, err := h.Validate(t.Context(), connect.NewRequest(api.RequestValidatePullRequest_builder{Path: new(repo), Revisions: revisions}.Build()))
		assertion.MustNoError(t, err)
		return res.Msg.GetReason()
	}
	assertion.Equal(t, validate(commits[1]), "")
	assertion.Equal(t, validate(commits[2], commits[1], commits[0]), "")
	assertion.Equal(t, validate(commits[2], commits[1]), "The revisions must be all commits between master@origin and the newest revision (3 commits)")

	_, err := h.Submit(t.Context(), connect.NewRequest(api.RequestSubmitPullRequest_builder{Path: new(repo), Revisions: []string{commits[2], commits[1]}}.Build()))
	assertion.MustError(t, err)
	connectErr, ok := errors.AsType[*connect.Error](err)
	assertion.True(t, ok)
	assertion.Equal(t, connectErr.Code(), connect.CodeFailedPrecondition)

	res, err := h.Submit(t.Context(), connect.NewRequest(api.RequestSubmitPullRequest_builder{Path: new(repo), Revisions: []string{commits[2], commits[1], commits[0]}}.Build()))
	assertion.MustNoError(t, err)
	assertion.MustLen(t, res.Msg.GetPushed(), 1)
	assertion.MustLen(t, res.Msg.GetPullRequests(), 1)
	assertion.Equal(t, res.Msg.GetPullRequests()[0].GetAction(), api.PullRequestAction_PULL_REQUEST_ACTION_CREATED)
	pr, _, err := client.PullRequests.Get(t.Context(), "f110", "mono", 1)
	assertion.MustNoError(t, err)
	assertion.Equal(t, pr.GetTitle(), "c: Add")
	assertion.Equal(t, pr.GetBase().GetRef(), "master")
	assertion.Equal(t, pr.GetHead().GetRef(), "push-"+res.Msg.GetPushed()[0][:12])
}

func TestSubmitPullRequestWithOnlyNoSend(t *testing.T) {
	if *jjBinaryPath == "" {
		t.Skipf("Skip %s because -test.jj-binary is not set", t.Name())
	}

	repo, commits := newStackRepository(t, "WIP", "no-send")
	ghMock := githubmock.NewMock()
	ghMock.Repository("f110/mono")
	mux := http.NewServeMux()
	ghMock.RegisterHandler(mux)
	mux.HandleFunc("GET /repos/{owner}/{repo}/branches", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	})
	client := github.NewClient(&http.Client{Transport: handlerTransport{mux}})
	h := NewPullRequestHandler(newConfig(t, repo))
	h.newGitHubClient = func(context.Context) (*github.Client, error) { return client, nil }

	const reason = `There are no commits to submit between master@origin and the revision. The commits whose description starts with "no-send:" or "WIP:" are excluded`
	for _, revisions := range [][]string{{commits[0]}, {commits[1], commits[0]}} {
		res, err := h.Validate(t.Context(), connect.NewRequest(api.RequestValidatePullRequest_builder{Path: new(repo), Revisions: revisions}.Build()))
		assertion.MustNoError(t, err)
		assertion.Equal(t, res.Msg.GetReason(), reason)

		_, err = h.Submit(t.Context(), connect.NewRequest(api.RequestSubmitPullRequest_builder{Path: new(repo), Revisions: revisions}.Build()))
		assertion.MustError(t, err)
		connectErr, ok := errors.AsType[*connect.Error](err)
		assertion.True(t, ok)
		assertion.Equal(t, connectErr.Code(), connect.CodeFailedPrecondition)
	}
	assertion.Len(t, ghMock.LookupRepository("f110/mono").GetPullRequests(), 0)
}

// newStackRepository returns the repository which has the commits of names on master@origin and the commit IDs of them
// from the oldest. origin is the local bare repository. The repository has the remote of GitHub which is not accessed
// to find the repository on GitHub.
func newStackRepository(t *testing.T, names ...string) (string, []string) {
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
	runCommand(t, repo, "jj", "git", "remote", "add", "github", "https://github.com/f110/mono.git")
	assertion.MustNoError(t, os.WriteFile(filepath.Join(repo, "README"), []byte("init"), 0644))
	runCommand(t, repo, "jj", "commit", "--message=init")
	runCommand(t, repo, "jj", "bookmark", "create", "master", "--revision=@-")
	runCommand(t, repo, "jj", "bookmark", "track", "master", "--remote=origin")
	runCommand(t, repo, "jj", "git", "push", "--bookmark=master")

	var commits []string
	for _, v := range names {
		assertion.MustNoError(t, os.WriteFile(filepath.Join(repo, v), []byte(v), 0644))
		runCommand(t, repo, "jj", "commit", "--message="+v+": Add")
		commits = append(commits, runCommand(t, repo, "jj", "log", "--no-graph", "--revisions=@-", "--template=commit_id.short()"))
	}
	return repo, commits
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
