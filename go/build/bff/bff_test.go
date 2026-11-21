package bff

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.f110.dev/mono/go/build/api"
	"go.f110.dev/mono/go/build/model"
	"go.f110.dev/mono/go/storage"
	"go.f110.dev/mono/go/testing/assertion"
)

type fakeAPIClient struct {
	api.APIClient
	serverInfo *api.ResponseGetServerInfo
}

func (f *fakeAPIClient) GetServerInfo(_ context.Context, _ *api.RequestGetServerInfo, _ ...grpc.CallOption) (*api.ResponseGetServerInfo, error) {
	return f.serverInfo, nil
}

func TestBFF_GetServerInfo(t *testing.T) {
	t.Run("pass the commit of the server", func(t *testing.T) {
		b := &BFF{apiClient: &fakeAPIClient{serverInfo: api.ResponseGetServerInfo_builder{
			CommitSha:  new("e192aef54cb0d31afd7cae64b079be2a12a56a74"),
			CommitTime: timestamppb.New(time.Date(2026, 11, 13, 14, 3, 20, 0, time.UTC)),
		}.Build()}}

		res, err := b.GetServerInfo(context.Background(), connect.NewRequest(RequestGetServerInfo_builder{}.Build()))
		assertion.MustNoError(t, err)
		assertion.Equal(t, res.Msg.GetCommitSha(), "e192aef54cb0d31afd7cae64b079be2a12a56a74")
		assertion.True(t, res.Msg.GetCommitTime().AsTime().Equal(time.Date(2026, 11, 13, 14, 3, 20, 0, time.UTC)))
	})

	t.Run("the commit of the server is unknown", func(t *testing.T) {
		b := &BFF{apiClient: &fakeAPIClient{serverInfo: api.ResponseGetServerInfo_builder{}.Build()}}

		res, err := b.GetServerInfo(context.Background(), connect.NewRequest(RequestGetServerInfo_builder{}.Build()))
		assertion.MustNoError(t, err)
		assertion.Equal(t, res.Msg.GetCommitSha(), "")
		assertion.False(t, res.Msg.HasCommitTime())
	})
}

func TestNewBFF(t *testing.T) {
	t.Run("serve the assets of the frontend", func(t *testing.T) {
		dir := t.TempDir()
		assertion.MustNoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("index"), 0644))
		assertion.MustNoError(t, os.Mkdir(filepath.Join(dir, "assets"), 0755))
		assertion.MustNoError(t, os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("app"), 0644))
		b := NewBFF(":0", nil, &fakeAPIClient{}, nil, "", storage.S3Options{}, dir)

		cases := map[string]string{
			"/":              "index",
			"/assets/app.js": "app",
			"/tasks/1":       "index",
		}
		for p, body := range cases {
			rec := httptest.NewRecorder()
			b.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
			assertion.Equal(t, rec.Code, http.StatusOK)
			assertion.Equal(t, rec.Body.String(), body)
		}
	})

	t.Run("the assets are not served without the directory", func(t *testing.T) {
		b := NewBFF(":0", nil, &fakeAPIClient{}, nil, "", storage.S3Options{}, "")

		rec := httptest.NewRecorder()
		b.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		assertion.Equal(t, rec.Code, http.StatusNotFound)
	})
}

func TestApiArtifactToBFFArtifact(t *testing.T) {
	t.Run("tagged image keeps its digest and surfaces the tag", func(t *testing.T) {
		a := model.Artifact_builder{
			Type:       new(model.ArtifactType_ARTIFACT_TYPE_CONTAINER_IMAGE),
			Hash:       new("sha256:abc123"),
			Name:       new("foo"),
			Repository: new("build/foo"),
			Reference:  new("latest"),
			Size:       new(int64(42)),
		}.Build()

		got := apiArtifactToBFFArtifact(a)
		assertion.Equal(t, "sha256:abc123", got.GetDigest())
		assertion.Equal(t, "latest", got.GetTag())
		assertion.Equal(t, "build/foo", got.GetRegistry())
		assertion.Equal(t, "foo", got.GetName())
	})

	t.Run("file hash is normalized to sha256 and untagged when stored under a digest", func(t *testing.T) {
		a := model.Artifact_builder{
			Type:      new(model.ArtifactType_ARTIFACT_TYPE_BINARY),
			Hash:      new("deadbeef"),        // files store a bare hex sha256
			Reference: new("sha256:deadbeef"), // addressed by content digest
		}.Build()

		got := apiArtifactToBFFArtifact(a)
		assertion.Equal(t, "sha256:deadbeef", got.GetDigest())
		assertion.Equal(t, "", got.GetTag())
	})
}
