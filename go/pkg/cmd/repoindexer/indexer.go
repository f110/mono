package repoindexer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/go-github/v32/github"
	"github.com/google/zoekt"
	"github.com/google/zoekt/build"
	"github.com/shurcooL/githubv4"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/logger"
)

type Indexer struct {
	config  *Config
	workDir string
	token   string

	repositories []*repository

	githubClient  *github.Client
	graphQLClient *githubv4.Client
}

func NewIndexer(rules *Config, workDir, token string) *Indexer {
	return &Indexer{config: rules, workDir: workDir, token: token}
}

func (x *Indexer) Sync() error {
	repositories := x.listRepositories()
	for _, v := range repositories {
		logger.Log.Debug("Found repository", zap.String("name", v.Name), zap.String("url", v.URL))

		if err := v.sync(x.workDir); err != nil {
			logger.Log.Info("Failed sync repository", zap.Error(err), zap.String("url", v.URL))
			continue
		}

		if err := v.checkout(x.workDir); err != nil {
			logger.Log.Info("Failed checkout repository", zap.Error(err), zap.String("name", v.Name))
			continue
		}
	}

	return nil
}

func (x *Indexer) Mutate() error {
	for _, v := range x.listRepositories() {
		dir := filepath.Join(x.workDir, v.Name)
		err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if info.Name() == "go.mod" {
				if strings.Contains(strings.TrimPrefix(path, dir), "vendor/") {
					return nil
				}

				if _, err := os.Stat(filepath.Join(filepath.Dir(path), "vendor")); !os.IsNotExist(err) {
					return nil
				}

				cmd := exec.Command("go", "mod", "vendor")
				cmd.Dir = filepath.Dir(path)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					return err
				}
			}

			return nil
		})
		if err != nil {
			logger.Log.Info("Failed mutate repository", zap.String("name", v.Name))
			continue
		}
	}

	return nil
}

func (x *Indexer) BuildIndex() error {
	for _, v := range x.listRepositories() {
		opt := build.Options{
			IndexDir: filepath.Join(x.workDir, ".index"),
			RepositoryDescription: zoekt.Repository{
				Name: v.Name,
			},
		}
		opt.SetDefaults()
		builder, err := build.NewBuilder(opt)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		dir := filepath.Join(x.workDir, v.Name)
		err = filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			buf, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err := builder.Add(zoekt.Document{
				Name:    path,
				Content: buf,
			}); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			logger.Log.Info("Failed create the index", zap.String("name", v.Name))
			continue
		}

		if err := builder.Finish(); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

type repository struct {
	Name string
	URL  string
}

func (x *repository) sync(workDir string) error {
	bareDir := filepath.Join(workDir, ".bare", x.Name)
	if _, err := os.Stat(bareDir); os.IsNotExist(err) {
		if err := os.MkdirAll(bareDir, 0755); err != nil {
			return xerrors.Errorf(": %w", err)
		}

		_, err = git.PlainClone(bareDir, true, &git.CloneOptions{
			URL:   x.URL,
			Depth: 1,
		})
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
	} else if err != nil {
		return xerrors.Errorf(": %w", err)
	} else {
		r, err := git.PlainOpen(bareDir)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		err = r.Fetch(&git.FetchOptions{Depth: 1})
		if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func (x *repository) checkout(workDir string) error {
	dir := filepath.Join(workDir, ".bare", x.Name)
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	ref, err := repo.Head()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, x.Name+".commit")); !os.IsNotExist(err) {
		buf, err := os.ReadFile(filepath.Join(workDir, x.Name+".commit"))
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		if string(buf) == ref.Hash().String() {
			logger.Log.Debug("Already checked out", zap.String("name", x.Name))
			return nil
		}
	}

	tree, err := commit.Tree()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(filepath.Join(workDir, x.Name)), 0755); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	err = os.WriteFile(filepath.Join(workDir, x.Name+".commit"), []byte(ref.Hash().String()), 0644)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	repoRootDir := filepath.Join(workDir, x.Name)
	err = tree.Files().ForEach(func(f *object.File) error {
		switch f.Mode {
		case filemode.Regular:
			r, err := f.Reader()
			if err != nil {
				return err
			}
			if err = os.MkdirAll(filepath.Dir(filepath.Join(repoRootDir, f.Name)), 0755); err != nil {
				return err
			}
			newFile, err := os.Create(filepath.Join(repoRootDir, f.Name))
			if err != nil {
				return err
			}
			_, err = io.Copy(newFile, r)
			if err != nil {
				return err
			}
			newFile.Close()
			r.Close()
		case filemode.Symlink:
		}

		return nil
	})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (x *Indexer) listRepositories() []*repository {
	if x.repositories != nil {
		return x.repositories
	}

	repos := make([]*repository, 0)

	for _, rule := range x.config.Rules {
		if rule.Owner != "" && rule.Name != "" {
			repo, _, err := x.githubRESTClient().Repositories.Get(context.Background(), rule.Owner, rule.Name)
			if err != nil {
				logger.Log.Info("Repository is not found", zap.String("owner", rule.Owner), zap.String("name", rule.Name))
				continue
			}
			repos = append(repos, &repository{Name: fmt.Sprintf("%s/%s", rule.Owner, rule.Name), URL: repo.GetGitURL()})
		}

		if rule.Query != "" {
			vars := map[string]interface{}{
				"searchQuery": githubv4.String(rule.Query),
				"cursor":      (*githubv4.String)(nil),
			}
			err := x.githubGraphQLClient().Query(context.Background(), &listRepositoriesQuery, vars)
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
				repos = append(repos, &repository{Name: fmt.Sprintf("%s/%s", v.Repository.Owner.Login, v.Repository.Name), URL: v.Repository.URL.String()})
			}
			if !listRepositoriesQuery.Search.PageInfo.HasNextPage {
				break
			}
			vars["cursor"] = listRepositoriesQuery.Search.PageInfo.EndCursor
		}
	}

	x.repositories = repos
	return repos
}

func (x *Indexer) githubRESTClient() *github.Client {
	if x.githubClient != nil {
		return x.githubClient
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: x.token})
	tc := oauth2.NewClient(context.Background(), ts)

	x.githubClient = github.NewClient(tc)
	return x.githubClient
}

func (x *Indexer) githubGraphQLClient() *githubv4.Client {
	if x.graphQLClient != nil {
		return x.graphQLClient
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: x.token})
	tc := oauth2.NewClient(context.Background(), ts)
	x.graphQLClient = githubv4.NewClient(tc)
	return x.graphQLClient
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
