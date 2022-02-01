package repoindexer

import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/v32/github"
	"github.com/shurcooL/githubv4"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/pkg/logger"
)

type Repository struct {
	Name             string
	URL              string
	DisableVendoring bool
	Refs             []plumbing.ReferenceName
	CABundle         []byte

	repo *git.Repository
}

func NewRepository(name, url string, refs []plumbing.ReferenceName, disableVendoring bool, caBundle []byte) *Repository {
	return &Repository{
		Name:             name,
		URL:              url,
		Refs:             refs,
		DisableVendoring: disableVendoring,
		CABundle:         caBundle,
	}
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
	caBundle            []byte

	mu           sync.Mutex
	repositories []*Repository
}

func NewRepositoryLister(rules []*Rule, restClient *github.Client, graphqlClient *githubv4.Client, caBundle []byte) *RepositoryLister {
	return &RepositoryLister{rules: rules, githubClient: restClient, githubGraphQLClient: graphqlClient, caBundle: caBundle}
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
				logger.Log.Info("Repository is not found", zap.String("owner", rule.Owner), zap.String("name", rule.Name), zap.Error(err))
				continue
			}

			u := repo.GetCloneURL()
			if rule.urlReplaceRule != nil {
				replaced := rule.urlReplaceRule.re.ReplaceAllString(u, rule.urlReplaceRule.replace)
				if u != replaced {
					u = replaced
				}
			}
			repos[fmt.Sprintf("%s/%s", rule.Owner, rule.Name)] = NewRepository(
				fmt.Sprintf("%s/%s", rule.Owner, rule.Name),
				u,
				x.refSpecs(rule.Branches, rule.Tags),
				rule.DisableVendoring,
				x.caBundle,
			)
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

				repos[fmt.Sprintf("%s/%s", v.Repository.Owner.Login, v.Repository.Name)] = NewRepository(
					fmt.Sprintf("%s/%s", v.Repository.Owner.Login, v.Repository.Name),
					u,
					x.refSpecs(rule.Branches, rule.Tags),
					rule.DisableVendoring,
					x.caBundle,
				)
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
