package notion

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/shurcooL/githubv4"
	"go.f110.dev/notion-api/v3"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"

	"go.f110.dev/mono/go/githubutil"
	"go.f110.dev/mono/go/pkg/k8s/volume"
	"go.f110.dev/mono/go/pkg/logger"
)

type githubTaskConfig struct {
	DatabaseID  string            `yaml:"database_id"`
	Properties  map[string]string `yaml:"properties"`
	URLProperty string            `yaml:"url_property"`
	RestrictOrg string            `yaml:"restrict_org"`
}

type GitHubTask struct {
	GHClient     *githubv4.Client
	NotionClient *notion.Client

	database   *notion.Database
	cron       *cron.Cron
	configFile string
	confMu     sync.Mutex
	config     *githubTaskConfig
	w          *volume.Watcher

	mu      sync.Mutex
	checked map[string]struct{}
}

func NewGitHubTask(appId, installationId int64, privateKeyFile, notionToken, configFile string) (*GitHubTask, error) {
	app, err := githubutil.NewApp(appId, installationId, privateKeyFile)
	if err != nil {
		return nil, err
	}
	tr := githubutil.NewTransportWithApp(http.DefaultTransport, app)
	ghClient := githubv4.NewClient(&http.Client{Transport: tr})

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: notionToken})
	tc := oauth2.NewClient(context.Background(), ts)
	notionClient, err := notion.New(tc, notion.BaseURL)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	if g, err := newGithubTask(ghClient, notionClient, configFile); err != nil {
		return nil, xerrors.WithStack(err)
	} else {
		return g, nil
	}
}

func NewGitHubTaskWithToken(githubToken, notionToken, configFile string) (*GitHubTask, error) {
	s := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})
	httpClient := oauth2.NewClient(context.Background(), s)
	client := githubv4.NewClient(httpClient)

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: notionToken})
	tc := oauth2.NewClient(context.Background(), ts)
	notionClient, err := notion.New(tc, notion.BaseURL)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	if g, err := newGithubTask(client, notionClient, configFile); err != nil {
		return nil, xerrors.WithStack(err)
	} else {
		return g, nil
	}
}

func newGithubTask(client *githubv4.Client, notionClient *notion.Client, configFile string) (*GitHubTask, error) {
	g := &GitHubTask{GHClient: client, NotionClient: notionClient, configFile: configFile, checked: make(map[string]struct{})}
	g.loadConfig()

	if volume.CanWatchVolume(configFile) {
		mountPath, err := volume.FindMountPath(configFile)
		if err == nil {
			w, err := volume.NewWatcher(mountPath, g.loadConfig)
			if err != nil {
				return nil, xerrors.WithStack(err)
			}
			g.w = w
		}
	}

	db, err := g.NotionClient.GetDatabase(context.TODO(), g.config.DatabaseID)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	g.database = db
	return g, nil
}

func (g *GitHubTask) Start(schedule string) error {
	g.cron = cron.New()
	_, err := g.cron.AddFunc(schedule, func() {
		logger.Log.Debug("Schedule check")
		if err := g.Execute(); err != nil {
			logger.Log.Warn("Failed to run", zap.Error(err))
		}
	})
	if err != nil {
		return xerrors.WithStack(err)
	}
	logger.Log.Info("Start cron")
	g.cron.Start()

	if err := g.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
	}

	return nil
}

func (g *GitHubTask) Execute() error {
	err := g.GHClient.Query(context.Background(), &query, map[string]interface{}{"query": githubv4.String("is:open assignee:@me")})
	if err != nil {
		return xerrors.WithStack(err)
	}

	g.confMu.Lock()
	defer g.confMu.Unlock()
	assigned := make(map[string]*IssueSchema)
	for _, v := range query.Search.Nodes {
		if g.config.RestrictOrg != "" && v.Issue.Repository.Owner.Login != g.config.RestrictOrg {
			continue
		}
		issue := v.Issue
		assigned[v.Issue.URL.String()] = &issue
	}

	g.mu.Lock()
	if len(g.checked) > 0 {
		for v := range g.checked {
			if _, ok := assigned[v]; ok {
				delete(assigned, v)
			}
		}
	} else {
		pages, err := g.NotionClient.GetPages(
			context.TODO(),
			g.config.DatabaseID,
			&notion.Filter{
				Property: g.config.URLProperty,
				Text: &notion.TextFilter{
					IsNotEmpty: true,
				},
			},
			nil,
		)
		if err != nil {
			return xerrors.WithStack(err)
		}
		for _, v := range pages {
			if _, ok := assigned[v.Properties[g.config.URLProperty].URL]; ok {
				delete(assigned, v.Properties[g.config.URLProperty].URL)
			}
		}
	}
	g.mu.Unlock()

	for _, v := range assigned {
		newPage, err := notion.NewPage(g.database, v.Title, nil)
		if err != nil {
			return xerrors.WithStack(err)
		}
		for k, v := range g.config.Properties {
			for p := range g.database.Properties {
				if k != p {
					continue
				}

				prop := g.database.Properties[p]
				switch prop.Type {
				case notion.PropertyTypeSelect:
					newPage.SetProperty(k, &notion.PropertyData{Type: prop.Type, Select: &notion.Option{Name: v}})
				}
			}
		}
		newPage.SetProperty(g.config.URLProperty, &notion.PropertyData{Type: "url", URL: v.URL.String()})
		logger.Log.Info("Create page", zap.String("title", v.Title), zap.String("url", v.URL.String()))
		_, err = g.NotionClient.CreatePage(context.TODO(), newPage)
		if err != nil {
			return xerrors.WithStack(err)
		}
		g.mu.Lock()
		g.checked[v.URL.String()] = struct{}{}
		g.mu.Unlock()
	}

	return nil
}

func (g *GitHubTask) loadConfig() {
	f, err := os.Open(g.configFile)
	if err != nil {
		logger.Log.Error("Failed to open config file", zap.Error(err), zap.String("path", g.configFile))
		return
	}
	var conf githubTaskConfig
	if err := yaml.NewDecoder(f).Decode(&conf); err != nil {
		logger.Log.Error("Decode failure", zap.Error(err))
		return
	}
	g.confMu.Lock()
	g.config = &conf
	g.confMu.Unlock()
}

var query struct {
	Search struct {
		Nodes []struct {
			Issue IssueSchema `graphql:"... on Issue"`
		}
	} `graphql:"search(query: $query type: ISSUE first: 100)"`
}

type IssueSchema struct {
	Number     int
	Title      string
	URL        githubv4.URI
	Repository struct {
		Owner struct {
			Login string
		}
	}
}
