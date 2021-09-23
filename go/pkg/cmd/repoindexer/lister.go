package repoindexer

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/v32/github"
	"github.com/shurcooL/githubv4"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/logger"
)

type Repository struct {
	Name             string
	URL              string
	DisableVendoring bool
	Refs             []plumbing.ReferenceName

	repo *git.Repository
}

type RepositoryListerOpt func(lister *RepositoryLister) error

type RepositoryLister struct {
	rules               []*Rule
	githubClient        *github.Client
	githubGraphQLClient *githubv4.Client

	mu           sync.Mutex
	repositories []*Repository
}

func GitHubApp(appId, installationId int64, privateKeyFile string) RepositoryListerOpt {
	return func(lister *RepositoryLister) error {
		restClient, err := newGitHubAppRESTClient(appId, installationId, privateKeyFile)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		lister.githubClient = restClient
		graphQLClient, err := newGitHubAppGraphQLClient(appId, installationId, privateKeyFile)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		lister.githubGraphQLClient = graphQLClient

		return nil
	}
}

func GitHubToken(token string) RepositoryListerOpt {
	return func(lister *RepositoryLister) error {
		lister.githubClient = newGitHubTokenRESTClient(token)
		lister.githubGraphQLClient = newGitHubTokenGraphQLClient(token)
		return nil
	}
}

func WithoutCredential() RepositoryListerOpt {
	return func(lister *RepositoryLister) error {
		lister.githubClient = github.NewClient(http.DefaultClient)
		lister.githubGraphQLClient = githubv4.NewClient(http.DefaultClient)
		return nil
	}
}

func NewRepositoryLister(rules []*Rule, opts ...RepositoryListerOpt) (*RepositoryLister, error) {
	lister := &RepositoryLister{rules: rules}
	for _, v := range opts {
		if err := v(lister); err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
	}
	return lister, nil
}

func (x *RepositoryLister) List() []*Repository {
	x.mu.Lock()
	if x.repositories != nil {
		defer x.mu.Unlock()
		return x.repositories
	}
	x.mu.Unlock()

	repos := make([]*Repository, 0)
	for _, rule := range x.rules {
		if rule.Owner != "" && rule.Name != "" {
			repo, _, err := x.githubClient.Repositories.Get(context.Background(), rule.Owner, rule.Name)
			if err != nil {
				logger.Log.Info("Repository is not found", zap.String("owner", rule.Owner), zap.String("name", rule.Name))
				continue
			}
			repos = append(repos, &Repository{
				Name:             fmt.Sprintf("%s/%s", rule.Owner, rule.Name),
				URL:              repo.GetCloneURL(),
				Refs:             x.refSpecs(rule.Branches, rule.Tags),
				DisableVendoring: rule.DisableVendoring,
			})
		}

		if rule.Query != "" {
			vars := map[string]interface{}{
				"searchQuery": githubv4.String(rule.Query),
				"cursor":      (*githubv4.String)(nil),
			}
			err := x.githubGraphQLClient.Query(context.Background(), &listRepositoriesQuery, vars)
			if err != nil {
				logger.Log.Info("Failed execute query", zap.Error(err))
				continue
			}
			for _, v := range listRepositoriesQuery.Search.Nodes {
				if v.Type != "Repository" {
					continue
				}
				if v.Repository.IsArchived {
					continue
				}
				repos = append(repos, &Repository{
					Name:             fmt.Sprintf("%s/%s", v.Repository.Owner.Login, v.Repository.Name),
					URL:              v.Repository.URL.String(),
					Refs:             x.refSpecs(rule.Branches, rule.Tags),
					DisableVendoring: rule.DisableVendoring,
				})
			}
			if !listRepositoriesQuery.Search.PageInfo.HasNextPage {
				break
			}
			vars["cursor"] = listRepositoriesQuery.Search.PageInfo.EndCursor
		}
	}
	if len(repos) == 0 {
		logger.Log.Warn("Not found any repository")
	}

	x.mu.Lock()
	x.repositories = repos
	x.mu.Unlock()
	return repos
}

func (x *RepositoryLister) ClearCache() {
	x.mu.Lock()
	x.repositories = nil
	x.mu.Unlock()
}

func (*RepositoryLister) refSpecs(branches, tags []string) []plumbing.ReferenceName {
	refs := make([]plumbing.ReferenceName, 0, len(branches)+len(tags))
	for _, v := range branches {
		refs = append(refs, plumbing.NewRemoteReferenceName("origin", v))
	}
	for _, v := range tags {
		refs = append(refs, plumbing.NewTagReferenceName(v))
	}

	return refs
}

func newGitHubAppRESTClient(appId, installationId int64, privateKeyFile string) (*github.Client, error) {
	tr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appId, installationId, privateKeyFile)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return github.NewClient(&http.Client{Transport: tr}), nil
}

func newGitHubTokenRESTClient(token string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)

	return github.NewClient(tc)
}

func newGitHubAppGraphQLClient(appId, installationId int64, privateKeyFile string) (*githubv4.Client, error) {
	tr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appId, installationId, privateKeyFile)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return githubv4.NewClient(&http.Client{Transport: tr}), nil
}

func newGitHubTokenGraphQLClient(token string) *githubv4.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)

	return githubv4.NewClient(tc)
}

var listRepositoriesQuery struct {
	Search struct {
		PageInfo struct {
			EndCursor   githubv4.String
			HasNextPage bool
		}
		Nodes []struct {
			Type       string           `graphql:"__typename"`
			Repository RepositorySchema `graphql:"... on Repository"`
		}
	} `graphql:"search(query: $searchQuery type: REPOSITORY first: 100 after: $cursor)"`
}

type RepositorySchema struct {
	Name  string
	Owner struct {
		Login string
	}
	URL        githubv4.URI
	IsArchived bool
}
