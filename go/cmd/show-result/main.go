package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/shurcooL/githubv4"
	"github.com/spf13/pflag"
	"golang.org/x/oauth2"
	"golang.org/x/xerrors"
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
		Nodes []Node
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

func getIssueAndPullRequest(client *githubv4.Client, user, start, org string, max int) ([]Node, error) {
	q := fmt.Sprintf("involves:%s updated:>=%s is:closed", user, start)
	if org != "" {
		q += " org:" + org
	}
	variables := map[string]interface{}{
		"searchQuery": githubv4.String(q),
		"cursor":      (*githubv4.String)(nil),
	}

	tickets := make([]Node, 0)
	for {
		if err := client.Query(context.Background(), &query, variables); err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		tickets = append(tickets, query.Search.Nodes...)

		if !query.Search.PageInfo.HasNextPage {
			break
		}
		if max > 0 && len(tickets) > max {
			break
		}

		variables["cursor"] = githubv4.NewString(query.Search.PageInfo.EndCursor)
	}

	return tickets, nil
}

func showResult() error {
	var start, org string
	max := -1
	fs := pflag.NewFlagSet("show-result", pflag.ContinueOnError)
	fs.StringVar(&start, "start", "", "")
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
		return xerrors.Errorf(": %w", err)
	}

	issues, err := getIssueAndPullRequest(client, meQuery.Viewer.Login, start, org, max)
	if err != nil {
		return xerrors.Errorf(": %w", err)
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

	for _, v := range issues {
		switch v.Type {
		case "PullRequest":
			if _, ok := marked[fmt.Sprintf("%s#%d", v.PullRequest.Repository.Name, v.PullRequest.Number)]; ok {
				continue
			}

			fmt.Printf("[%s#%d - %s](%s)\n", v.PullRequest.Repository.Name, v.PullRequest.Number, v.PullRequest.Title, v.PullRequest.URL)
		case "Issue":
			fmt.Printf("[%s#%s](%s)\n", v.Issue.Repository.Name, v.Issue.Title, v.Issue.URL)
			for _, r := range v.Issue.TimelineItems.Nodes {
				if r.Type != "CrossReferencedEvent" {
					continue
				}
				if r.CrossReferencedEvent.Source.Type != "PullRequest" {
					continue
				}
				fmt.Printf("\t[%s#%d - %s](%s)\n",
					r.CrossReferencedEvent.Source.PullRequest.Repository.Name,
					r.CrossReferencedEvent.Source.PullRequest.Number,
					r.CrossReferencedEvent.Source.PullRequest.Title,
					r.CrossReferencedEvent.Source.PullRequest.URL,
				)
			}
		}
	}

	return nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if err := showResult(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
