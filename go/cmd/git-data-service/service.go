package main

import (
	"context"
	"io"

	goGit "github.com/go-git/go-git/v5"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/pkg/git"
	"go.f110.dev/mono/go/pkg/storage"
)

type gitDataService struct {
	repo map[string]*goGit.Repository
}

type repository struct {
	Name   string
	Prefix string
}

type ObjectStorageInterface interface {
	PutReader(ctx context.Context, name string, data io.Reader) error
	Delete(ctx context.Context, name string) error
	Get(ctx context.Context, name string) (io.ReadCloser, error)
	List(ctx context.Context, prefix string) ([]*storage.Object, error)
}

func newService(s ObjectStorageInterface, repositories []repository) (*gitDataService, error) {
	repo := make(map[string]*goGit.Repository)
	for _, r := range repositories {
		storer := git.NewObjectStorageStorer(s, r.Prefix)
		gitRepo, err := goGit.Open(storer, nil)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}

		repo[r.Name] = gitRepo
	}

	return &gitDataService{repo: repo}, nil
}

func (g *gitDataService) ListRepositories(ctx context.Context, repositories *git.RequestListRepositories) (*git.ResponseListRepositories, error) {
	var list []string
	for k := range g.repo {
		list = append(list, k)
	}

	return &git.ResponseListRepositories{Repositories: list}, nil
}

func (g *gitDataService) ListReferences(ctx context.Context, references *git.RequestListReferences) (*git.ResponseListReferences, error) {
	//TODO implement me
	panic("implement me")
}

func (g *gitDataService) GetCommit(ctx context.Context, commit *git.RequestGetCommit) (*git.ResponseGetCommit, error) {
	//TODO implement me
	panic("implement me")
}

func (g *gitDataService) GetTree(ctx context.Context, tree *git.RequestGetTree) (*git.ResponseGetTree, error) {
	//TODO implement me
	panic("implement me")
}

func (g *gitDataService) GetBlob(ctx context.Context, blob *git.RequestGetBlob) (*git.ResponseGetBlob, error) {
	//TODO implement me
	panic("implement me")
}
