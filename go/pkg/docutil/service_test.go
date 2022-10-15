package docutil

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/pkg/git"
)

func TestDocSearchService(t *testing.T) {
	doc := `# Test document
[Link](https://example.com)

https://example.com/autolink

[Anchor](#anchor)
`
	service := NewDocSearchService(&mockGitClient{Blobs: map[string]string{"README.md": doc}}, nil)
	err := service.scanRepository(context.Background(), &git.Repository{Name: "test", DefaultBranch: "master"}, 1)
	require.NoError(t, err)
}

func TestCitedLink(t *testing.T) {
	blobs := map[string]string{
		"docs/README.md": `# README
- [Page 1](page1.md)
- [Issue 1](https://github.com/f110/mono/issues/1)
`,
		"docs/page1.md": "# Page 1",
		"README.md": `# Top README
- [docs](./docs/README.md)`,
	}
	service := NewDocSearchService(&mockGitClient{Blobs: blobs}, nil)
	err := service.scanRepository(
		context.Background(),
		&git.Repository{
			Name:          "mono",
			DefaultBranch: "master",
			Url:           "https://github.com/f110/mono",
		},
		1,
	)
	require.NoError(t, err)
	service.interpolateCitedLinks()

	if assert.Len(t, service.data["mono"].Pages["docs/README.md"].LinkOut, 2) {
		out := service.data["mono"].Pages["docs/README.md"].LinkOut
		assert.Equal(t, "docs/page1.md", out[0].Destination)
		assert.Equal(t, "https://github.com/f110/mono/issues/1", out[1].Destination)
	}
	assert.Len(t, service.data["mono"].Pages["docs/README.md"].LinkIn, 1)
	assert.Len(t, service.data["mono"].Pages["docs/page1.md"].LinkIn, 1)
}

type mockGitClient struct {
	Blobs map[string]string

	treeEntry []*git.TreeEntry
}

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
	if m.treeEntry != nil {
		return &git.ResponseGetTree{Tree: m.treeEntry}, nil
	}

	buf := make([]byte, 512)
	var entries []*git.TreeEntry
	for k := range m.Blobs {
		_, err := rand.Read(buf)
		if err != nil {
			return nil, err
		}
		h := sha256.Sum256(buf)
		entries = append(entries, &git.TreeEntry{Path: k, Sha: hex.EncodeToString(h[:])})
	}
	m.treeEntry = entries
	return &git.ResponseGetTree{
		Tree: entries,
	}, nil
}

func (m *mockGitClient) GetBlob(ctx context.Context, in *git.RequestGetBlob, opts ...grpc.CallOption) (*git.ResponseGetBlob, error) {
	for _, v := range m.treeEntry {
		if v.Sha == in.Sha {
			return &git.ResponseGetBlob{Content: []byte(m.Blobs[v.Path])}, nil
		}
	}
	return nil, errors.New("not found")
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

func (m *mockGitClient) Stat(ctx context.Context, in *git.RequestStat, opts ...grpc.CallOption) (*git.ResponseStat, error) {
	// TODO: implement me
	panic("implement me")
}
