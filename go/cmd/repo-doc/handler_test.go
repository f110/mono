package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/pkg/git"
)

func TestParsePath(t *testing.T) {
	cases := []struct {
		URL      string
		Repo     string
		Ref      string
		FilePath string
	}{
		{
			URL:      "http://example.com/test1/master/-/docs/README.md",
			Repo:     "test1",
			Ref:      "master",
			FilePath: "docs/README.md",
		},
		{
			URL:      "http://example.com/test1/feature/update-doc/-/docs/README.md",
			Repo:     "test1",
			Ref:      "feature/update-doc",
			FilePath: "docs/README.md",
		},
		{
			URL:      "http://example.com/test1/8e6e2933140691846d824231bde4af011200cf5a/-/docs/README.md",
			Repo:     "test1",
			Ref:      "8e6e2933140691846d824231bde4af011200cf5a",
			FilePath: "docs/README.md",
		},
	}

	h := newHttpHandler(nil, "", "", 0)
	for _, tc := range cases {
		t.Run(tc.URL, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.URL, nil)
			repo, ref, filepath := h.parsePath(req)
			assert.Equal(t, tc.Repo, repo)
			assert.Equal(t, tc.Ref, ref)
			assert.Equal(t, tc.FilePath, filepath)
		})
	}
}

func TestServeHTTP(t *testing.T) {
	client := &stubGitDataClient{}
	h := newHttpHandler(client, "", "", 0)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/test/master/-/README.md", nil)
	h.ServeHTTP(recorder, req)

	assert.Equal(t, `
<html>
<head></head>
<body>
<h1>Document title</h1>
<p>Hello World!</p>

</body>
</html>`, recorder.Body.String())
	assert.Equal(t, http.StatusOK, recorder.Code)
}

type stubGitDataClient struct{}

var _ git.GitDataClient = &stubGitDataClient{}

func (s *stubGitDataClient) ListRepositories(ctx context.Context, in *git.RequestListRepositories, opts ...grpc.CallOption) (*git.ResponseListRepositories, error) {
	//TODO implement me
	panic("implement me")
}

func (s *stubGitDataClient) ListReferences(ctx context.Context, in *git.RequestListReferences, opts ...grpc.CallOption) (*git.ResponseListReferences, error) {
	//TODO implement me
	panic("implement me")
}

func (s *stubGitDataClient) GetReference(_ context.Context, in *git.RequestGetReference, opts ...grpc.CallOption) (*git.ResponseGetReference, error) {
	return &git.ResponseGetReference{
		Ref: &git.Reference{
			Name: "master",
			Hash: "012345",
		},
	}, nil
}

func (s *stubGitDataClient) GetCommit(ctx context.Context, in *git.RequestGetCommit, opts ...grpc.CallOption) (*git.ResponseGetCommit, error) {
	//TODO implement me
	panic("implement me")
}

func (s *stubGitDataClient) GetTree(_ context.Context, in *git.RequestGetTree, opts ...grpc.CallOption) (*git.ResponseGetTree, error) {
	return &git.ResponseGetTree{
		Tree: []*git.TreeEntry{
			{Path: "README.md", Sha: "0123456789"},
		},
	}, nil
}

func (s *stubGitDataClient) GetBlob(_ context.Context, in *git.RequestGetBlob, opts ...grpc.CallOption) (*git.ResponseGetBlob, error) {
	return &git.ResponseGetBlob{
		Sha:     "0123456789",
		Content: []byte("# Document title\nHello World!\n"),
		Size:    30,
	}, nil
}

func (s *stubGitDataClient) GetFile(_ context.Context, in *git.RequestGetFile, opts ...grpc.CallOption) (*git.ResponseGetFile, error) {
	return &git.ResponseGetFile{
		Content: []byte("# Document title\nHello World!\n"),
	}, nil
}

func (s *stubGitDataClient) ListTag(ctx context.Context, in *git.RequestListTag, opts ...grpc.CallOption) (*git.ResponseListTag, error) {
	//TODO implement me
	panic("implement me")
}

func (s *stubGitDataClient) ListBranch(ctx context.Context, in *git.RequestListBranch, opts ...grpc.CallOption) (*git.ResponseListBranch, error) {
	//TODO implement me
	panic("implement me")
}
