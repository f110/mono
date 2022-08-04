package docutil

import (
	"container/list"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/pkg/git"
)

func Test(t *testing.T) {
	r := strings.NewReader(`<!DOCTYPE html>
	<html lang="en" data-layout="responsive">
	 <head>
	
	   <script>
	     window.addEventListener('error', window.__err=function f(e){f.p=f.p||[];f.p.push(e)});
	   </script>
	   <script>
	     (function() {
	       const theme = document.cookie.match(/prefers-color-scheme=(light|dark|auto)/)?.[1]
	       if (theme) {
	         document.querySelector('html').setAttribute('data-theme', theme);
	       }
	     }())
	   </script>
	   <meta charset="utf-8">
	   <meta http-equiv="X-UA-Compatible" content="IE=edge">
	   <meta name="viewport" content="width=device-width, initial-scale=1.0">
	   <meta name="Description" content="Package html implements an HTML5-compliant tokenizer and parser.">
	
	   <meta class="js-gtmID" data-gtmid="GTM-W8MVQXG">
	   <link rel="shortcut icon" href="/static/shared/icon/favicon.ico">
	
	 <link rel="canonical" href="https://pkg.go.dev/golang.org/x/net/html">
	
	   <link href="/static/frontend/frontend.min.css?version=prod-frontend-00043-nan" rel="stylesheet">
	
	 <title>html package - golang.org/x/net/html - Go Packages</title>
	
	
	 <link href="/static/frontend/unit/unit.min.css?version=prod-frontend-00043-nan" rel="stylesheet">
	
	 <link href="/static/frontend/unit/main/main.min.css?version=prod-frontend-00043-nan" rel="stylesheet">
	
	
	 </head>
	 <body>
	 <ul>
	     <li><a href="https://example.com/foo">foo</a></li>
	     <li><a href="https://example.com/bar">bar</a></li>
	     <li><a href="https://example.com/baz">baz</a></li>
	 </ul>
	</body>
	</html>`)

	node, err := html.Parse(r)
	if err != nil {
		t.Fatal(err)
	}

	stack := list.New()
	stack.PushBack(node)
	for stack.Len() != 0 {
		e := stack.Back()
		stack.Remove(e)

		node := e.Value.(*html.Node)
		if node.DataAtom == atom.Title {
			t.Log(node.FirstChild.Data)
			break
		}

		for c := node.FirstChild; c != nil; c = c.NextSibling {
			stack.PushBack(c)
		}
	}
}

func TestParseMarkdown(t *testing.T) {
	service := NewDocSearchService(&mockGitClient{}, nil)
	err := service.scanRepository(context.Background(), &git.Repository{Name: "test", DefaultBranch: "master"}, 1)
	require.NoError(t, err)
	t.Log(service.data["test"]["README.md"].LinkOut)
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
