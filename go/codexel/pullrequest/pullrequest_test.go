package pullrequest

import (
	"path/filepath"
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestFile(t *testing.T) {
	f := NewFile(filepath.Join(t.TempDir(), "codexel", "pullrequests.json"))

	prs, err := f.List("/repo")
	assertion.MustNoError(t, err)
	assertion.Len(t, prs, 0)

	err = f.Save([]*PullRequest{
		{Repository: "/repo", ChangeID: "kkkk", URL: "https://github.com/f110/mono/pull/1"},
		{Repository: "/repo", ChangeID: "llll", URL: "https://github.com/f110/mono/pull/2"},
		{Repository: "/other", ChangeID: "kkkk", URL: "https://github.com/f110/other/pull/1"},
	})
	assertion.MustNoError(t, err)
	err = f.Save([]*PullRequest{{Repository: "/repo", ChangeID: "kkkk", URL: "https://github.com/f110/mono/pull/3"}})
	assertion.MustNoError(t, err)

	prs, err = f.List("/repo")
	assertion.MustNoError(t, err)
	assertion.Equal(t, prs, []*PullRequest{
		{Repository: "/repo", ChangeID: "llll", URL: "https://github.com/f110/mono/pull/2"},
		{Repository: "/repo", ChangeID: "kkkk", URL: "https://github.com/f110/mono/pull/3"},
	})
}
