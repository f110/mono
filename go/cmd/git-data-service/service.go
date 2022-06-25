package main

import (
	"context"

	"go.f110.dev/mono/go/pkg/git"
)

type gitDataService struct{}

func newService() *gitDataService {
	return &gitDataService{}
}

func (g *gitDataService) ListRepositories(ctx context.Context, repositories *git.RequestListRepositories) (*git.ResponseListRepositories, error) {
	//TODO implement me
	panic("implement me")
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
