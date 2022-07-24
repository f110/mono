package docutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/pkg/git"
)

func TestParseMarkdown(t *testing.T) {
	service := NewDocSearchService(&mockGitClient{})
	err := service.scanRepository(context.Background(), &git.Repository{Name: "test", DefaultBranch: "master"})
	require.NoError(t, err)
	t.Log(service.pageLink["test"]["README.md"].LinkOut)
}

type mockGitClient struct{}

func (m *mockGitClient) ListRepositories(ctx context.Context, in *git.RequestListRepositories, opts ...grpc.CallOption) (*git.ResponseListRepositories, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockGitClient) ListReferences(ctx context.Context, in *git.RequestListReferences, opts ...grpc.CallOption) (*git.ResponseListReferences, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockGitClient) GetRepository(ctx context.Context, in *git.RequestGetRepository, opts ...grpc.CallOption) (*git.ResponseGetRepository, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockGitClient) GetReference(ctx context.Context, in *git.RequestGetReference, opts ...grpc.CallOption) (*git.ResponseGetReference, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockGitClient) GetCommit(ctx context.Context, in *git.RequestGetCommit, opts ...grpc.CallOption) (*git.ResponseGetCommit, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockGitClient) GetTree(ctx context.Context, in *git.RequestGetTree, opts ...grpc.CallOption) (*git.ResponseGetTree, error) {
	return &git.ResponseGetTree{
		Tree: []*git.TreeEntry{
			{Path: "README.md"},
		},
	}, nil
}

func (m *mockGitClient) GetBlob(ctx context.Context, in *git.RequestGetBlob, opts ...grpc.CallOption) (*git.ResponseGetBlob, error) {
	return &git.ResponseGetBlob{Content: []byte(`# Test document
[Link](https://example.com)

https://example.com/autolink

[Anchor](#anchor)
`)}, nil
}

func (m *mockGitClient) GetFile(ctx context.Context, in *git.RequestGetFile, opts ...grpc.CallOption) (*git.ResponseGetFile, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockGitClient) ListTag(ctx context.Context, in *git.RequestListTag, opts ...grpc.CallOption) (*git.ResponseListTag, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockGitClient) ListBranch(ctx context.Context, in *git.RequestListBranch, opts ...grpc.CallOption) (*git.ResponseListBranch, error) {
	//TODO implement me
	panic("implement me")
}
