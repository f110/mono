package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/docutil"
	"go.f110.dev/mono/go/pkg/git"
)

func TestParsePath(t *testing.T) {
	cases := []struct {
		URL      string
		Repo     string
		FilePath string
	}{
		{
			URL:      "http://example.com/test1/_/docs/README.md",
			Repo:     "test1",
			FilePath: "docs/README.md",
		},
		{
			URL:      "http://example.com/test1/_/docs/README.md",
			Repo:     "test1",
			FilePath: "docs/README.md",
		},
		{
			URL:      "http://example.com/test1/_/docs/README.md",
			Repo:     "test1",
			FilePath: "docs/README.md",
		},
	}

	h, err := newHttpHandler(context.Background(), nil, &stubDocSearchClient{}, "", "", 0)
	require.NoError(t, err)
	for _, tc := range cases {
		t.Run(tc.URL, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.URL, nil)
			repo, filepath := h.parsePath(req)
			assert.Equal(t, tc.Repo, repo)
			assert.Equal(t, tc.FilePath, filepath)
		})
	}
}

func TestServeHTTP(t *testing.T) {
	gitData := &stubGitDataClient{}
	docSearch := &stubDocSearchClient{}
	h, err := newHttpHandler(context.Background(), gitData, docSearch, "repo-doc", "", 0)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/test/_/README.md", nil)
	h.ServeHTTP(recorder, req)

	assert.Contains(t, recorder.Body.String(), "<title>repo-doc - Document title</title>")
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

func (s *stubGitDataClient) GetRepository(ctx context.Context, in *git.RequestGetRepository, opts ...grpc.CallOption) (*git.ResponseGetRepository, error) {
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
	return &git.ResponseGetCommit{
		Commit: &git.Commit{
			Author: &git.Signature{},
		},
	}, nil
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

func (s *stubGitDataClient) Stat(_ context.Context, in *git.RequestStat, opts ...grpc.CallOption) (*git.ResponseStat, error) {
	return &git.ResponseStat{}, nil
}

func (s *stubGitDataClient) ListTag(ctx context.Context, in *git.RequestListTag, opts ...grpc.CallOption) (*git.ResponseListTag, error) {
	//TODO implement me
	panic("implement me")
}

func (s *stubGitDataClient) ListBranch(ctx context.Context, in *git.RequestListBranch, opts ...grpc.CallOption) (*git.ResponseListBranch, error) {
	//TODO implement me
	panic("implement me")
}

type stubDocSearchClient struct{}

var _ docutil.DocSearchClient = &stubDocSearchClient{}

func (s *stubDocSearchClient) AvailableFeatures(ctx context.Context, in *docutil.RequestAvailableFeatures, opts ...grpc.CallOption) (*docutil.ResponseAvailableFeatures, error) {
	return &docutil.ResponseAvailableFeatures{PageLink: true, SupportedFileType: []docutil.FileType{docutil.FileType_FILE_TYPE_MARKDOWN}}, nil
}

func (s *stubDocSearchClient) ListRepository(ctx context.Context, in *docutil.RequestListRepository, opts ...grpc.CallOption) (*docutil.ResponseListRepository, error) {
	return &docutil.ResponseListRepository{}, nil
}

func (s *stubDocSearchClient) GetRepository(ctx context.Context, in *docutil.RequestGetRepository, opts ...grpc.CallOption) (*docutil.ResponseGetRepository, error) {
	return &docutil.ResponseGetRepository{
		Repository: &docutil.Repository{Name: "test"},
	}, nil
}

func (s *stubDocSearchClient) GetPage(ctx context.Context, in *docutil.RequestGetPage, opts ...grpc.CallOption) (*docutil.ResponseGetPage, error) {
	return &docutil.ResponseGetPage{
		Doc: "# Document title\nHello World!\n",
	}, nil
}

func (s *stubDocSearchClient) PageLink(ctx context.Context, in *docutil.RequestPageLink, opts ...grpc.CallOption) (*docutil.ResponsePageLink, error) {
	return &docutil.ResponsePageLink{}, nil
}

func (s *stubDocSearchClient) GetDirectory(ctx context.Context, in *docutil.RequestGetDirectory, opts ...grpc.CallOption) (*docutil.ResponseGetDirectory, error) {
	return &docutil.ResponseGetDirectory{}, nil
}
