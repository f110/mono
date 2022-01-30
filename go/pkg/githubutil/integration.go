package githubutil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/url"
	"os"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v32/github"
	"github.com/shurcooL/githubv4"
	"github.com/spf13/pflag"
	"golang.org/x/oauth2"
	"golang.org/x/xerrors"
)

type GitHubClientFactory struct {
	Initialized   bool
	REST          *github.Client
	GraphQL       *githubv4.Client
	TokenProvider *TokenProvider

	Name                  string
	AppID                 int64
	InstallationID        int64
	PrivateKeyFile        string
	Token                 string
	GitHubAPIEndpoint     string
	GitHubGraphQLEndpoint string
	CACertFile            string

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
	fs.StringVar(&g.CACertFile, "github-ca-cert-file", g.CACertFile, "Certificate file path")
}

func (g *GitHubClientFactory) Init() error {
	if os.Getenv("GITHUB_TOKEN") != "" && g.Token == "" {
		g.Token = os.Getenv("GITHUB_TOKEN")
	}

	if g.requiredCredential {
		if g.Token == "" && !(g.AppID > 0 && g.InstallationID > 0 && g.PrivateKeyFile != "") {
			return xerrors.Errorf("any a credential for GitHub is mandatory. GitHub app or Personal access token is required")
		}
	}

	httpClient := http.DefaultClient
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if g.CACertFile != "" {
		cp, err := x509.SystemCertPool()
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		b, err := os.ReadFile(g.CACertFile)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		if ok := cp.AppendCertsFromPEM(b); !ok {
			return xerrors.Errorf("failed to read a certificate")
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: cp}
	}
	var appTransport *ghinstallation.Transport
	if g.AppID > 0 && g.InstallationID > 0 && g.PrivateKeyFile != "" {
		tr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, g.AppID, g.InstallationID, g.PrivateKeyFile)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		httpClient = &http.Client{Transport: tr}
		appTransport = tr
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

	g.TokenProvider = &TokenProvider{pat: g.Token, appProvider: appTransport}
	g.Initialized = true
	return nil
}

type TokenProvider struct {
	pat         string
	appProvider *ghinstallation.Transport
}

func (p *TokenProvider) Token(ctx context.Context) (string, error) {
	if p.pat != "" {
		return p.pat, nil
	}
	if p.appProvider != nil {
		return p.appProvider.Token(ctx)
	}

	return "", xerrors.New("does not configure with any credential")
}
