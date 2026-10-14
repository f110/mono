package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v85/github"

	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/testing/assertion"
)

func newFakeSearchServer(t *testing.T, created []time.Time, failuresOutOf3 int) *github.Client {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		if requests%3 < failuresOutOf3 {
			err := json.NewEncoder(w).Encode(&github.IssuesSearchResult{Total: new(0), IncompleteResults: new(false)})
			assertion.MustNoError(t, err)
			return
		}
		q := req.URL.Query().Get("q")
		var from, to time.Time
		for v := range strings.FieldsSeq(q) {
			r, ok := strings.CutPrefix(v, "created:")
			if !ok {
				continue
			}
			f, s, _ := strings.Cut(r, "..")
			var err error
			from, err = time.Parse(searchTimeFormat, f)
			assertion.MustNoError(t, err)
			to, err = time.Parse(searchTimeFormat, s)
			assertion.MustNoError(t, err)
		}

		var matched []*github.Issue
		for i, v := range created {
			if v.Before(from) || v.After(to) {
				continue
			}
			matched = append(matched, &github.Issue{Number: new(i + 1), RepositoryURL: new("https://api.github.com/repos/org/repo")})
		}
		page, _ := strconv.Atoi(req.URL.Query().Get("page"))
		if page == 0 {
			page = 1
		}
		perPage, _ := strconv.Atoi(req.URL.Query().Get("per_page"))
		start := (page - 1) * perPage
		end := min(start+perPage, len(matched), searchResultLimit)
		if end < min(len(matched), searchResultLimit) {
			u := *req.URL
			v := u.Query()
			v.Set("page", strconv.Itoa(page+1))
			u.RawQuery = v.Encode()
			w.Header().Set("Link", fmt.Sprintf(`<http://%s%s>; rel="next"`, req.Host, u.String()))
		}
		var items []*github.Issue
		if start < end {
			items = matched[start:end]
		}
		err := json.NewEncoder(w).Encode(&github.IssuesSearchResult{Total: new(len(matched)), IncompleteResults: new(false), Issues: items})
		assertion.MustNoError(t, err)
	}))
	t.Cleanup(srv.Close)

	client := github.NewClient(nil)
	u, err := url.Parse(srv.URL + "/")
	assertion.MustNoError(t, err)
	client.BaseURL = u
	return client
}

func TestSearchPullRequests(t *testing.T) {
	assertion.MustNoError(t, slogger.Init())
	searchRetryInterval = 0
	from := time.Date(2024, 10, 22, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 22, 23, 59, 59, 0, time.UTC)

	cases := []struct {
		Name           string
		Count          int
		FailuresOutOf3 int
	}{
		{Name: "Single page", Count: 50},
		{Name: "Multiple pages", Count: 350},
		{Name: "Exceed search limit", Count: 2500},
		{Name: "Flaky search API", Count: 2500, FailuresOutOf3: 1},
		{Name: "Very flaky search API", Count: 2500, FailuresOutOf3: 2},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			created := make([]time.Time, tc.Count)
			step := to.Sub(from) / time.Duration(tc.Count)
			for i := range created {
				created[i] = from.Add(step * time.Duration(i)).Truncate(time.Second)
			}
			client := newFakeSearchServer(t, created, tc.FailuresOutOf3)

			var issues []*github.Issue
			err := searchPullRequests(context.Background(), client, "me", "org", from, to, func(issue *github.Issue) error {
				issues = append(issues, issue)
				return nil
			})
			assertion.MustNoError(t, err)
			assertion.MustLen(t, issues, tc.Count)
			seen := make(map[int]struct{})
			for _, v := range issues {
				seen[v.GetNumber()] = struct{}{}
			}
			assertion.Len(t, seen, tc.Count)
		})
	}
}

func TestRepositoryFromURL(t *testing.T) {
	owner, repo, err := repositoryFromURL("https://api.github.com/repos/f110/mono")
	assertion.MustNoError(t, err)
	assertion.Equal(t, owner, "f110")
	assertion.Equal(t, repo, "mono")

	_, _, err = repositoryFromURL("https://api.github.com/users/f110")
	assertion.Error(t, err)
}

func TestIsUpToDate(t *testing.T) {
	updatedAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		Name      string
		Saved     *github.PullRequest
		UpdatedAt time.Time
		Expected  bool
	}{
		{Name: "Not dumped yet", UpdatedAt: updatedAt, Expected: false},
		{Name: "Closed and not updated", Saved: &github.PullRequest{State: new("closed"), UpdatedAt: &github.Timestamp{Time: updatedAt}}, UpdatedAt: updatedAt, Expected: true},
		{Name: "Closed and updated", Saved: &github.PullRequest{State: new("closed"), UpdatedAt: &github.Timestamp{Time: updatedAt}}, UpdatedAt: updatedAt.Add(time.Second), Expected: false},
		{Name: "Open and not updated", Saved: &github.PullRequest{State: new("open"), UpdatedAt: &github.Timestamp{Time: updatedAt}}, UpdatedAt: updatedAt, Expected: false},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.Saved != nil {
				assertion.MustNoError(t, writeJSON(filepath.Join(dir, "pr.json"), tc.Saved))
			}

			upToDate, err := isUpToDate(dir, tc.UpdatedAt)
			assertion.MustNoError(t, err)
			assertion.Equal(t, upToDate, tc.Expected)
		})
	}
}
