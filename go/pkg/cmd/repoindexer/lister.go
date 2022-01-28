package repoindexer

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
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

type replaceRule struct {
	re      *regexp.Regexp
	replace string
}

type RepositoryLister struct {
	rules               []*Rule
	githubClient        *github.Client
	githubGraphQLClient *githubv4.Client

	mu           sync.Mutex
	repositories []*Repository
}

func GitHubApp(appId, installationId int64, privateKeyFile string, customRESTEndpoint, customGraphQLEndpoint string) RepositoryListerOpt {
	return func(lister *RepositoryLister) error {
		restClient, err := newGitHubAppRESTClient(appId, installationId, privateKeyFile, customRESTEndpoint)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		lister.githubClient = restClient
		graphQLClient, err := newGitHubAppGraphQLClient(appId, installationId, privateKeyFile, customGraphQLEndpoint)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		lister.githubGraphQLClient = graphQLClient

		return nil
	}
}

func GitHubToken(token string, customRESTEndpoint, customGraphQLEndpoint string) RepositoryListerOpt {
	return func(lister *RepositoryLister) error {
		var err error
		lister.githubClient, err = newGitHubTokenRESTClient(token, customRESTEndpoint)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		lister.githubGraphQLClient = newGitHubTokenGraphQLClient(token, customGraphQLEndpoint)
		return nil
	}
}

func WithoutCredential(customRESTEndpoint, customGraphQLEndpoint string) RepositoryListerOpt {
	return func(lister *RepositoryLister) error {
		lister.githubClient = github.NewClient(http.DefaultClient)
		if customRESTEndpoint != "" {
			u, err := url.Parse(customRESTEndpoint)
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			lister.githubClient.BaseURL = u
		}
		if customGraphQLEndpoint != "" {
			lister.githubGraphQLClient = githubv4.NewEnterpriseClient(customGraphQLEndpoint, http.DefaultClient)
		} else {
			lister.githubGraphQLClient = githubv4.NewClient(http.DefaultClient)
		}
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

func (x *RepositoryLister) List(ctx context.Context) []*Repository {
	x.mu.Lock()
	if x.repositories != nil {
		defer x.mu.Unlock()
		return x.repositories
	}
	x.mu.Unlock()

	repos := make(map[string]*Repository, 0)
	for _, rule := range x.rules {
		if rule.Owner != "" && rule.Name != "" {
			repo, _, err := x.githubClient.Repositories.Get(ctx, rule.Owner, rule.Name)
			if err != nil {
				logger.Log.Info("Repository is not found", zap.String("owner", rule.Owner), zap.String("name", rule.Name))
				continue
			}

			u := repo.GetCloneURL()
			if rule.urlReplaceRule != nil {
				replaced := rule.urlReplaceRule.re.ReplaceAllString(u, rule.urlReplaceRule.replace)
				if u != replaced {
					u = replaced
				}
			}
			repos[fmt.Sprintf("%s/%s", rule.Owner, rule.Name)] = &Repository{
				Name:             fmt.Sprintf("%s/%s", rule.Owner, rule.Name),
				URL:              u,
				Refs:             x.refSpecs(rule.Branches, rule.Tags),
				DisableVendoring: rule.DisableVendoring,
			}
		}
	}

	for _, rule := range x.rules {
		if rule.Query != "" {
			vars := map[string]interface{}{
				"searchQuery": githubv4.String(rule.Query),
				"cursor":      (*githubv4.String)(nil),
			}
			err := x.githubGraphQLClient.Query(ctx, &listRepositoriesQuery, vars)
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
				if _, ok := repos[fmt.Sprintf("%s/%s", v.Repository.Owner.Login, v.Repository.Name)]; ok {
					continue
				}

				u := v.Repository.URL.String()
				if rule.urlReplaceRule != nil {
					replaced := rule.urlReplaceRule.re.ReplaceAllString(u, rule.urlReplaceRule.replace)
					if u != replaced {
						u = replaced
						break
					}
				}
				repos[fmt.Sprintf("%s/%s", v.Repository.Owner.Login, v.Repository.Name)] = &Repository{
					Name:             fmt.Sprintf("%s/%s", v.Repository.Owner.Login, v.Repository.Name),
					URL:              u,
					Refs:             x.refSpecs(rule.Branches, rule.Tags),
					DisableVendoring: rule.DisableVendoring,
				}
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

	repositories := make([]*Repository, 0)
	for _, v := range repos {
		repositories = append(repositories, v)
	}
	x.mu.Lock()
	x.repositories = repositories
	x.mu.Unlock()
	return repositories
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

func newGitHubAppRESTClient(appId, installationId int64, privateKeyFile string, customEndpoint string) (*github.Client, error) {
	tr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appId, installationId, privateKeyFile)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	if c, err := newGitHubClient(&http.Client{Transport: tr}, customEndpoint); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	} else {
		return c, nil
	}
}

func newGitHubTokenRESTClient(token string, customEndpoint string) (*github.Client, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)

	if c, err := newGitHubClient(tc, customEndpoint); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	} else {
		return c, nil
	}
}

func newGitHubClient(httpClient *http.Client, customEndpoint string) (*github.Client, error) {
	if customEndpoint != "" {
		u, err := url.Parse(customEndpoint)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		c := github.NewClient(httpClient)
		c.BaseURL = u
		return c, nil
	} else {
		return github.NewClient(httpClient), nil
	}
}

func newGitHubAppGraphQLClient(appId, installationId int64, privateKeyFile string, customGraphQLEndpoint string) (*githubv4.Client, error) {
	tr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appId, installationId, privateKeyFile)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	if customGraphQLEndpoint != "" {
		return githubv4.NewEnterpriseClient(customGraphQLEndpoint, &http.Client{Transport: tr}), nil
	} else {
		return githubv4.NewClient(&http.Client{Transport: tr}), nil
	}
}

func newGitHubTokenGraphQLClient(token string, customEndpoint string) *githubv4.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)

	if customEndpoint != "" {
		return githubv4.NewEnterpriseClient(customEndpoint, tc)
	} else {
		return githubv4.NewClient(tc)
	}
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
