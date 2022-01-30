package githubutil

import (
	"context"
	"net/http"
	"net/url"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v32/github"
	"github.com/shurcooL/githubv4"
	"github.com/spf13/pflag"
	"golang.org/x/oauth2"
	"golang.org/x/xerrors"
)

type GitHubClientFactory struct {
	Initialized bool
	REST        *github.Client
	GraphQL     *githubv4.Client

	Name                  string
	AppID                 int64
	InstallationID        int64
	PrivateKeyFile        string
	Token                 string
	GitHubAPIEndpoint     string
	GitHubGraphQLEndpoint string

	requiredCredential bool
}

func NewGitHubClientFactory(name string, requiredCredential bool) *GitHubClientFactory {
	return &GitHubClientFactory{Name: name, requiredCredential: requiredCredential}
}

func (g *GitHubClientFactory) Flags(fs *pflag.FlagSet) {
	fs.Int64Var(&g.AppID, "github-app-id", g.AppID, "GitHub Application ID")
	fs.Int64Var(&g.InstallationID, "github-installation-id", g.InstallationID, "GitHub Application installation ID")
	fs.StringVar(&g.PrivateKeyFile, "github-private-key-file", g.PrivateKeyFile, "Private key file for GitHub App")
	fs.StringVar(&g.Token, "github-token", g.Token, "Personal access token for GitHub")
	fs.StringVar(&g.GitHubAPIEndpoint, "github-api-endpoint", g.GitHubAPIEndpoint, "REST API endpoint of github if you want to use non-default endpoint")
	fs.StringVar(&g.GitHubGraphQLEndpoint, "github-graphql-endpoint", g.GitHubGraphQLEndpoint, "GraphQL endpoint of github if you want to use non-default endpoint")
}

func (g *GitHubClientFactory) Init() error {
	if g.requiredCredential {
		if g.Token == "" && !(g.AppID > 0 && g.InstallationID > 0 && g.PrivateKeyFile != "") {
			return xerrors.Errorf("any a credential for GitHub is mandatory. GitHub app or Personal access token is required")
		}
	}

	httpClient := http.DefaultClient
	if g.AppID > 0 && g.InstallationID > 0 && g.PrivateKeyFile != "" {
		tr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, g.AppID, g.InstallationID, g.PrivateKeyFile)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		httpClient = &http.Client{Transport: tr}
	}
	if g.Token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: g.Token})
		httpClient = oauth2.NewClient(context.Background(), ts)
	}

	restClient := github.NewClient(httpClient)
	if g.GitHubAPIEndpoint != "" {
		u, err := url.Parse(g.GitHubAPIEndpoint)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		restClient.BaseURL = u
	}
	g.REST = restClient

	if g.GitHubGraphQLEndpoint != "" {
		g.GraphQL = githubv4.NewEnterpriseClient(g.GitHubGraphQLEndpoint, httpClient)
	} else {
		g.GraphQL = githubv4.NewClient(httpClient)
	}

	g.Initialized = true
	return nil
}
