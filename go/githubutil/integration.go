package githubutil

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/url"
	"os"

	"github.com/google/go-github/v49/github"
	"github.com/shurcooL/githubv4"
	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
)

type GitHubClientFactory struct {
	Initialized   bool
	REST          *github.Client
	GraphQL       *githubv4.Client
	TokenProvider *TokenProvider

	Name           string
	AppID          int64
	InstallationID int64
	PrivateKeyFile string
	// Token is the personal access token. Not an app token or an access token.
	// An access token will provided via TokenProvider.
	Token                 string
	GitHubAPIEndpoint     string
	GitHubGraphQLEndpoint string
	CACertFile            string

	requiredCredential bool
}

func NewGitHubClientFactory(name string, requiredCredential bool) *GitHubClientFactory {
	return &GitHubClientFactory{Name: name, requiredCredential: requiredCredential}
}

func (g *GitHubClientFactory) Flags(fs *cli.FlagSet) {
	fs.Int64("github-app-id", "GitHub Application ID")
	fs.Int64("github-installation-id", "GitHub Application installation ID").Var(&g.InstallationID)
	fs.String("github-private-key-file", "Private key file for GitHub App").Var(&g.PrivateKeyFile)
	fs.String("github-token", "Personal access token for GitHub").Var(&g.Token)
	fs.String("github-api-endpoint", "REST API endpoint of github if you want to use non-default endpoint").Var(&g.GitHubAPIEndpoint)
	fs.String("github-graphql-endpoint", "GraphQL endpoint of github if you want to use non-default endpoint").Var(&g.GitHubGraphQLEndpoint)
	fs.String("github-ca-cert-file", "Certificate file path").Var(&g.CACertFile)
}

func (g *GitHubClientFactory) PFlags(fs *pflag.FlagSet) {
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
			return xerrors.Define("any a credential for GitHub is mandatory. GitHub app or Personal access token is required").WithStack()
		}
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		return xerrors.WithStack(err)
	}
	if g.CACertFile != "" {
		b, err := os.ReadFile(g.CACertFile)
		if err != nil {
			return xerrors.WithStack(err)
		}
		if ok := rootCAs.AppendCertsFromPEM(b); !ok {
			return xerrors.Define("failed to read a certificate").WithStack()
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: rootCAs}
	}

	var httpClient *http.Client
	var app *App
	if g.AppID > 0 && g.InstallationID > 0 && g.PrivateKeyFile != "" {
		app, err = NewApp(g.AppID, g.InstallationID, g.PrivateKeyFile)
		if err != nil {
			return err
		}
	}
	if g.Token != "" || app != nil {
		g.TokenProvider = &TokenProvider{pat: g.Token, app: app}
		httpClient = &http.Client{Transport: NewTransport(transport, g.TokenProvider)}
	}
	if httpClient == nil {
		// If not provided any credential, We make the bare client.
		httpClient = &http.Client{Transport: transport}
	}

	restClient := github.NewClient(httpClient)
	if g.GitHubAPIEndpoint != "" {
		u, err := url.Parse(g.GitHubAPIEndpoint)
		if err != nil {
			return xerrors.WithStack(err)
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
