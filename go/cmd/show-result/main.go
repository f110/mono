package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/shurcooL/githubv4"
	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	"golang.org/x/oauth2"
)

type PullRequest struct {
	Id     string
	URL    string
	Title  string
	Number int
	State  githubv4.PullRequestState
	Author struct {
		Login string
	}
	Repository struct {
		Name  string
		Owner struct {
			Login string
		}
	}
	ReviewRequests struct {
		Nodes []struct {
			RequestedReviewer struct {
				User struct {
					Login string
				} `graphql:"... on User"`
			}
		}
	} `graphql:"reviewRequests(first: 30)"`
	Reviews struct {
		Nodes []struct {
			Author struct {
				Login string
			}
			State githubv4.PullRequestReviewState
		}
	} `graphql:"reviews(first: 30)"`
	Comments struct {
		Nodes []struct {
			Author struct {
				Login string
			}
			Body string
		}
	} `graphql:"comments(first: 50)"`
}

type Issue struct {
	Id     string
	URL    string
	Title  string
	Number int
	Author struct {
		Login string
	}
	Repository struct {
		Name  string
		Owner struct {
			Login string
		}
	}
	State    githubv4.IssueState
	Comments struct {
		Nodes []struct {
			Author struct {
				Login string
			}
			Body string
		}
	} `graphql:"comments(first: 50)"`
	TimelineItems struct {
		Nodes []struct {
			Type                 string `graphql:"__typename"`
			CrossReferencedEvent struct {
				Source struct {
					Type        string      `graphql:"__typename"`
					PullRequest PullRequest `graphql:"... on PullRequest"`
				}
			} `graphql:"... on CrossReferencedEvent"`
		}
	} `graphql:"timelineItems(first: 20 itemTypes: CROSS_REFERENCED_EVENT)"`
}

type Node struct {
	Type        string      `graphql:"__typename"`
	PullRequest PullRequest `graphql:"... on PullRequest"`
	Issue       Issue       `graphql:"... on Issue"`
}

var query struct {
	Search struct {
		PageInfo struct {
			EndCursor   githubv4.String
			HasNextPage bool
		}
		Nodes      []Node
		IssueCount int
	} `graphql:"search(query: $searchQuery type: ISSUE first: 100 after: $cursor)"`
}

var meQuery struct {
	Viewer struct {
		Login string
	}
}

type Nodes []Node

type filterFunc func(n Node) bool

func (nodes Nodes) Filter(funcs ...filterFunc) Nodes {
	result := make([]Node, 0, len(nodes))
	for _, v := range nodes {
		ok := false
		for _, f := range funcs {
			if c := f(v); c {
				ok = true
				break
			}
		}

		if ok {
			result = append(result, v)
		}
	}

	return result
}

func excludeReviewOnly(username string) filterFunc {
	return func(n Node) bool {
		if n.Type != "PullRequest" {
			return true
		}

		if n.PullRequest.Author.Login == username {
			return true
		}

		for _, c := range n.PullRequest.Comments.Nodes {
			if c.Author.Login == username {
				return true
			}
		}

		return false
	}
}

func getIssueAndPullRequest(client *githubv4.Client, user, start, end, org string, max int) ([]Node, error) {
	updated := fmt.Sprintf(">=%s", start)
	if end != "" {
		updated = fmt.Sprintf("%s..%s", start, end)
	}
	q := fmt.Sprintf("involves:%s updated:%s is:closed", user, updated)
	if org != "" {
		q += " org:" + org
	}
	variables := map[string]interface{}{
		"searchQuery": githubv4.String(q),
		"cursor":      (*githubv4.String)(nil),
	}

	tickets := make([]Node, 0)
	page := 1
	for {
		if err := client.Query(context.Background(), &query, variables); err != nil {
			return nil, xerrors.WithStack(err)
		}
		// Is first page
		if len(tickets) == 0 {
			fmt.Fprintf(os.Stderr, "Found %d issues\n", query.Search.IssueCount)
		}
		if query.Search.IssueCount > 1000 {
			return nil, xerrors.New("result over than 1000. GitHub API can not fetch results over than 1000")
		}
		tickets = append(tickets, query.Search.Nodes...)

		fmt.Fprintf(os.Stderr, "Got %d page\n", page)
		if !query.Search.PageInfo.HasNextPage {
			fmt.Fprintln(os.Stderr, "Doesn't have next page")
			break
		}
		if max > 0 && len(tickets) > max {
			break
		}

		page++
		variables["cursor"] = githubv4.NewString(query.Search.PageInfo.EndCursor)
		fmt.Fprintf(os.Stderr, "Cursor: %s\n", query.Search.PageInfo.EndCursor)
	}

	return tickets, nil
}

func showResult() error {
	var start, end, org string
	max := -1
	fs := pflag.NewFlagSet("show-result", pflag.ContinueOnError)
	fs.StringVar(&start, "start", "", "")
	fs.StringVar(&end, "end", "", "")
	fs.IntVar(&max, "max", max, "")
	fs.StringVar(&org, "org", "", "(Optional) Organization name")
	if err := fs.Parse(os.Args); err != nil {
		return err
	}

	if os.Getenv("GITHUB_APITOKEN") == "" {
		return xerrors.New("GITHUB_APITOKEN is mandatory environment variable")
	}
	if start == "" {
		return xerrors.New("start is a mandatory argument")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_APITOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := githubv4.NewClient(tc)

	err := client.Query(context.Background(), &meQuery, nil)
	if err != nil {
		return xerrors.WithStack(err)
	}

	issues, err := getIssueAndPullRequest(client, meQuery.Viewer.Login, start, end, org, max)
	if err != nil {
		return xerrors.WithStack(err)
	}
	rawList := new(bytes.Buffer)
	for _, v := range issues {
		switch v.Type {
		case "Issue":
			fmt.Fprintf(rawList, "* [%s#%d - %s](%s)\n", v.Issue.Repository.Name, v.Issue.Number, v.Issue.Title, v.Issue.URL)
		case "PullRequest":
			fmt.Fprintf(rawList, "* [%s#%d - %s](%s)\n", v.PullRequest.Repository.Name, v.PullRequest.Number, v.PullRequest.Title, v.PullRequest.URL)
		}
	}
	issues = Nodes(issues).Filter(excludeReviewOnly(meQuery.Viewer.Login))

	marked := make(map[string]struct{})
	for _, v := range issues {
		if v.Type != "Issue" {
			continue
		}
		for _, r := range v.Issue.TimelineItems.Nodes {
			if r.Type != "CrossReferencedEvent" {
				continue
			}
			if r.CrossReferencedEvent.Source.Type != "PullRequest" {
				continue
			}

			marked[fmt.Sprintf("%s#%d", r.CrossReferencedEvent.Source.PullRequest.Repository.Name, r.CrossReferencedEvent.Source.PullRequest.Number)] = struct{}{}
		}
	}

	body := new(bytes.Buffer)
	for _, v := range issues {
		switch v.Type {
		case "PullRequest":
			if _, ok := marked[fmt.Sprintf("%s#%d", v.PullRequest.Repository.Name, v.PullRequest.Number)]; ok {
				continue
			}

			fmt.Fprintf(body, "* [%s#%d - %s](%s)\n", v.PullRequest.Repository.Name, v.PullRequest.Number, v.PullRequest.Title, v.PullRequest.URL)
		case "Issue":
			fmt.Fprintf(body, "* [%s#%s](%s)\n", v.Issue.Repository.Name, v.Issue.Title, v.Issue.URL)
			for _, r := range v.Issue.TimelineItems.Nodes {
				if r.Type != "CrossReferencedEvent" {
					continue
				}
				if r.CrossReferencedEvent.Source.Type != "PullRequest" {
					continue
				}
				fmt.Fprintf(body, "    * [%s#%d - %s](%s)\n",
					r.CrossReferencedEvent.Source.PullRequest.Repository.Name,
					r.CrossReferencedEvent.Source.PullRequest.Number,
					r.CrossReferencedEvent.Source.PullRequest.Title,
					r.CrossReferencedEvent.Source.PullRequest.URL,
				)
			}
		}
	}

	fmt.Println("# Raw")
	fmt.Print(rawList.String())
	fmt.Println()
	fmt.Println("# Organized")
	fmt.Print(body.String())

	return nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if err := showResult(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
