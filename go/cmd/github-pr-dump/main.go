package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
	githubutil "go.f110.dev/mono/go/github"
	"go.f110.dev/mono/go/logger/slogger"
)

const (
	searchResultLimit = 1000
	searchPerPage     = 100
	searchMaxAttempts = 10
	searchTimeFormat  = "2006-01-02T15:04:05Z"
	dateFormat        = "2006-01-02"
)

var (
	searchRetryInterval    = 3 * time.Second
	searchMaxRetryInterval = time.Minute
)

type githubPRDumpCommand struct {
	Org    string
	Author string
	Since  string
	Until  string
	OutDir string

	githubClientFactory *githubutil.GitHubClientFactory
}

func newGitHubPRDumpCommand() *githubPRDumpCommand {
	return &githubPRDumpCommand{
		githubClientFactory: githubutil.NewGitHubClientFactory("github-pr-dump", true),
	}
}

func (c *githubPRDumpCommand) Flags(fs *cli.FlagSet) {
	fs.String("org", "Organization name").Var(&c.Org).Required()
	fs.String("author", "Author of pull requests. Default is the authenticated user").Var(&c.Author)
	fs.String("since", "Start date (YYYY-MM-DD). Default is 2 years ago").Var(&c.Since)
	fs.String("until", "End date (YYYY-MM-DD). Default is today").Var(&c.Until)
	fs.String("out", "Output directory").Var(&c.OutDir).Required()
	c.githubClientFactory.Flags(fs)
}

func (c *githubPRDumpCommand) Run(ctx context.Context) error {
	now := time.Now().UTC()
	since, err := parseDate(c.Since, now.AddDate(-2, 0, 0))
	if err != nil {
		return err
	}
	until, err := parseDate(c.Until, now)
	if err != nil {
		return err
	}
	until = until.Add(24*time.Hour - time.Second)

	if err := c.githubClientFactory.Init(); err != nil {
		return err
	}
	client := c.githubClientFactory.REST
	ctx = context.WithValue(ctx, github.SleepUntilPrimaryRateLimitResetWhenRateLimited, true)

	author := c.Author
	if author == "" {
		user, _, err := client.Users.Get(ctx, "")
		if err != nil {
			return xerrors.WithStack(err)
		}
		author = user.GetLogin()
	}

	var done, failed int
	err = searchPullRequests(ctx, client, author, c.Org, since, until, func(issue *github.Issue) error {
		owner, repo, err := repositoryFromURL(issue.GetRepositoryURL())
		if err != nil {
			return err
		}
		number := issue.GetNumber()
		done++
		log := slogger.Log.With(slog.String("repo", owner+"/"+repo), slog.Int("number", number), slog.Int("progress", done))
		dir := filepath.Join(c.OutDir, owner, repo, strconv.Itoa(number))
		upToDate, err := isUpToDate(dir, issue.GetUpdatedAt().Time)
		if err != nil {
			return err
		}
		if upToDate {
			log.Debug("Skip the pull request because it is up to date")
			return nil
		}
		err = retryOnSecondaryRateLimit(ctx, func() error {
			return dumpPullRequest(ctx, client, dir, owner, repo, number)
		})
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Error("Failed to dump pull request", slogger.E(err))
			failed++
			return nil
		}
		log.Info("Dumped pull request")
		return nil
	})
	if err != nil {
		return err
	}
	if failed > 0 {
		return xerrors.Definef("failed to dump %d pull requests. Re-run to retry", failed).WithStack()
	}

	return nil
}

func parseDate(s string, defaultValue time.Time) (time.Time, error) {
	if s == "" {
		return defaultValue.Truncate(24 * time.Hour), nil
	}
	t, err := time.Parse(dateFormat, s)
	if err != nil {
		return time.Time{}, xerrors.WithStack(err)
	}
	return t, nil
}

func searchQuery(author, org string, from, to time.Time) string {
	return fmt.Sprintf("is:pr author:%s org:%s created:%s..%s", author, org, from.UTC().Format(searchTimeFormat), to.UTC().Format(searchTimeFormat))
}

// searchPullRequests calls fn for each pull request created by the author in the range [from, to].
// The search API often returns empty results even if incomplete_results is false.
// Therefore, the total count of the split ranges is verified by the sum and the number of items in each page is verified by the total count.
func searchPullRequests(ctx context.Context, client *github.Client, author, org string, from, to time.Time, fn func(*github.Issue) error) error {
	total, err := countPullRequests(ctx, client, author, org, from, to)
	if err != nil {
		return err
	}
	slogger.Log.Info("Found pull requests", slog.String("author", author), slog.Int("total", total))

	return walkPullRequests(ctx, client, author, org, from, to, total, fn)
}

// countPullRequests returns the number of pull requests in the range [from, to].
// The search API often returns 0 by mistake, so 0 is accepted only if the search API keeps returning 0.
func countPullRequests(ctx context.Context, client *github.Client, author, org string, from, to time.Time) (int, error) {
	q := searchQuery(author, org, from, to)
	var total int
	_, err := retrySearch(ctx, q, func() (bool, error) {
		res, err := searchIssues(ctx, client, q, 1, 1)
		if err != nil {
			return false, err
		}
		total = res.GetTotal()
		return !res.GetIncompleteResults() && total > 0, nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}

func walkPullRequests(ctx context.Context, client *github.Client, author, org string, from, to time.Time, total int, fn func(*github.Issue) error) error {
	if total == 0 {
		return nil
	}

	q := searchQuery(author, org, from, to)
	if total > searchResultLimit {
		if to.Sub(from) <= time.Second {
			return xerrors.Definef("too many pull requests at %s", from.Format(searchTimeFormat)).WithStack()
		}
		mid := from.Add(to.Sub(from) / 2).Truncate(time.Second)
		var first, second int
		ok, err := retrySearch(ctx, q, func() (bool, error) {
			var err error
			first, err = countPullRequests(ctx, client, author, org, from, mid)
			if err != nil {
				return false, err
			}
			second, err = countPullRequests(ctx, client, author, org, mid.Add(time.Second), to)
			if err != nil {
				return false, err
			}
			return first+second == total, nil
		})
		if err != nil {
			return err
		}
		if !ok {
			return xerrors.Definef("the sum of the split ranges does not match the total count %d: %s", total, q).WithStack()
		}
		if err := walkPullRequests(ctx, client, author, org, from, mid, first, fn); err != nil {
			return err
		}
		return walkPullRequests(ctx, client, author, org, mid.Add(time.Second), to, second, fn)
	}

	for page := 1; (page-1)*searchPerPage < total; page++ {
		expected := min(searchPerPage, total-(page-1)*searchPerPage)
		var res *github.IssuesSearchResult
		ok, err := retrySearch(ctx, q, func() (bool, error) {
			var err error
			res, err = searchIssues(ctx, client, q, page, searchPerPage)
			if err != nil {
				return false, err
			}
			return !res.GetIncompleteResults() && res.GetTotal() == total && len(res.Issues) == expected, nil
		})
		if err != nil {
			return err
		}
		if !ok {
			return xerrors.Definef("page %d does not have %d pull requests: %s", page, expected, q).WithStack()
		}
		for _, v := range res.Issues {
			if err := fn(v); err != nil {
				return err
			}
		}
	}
	return nil
}

func searchIssues(ctx context.Context, client *github.Client, q string, page, perPage int) (*github.IssuesSearchResult, error) {
	opts := &github.SearchOptions{Sort: "created", Order: "asc", ListOptions: github.ListOptions{Page: page, PerPage: perPage}}
	var res *github.IssuesSearchResult
	err := retryOnSecondaryRateLimit(ctx, func() error {
		var err error
		res, _, err = client.Search.Issues(ctx, q, opts)
		if err != nil {
			return xerrors.WithStack(err)
		}
		return nil
	})
	return res, err
}

// retrySearch calls fn until fn returns true with exponential backoff.
// It returns false if fn doesn't return true in searchMaxAttempts attempts.
func retrySearch(ctx context.Context, q string, fn func() (bool, error)) (bool, error) {
	interval := searchRetryInterval
	for attempt := range searchMaxAttempts {
		if attempt > 0 {
			slogger.Log.Info("Search results are inconsistent. Retrying", slog.String("query", q), slog.Int("attempt", attempt), slog.Duration("wait", interval))
			if err := sleep(ctx, interval); err != nil {
				return false, err
			}
			interval = min(interval*2, searchMaxRetryInterval)
		}
		ok, err := fn()
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func repositoryFromURL(s string) (string, string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", "", xerrors.WithStack(err)
	}
	s = strings.TrimPrefix(u.Path, "/api/v3")
	owner, repo, ok := strings.Cut(strings.TrimPrefix(s, "/repos/"), "/")
	if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return "", "", xerrors.Definef("unexpected repository url: %s", s).WithStack()
	}
	return owner, repo, nil
}

// isUpToDate reports whether the pull request saved in dir doesn't need to be dumped again.
// An open pull request is always dumped again in case updated_at doesn't reflect some changes.
func isUpToDate(dir string, updatedAt time.Time) (bool, error) {
	b, err := os.ReadFile(filepath.Join(dir, "pr.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, xerrors.WithStack(err)
	}
	var pr github.PullRequest
	if err := json.Unmarshal(b, &pr); err != nil {
		return false, xerrors.WithStack(err)
	}
	if pr.GetState() == "open" {
		return false, nil
	}
	return !updatedAt.After(pr.GetUpdatedAt().Time), nil
}

// dumpPullRequest writes the pull request, its comments and its diff under dir.
// pr.json is written last so that its existence means the pull request has been dumped completely.
func dumpPullRequest(ctx context.Context, client *github.Client, dir, owner, repo string, number int) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return xerrors.WithStack(err)
	}

	pr, _, err := client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return xerrors.WithStack(err)
	}
	issueComments, err := collect(client.Issues.ListCommentsIter(ctx, owner, repo, number, &github.IssueListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100}}))
	if err != nil {
		return err
	}
	reviews, err := collect(client.PullRequests.ListReviewsIter(ctx, owner, repo, number, &github.ListOptions{PerPage: 100}))
	if err != nil {
		return err
	}
	reviewComments, err := collect(client.PullRequests.ListCommentsIter(ctx, owner, repo, number, &github.PullRequestListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100}}))
	if err != nil {
		return err
	}

	diff, _, err := client.PullRequests.GetRaw(ctx, owner, repo, number, github.RawOptions{Type: github.Diff})
	var errResp *github.ErrorResponse
	switch {
	case err == nil:
		if err := os.WriteFile(filepath.Join(dir, "pr.diff"), []byte(diff), 0644); err != nil {
			return xerrors.WithStack(err)
		}
		if err := removeIfExists(filepath.Join(dir, "files.json")); err != nil {
			return err
		}
	case errors.As(err, &errResp) && errResp.Response.StatusCode == http.StatusNotAcceptable:
		// The diff is too large to get as a raw format. Save the patches per file instead.
		files, err := collect(client.PullRequests.ListFilesIter(ctx, owner, repo, number, &github.ListOptions{PerPage: 100}))
		if err != nil {
			return err
		}
		if err := writeJSON(filepath.Join(dir, "files.json"), files); err != nil {
			return err
		}
		if err := removeIfExists(filepath.Join(dir, "pr.diff")); err != nil {
			return err
		}
	default:
		return xerrors.WithStack(err)
	}

	if err := writeJSON(filepath.Join(dir, "issue_comments.json"), issueComments); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(dir, "reviews.json"), reviews); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(dir, "review_comments.json"), reviewComments); err != nil {
		return err
	}
	return writeJSON(filepath.Join(dir, "pr.json"), pr)
}

func collect[T any](seq func(yield func(T, error) bool)) ([]T, error) {
	result := make([]T, 0)
	for v, err := range seq {
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		result = append(result, v)
	}
	return result, nil
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return xerrors.WithStack(err)
	}
	return nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return xerrors.WithStack(err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func retryOnSecondaryRateLimit(ctx context.Context, fn func() error) error {
	for {
		err := fn()
		var abuseErr *github.AbuseRateLimitError
		if !errors.As(err, &abuseErr) {
			return err
		}
		wait := abuseErr.GetRetryAfter()
		if wait == 0 {
			wait = time.Minute
		}
		slogger.Log.Info("Hit the secondary rate limit", slog.Duration("wait", wait))
		if err := sleep(ctx, wait); err != nil {
			return err
		}
	}
}

func main() {
	c := newGitHubPRDumpCommand()
	cmd := &cli.Command{
		Use:   "github-pr-dump",
		Short: "Dump pull requests created by the author in the organization",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return c.Run(ctx)
		},
	}
	c.Flags(cmd.Flags())

	if err := cmd.Execute(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
