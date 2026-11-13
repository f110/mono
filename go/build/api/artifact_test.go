package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/database/dao/daotest"
	"go.f110.dev/mono/go/build/model"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestListArtifacts(t *testing.T) {
	t.Run("by task id", func(t *testing.T) {
		t.Parallel()
		m := daotest.NewArtifact()
		m.RegisterListByTaskId(7, []*database.Artifact{
			{Id: 1, TaskId: 7, Type: database.ArtifactTypeBinary, Hash: "aaa", Name: "foo", Repository: "build/foo", Reference: "sha256:aaa", Size: 42},
		}, nil)
		s := &apiService{dao: dao.Options{Artifact: m}}

		req := &RequestListArtifacts{}
		req.SetTaskId(7)
		res, err := s.ListArtifacts(context.Background(), req)
		assertion.MustNoError(t, err)
		assertion.Len(t, res.GetArtifacts(), 1)
		got := res.GetArtifacts()[0]
		assertion.Equal(t, int32(7), got.GetTaskId())
		assertion.Equal(t, "aaa", got.GetHash())
		assertion.Equal(t, "build/foo", got.GetRepository())
		assertion.Equal(t, model.ArtifactType_ARTIFACT_TYPE_BINARY, got.GetType())
	})

	t.Run("by repository and revision aggregates the commit's tasks", func(t *testing.T) {
		t.Parallel()
		taskDAO := daotest.NewTask()
		taskDAO.RegisterListByRevision(3, "rev1", []*database.Task{{Id: 10}, {Id: 11}}, nil)
		m := daotest.NewArtifact()
		m.RegisterListByTaskId(10, []*database.Artifact{{Id: 1, TaskId: 10, Hash: "img"}}, nil)
		m.RegisterListByTaskId(11, []*database.Artifact{{Id: 2, TaskId: 11, Hash: "bin"}}, nil)
		s := &apiService{dao: dao.Options{Task: taskDAO, Artifact: m}}

		req := &RequestListArtifacts{}
		req.SetRepositoryId(3)
		req.SetRevision("rev1")
		res, err := s.ListArtifacts(context.Background(), req)
		assertion.MustNoError(t, err)
		assertion.Len(t, res.GetArtifacts(), 2)
	})

	t.Run("without a selector is rejected", func(t *testing.T) {
		t.Parallel()
		s := &apiService{}
		_, err := s.ListArtifacts(context.Background(), &RequestListArtifacts{})
		assertion.Error(t, err)
	})
}

func artifactRef(t ArtifactType, hash string) *ArtifactRef {
	r := &ArtifactRef{}
	r.SetType(t)
	r.SetHash(hash)
	return r
}

func TestSyncArtifacts(t *testing.T) {
	t.Run("all new, deduplicated", func(t *testing.T) {
		t.Parallel()
		m := daotest.NewArtifact()
		m.RegisterListByHash("aaa", nil, nil) // not stored yet
		m.RegisterListByHash("bbb", nil, nil)
		s := &apiService{dao: dao.Options{Artifact: m}}

		req := &RequestSyncArtifacts{}
		req.SetTaskId(7)
		req.SetArtifacts([]*ArtifactRef{
			artifactRef(ArtifactType_ARTIFACT_TYPE_BINARY, "aaa"),
			artifactRef(ArtifactType_ARTIFACT_TYPE_K8S_MANIFEST, "bbb"),
			artifactRef(ArtifactType_ARTIFACT_TYPE_BINARY, "aaa"), // duplicate collapses
		})
		res, err := s.SyncArtifacts(context.Background(), req)
		assertion.MustNoError(t, err)
		assertion.Equal(t, []string{"aaa", "bbb"}, res.GetNewHashes())
		assertion.Len(t, m.Called("Create"), 0) // new content is recorded on upload, not here
	})

	t.Run("already stored records the production", func(t *testing.T) {
		t.Parallel()
		m := daotest.NewArtifact()
		m.RegisterListByHash("aaa", []*database.Artifact{{Repository: "example.com/foo", Reference: "sha256:x", Size: 42}}, nil)
		s := &apiService{dao: dao.Options{Artifact: m}}

		ref := artifactRef(ArtifactType_ARTIFACT_TYPE_BINARY, "aaa")
		ref.SetName("foo")
		req := &RequestSyncArtifacts{}
		req.SetTaskId(7)
		req.SetArtifacts([]*ArtifactRef{ref})
		res, err := s.SyncArtifacts(context.Background(), req)
		assertion.MustNoError(t, err)
		assertion.Len(t, res.GetNewHashes(), 0) // already stored: nothing to upload

		called := m.Called("Create")
		assertion.Len(t, called, 1) // but the production is recorded for the task
		stored := called[0].Args["artifact"].(*database.Artifact)
		assertion.Equal(t, int32(7), stored.TaskId)
		assertion.Equal(t, "aaa", stored.Hash)
		assertion.Equal(t, "foo", stored.Name)
		assertion.Equal(t, "example.com/foo", stored.Repository)
		assertion.Equal(t, "sha256:x", stored.Reference) // copied from existing content
		assertion.Equal(t, int64(42), stored.Size)
	})
}

// fakeRegistry records the last push so tests can assert on it.
type fakeRegistry struct {
	fileRepo, fileTag, fileArtifactType, fileMediaType, fileName string
	fileContent                                                  []byte
	fileRef                                                      string
	imageRef                                                     string
}

func (f *fakeRegistry) PushImage(_ context.Context, _, _, _ string, _ io.Reader) (string, error) {
	return f.imageRef, nil
}

func (f *fakeRegistry) PushFile(_ context.Context, repository, tag, artifactType, mediaType, filename string, content []byte) (string, error) {
	f.fileRepo, f.fileTag, f.fileArtifactType, f.fileMediaType, f.fileName, f.fileContent = repository, tag, artifactType, mediaType, filename, content
	return f.fileRef, nil
}

func uploadMeta(taskID int32, ref *ArtifactRef) *ArtifactUploadMeta {
	m := &ArtifactUploadMeta{}
	m.SetTaskId(taskID)
	m.SetArtifact(ref)
	return m
}

func TestStoreArtifact(t *testing.T) {
	content := []byte("hello world")

	t.Run("binary success records a row", func(t *testing.T) {
		reg := &fakeRegistry{fileRef: "sha256:stored"}
		m := daotest.NewArtifact()
		s := &apiService{dao: dao.Options{Artifact: m}, artifactRegistry: reg}

		sum := sha256.Sum256(content)
		hash := hex.EncodeToString(sum[:])
		ref := artifactRef(ArtifactType_ARTIFACT_TYPE_BINARY, hash)
		ref.SetRepository("example.com/foo")
		ref.SetName("foo")
		res, err := s.storeArtifact(context.Background(), uploadMeta(7, ref), content)
		assertion.MustNoError(t, err)
		assertion.Equal(t, "example.com/foo", res.GetRepository())
		assertion.Equal(t, "sha256:stored", res.GetReference())
		assertion.Equal(t, "application/vnd.mono.build.binary", reg.fileArtifactType)
		assertion.Equal(t, "application/octet-stream", reg.fileMediaType)
		assertion.Equal(t, content, reg.fileContent)

		called := m.Called("Create")
		assertion.Len(t, called, 1)
		stored := called[0].Args["artifact"].(*database.Artifact)
		assertion.Equal(t, hash, stored.Hash)
		assertion.Equal(t, int32(7), stored.TaskId)
		assertion.Equal(t, database.ArtifactTypeBinary, stored.Type)
	})

	t.Run("hash mismatch is rejected", func(t *testing.T) {
		reg := &fakeRegistry{fileRef: "sha256:stored"}
		m := daotest.NewArtifact()
		s := &apiService{dao: dao.Options{Artifact: m}, artifactRegistry: reg}

		ref := artifactRef(ArtifactType_ARTIFACT_TYPE_BINARY, "deadbeef")
		ref.SetRepository("example.com/foo")
		_, err := s.storeArtifact(context.Background(), uploadMeta(7, ref), content)
		assertion.Error(t, err)
		assertion.Len(t, m.Called("Create"), 0) // nothing recorded
	})
}

func TestCollectUpload(t *testing.T) {
	ref := artifactRef(ArtifactType_ARTIFACT_TYPE_BINARY, "h")
	msgs := []*RequestUploadArtifact{
		uploadMetaMsg(uploadMeta(7, ref)),
		uploadChunkMsg([]byte("hello ")),
		uploadChunkMsg([]byte("world")),
	}
	i := 0
	recv := func() (*RequestUploadArtifact, error) {
		if i >= len(msgs) {
			return nil, io.EOF
		}
		m := msgs[i]
		i++
		return m, nil
	}
	meta, data, err := collectUpload(recv)
	assertion.MustNoError(t, err)
	assertion.Equal(t, int32(7), meta.GetTaskId())
	assertion.Equal(t, "hello world", string(data))
}

func TestCollectUpload_chunkBeforeMeta(t *testing.T) {
	msgs := []*RequestUploadArtifact{uploadChunkMsg([]byte("x"))}
	i := 0
	recv := func() (*RequestUploadArtifact, error) {
		if i >= len(msgs) {
			return nil, io.EOF
		}
		m := msgs[i]
		i++
		return m, nil
	}
	_, _, err := collectUpload(recv)
	assertion.Error(t, err)
}

func uploadMetaMsg(m *ArtifactUploadMeta) *RequestUploadArtifact {
	r := &RequestUploadArtifact{}
	r.SetMeta(m)
	return r
}

func uploadChunkMsg(b []byte) *RequestUploadArtifact {
	r := &RequestUploadArtifact{}
	r.SetChunk(b)
	return r
}

func TestCraneArtifactRegistry_insecure(t *testing.T) {
	cases := []struct {
		insecure bool
		scheme   string
	}{
		{insecure: false, scheme: "https"},
		{insecure: true, scheme: "http"},
	}
	for _, tc := range cases {
		r := NewCraneArtifactRegistry(tc.insecure).(*craneArtifactRegistry)
		ref, err := name.ParseReference("zot.zot.svc.cluster.local/build/foo:latest", crane.GetOptions(r.options(context.Background())...).Name...)
		assertion.MustNoError(t, err)
		assertion.Equal(t, tc.scheme, ref.Context().Scheme())
	}
}

func TestCraneArtifactRegistry_PushFile(t *testing.T) {
	s := httptest.NewServer(registry.New())
	t.Cleanup(s.Close)
	host := strings.TrimPrefix(s.URL, "http://")
	r := NewCraneArtifactRegistry(true)

	content := []byte("kind: Deployment\n")
	ref, err := r.PushFile(context.Background(), host+"/build/manifests", "", "application/vnd.mono.build.k8s-manifest", "application/yaml", "kustomize.base.yaml", content)
	assertion.MustNoError(t, err)

	raw, err := crane.Manifest(host+"/build/manifests@"+ref, crane.Insecure)
	assertion.MustNoError(t, err)
	var m v1.Manifest
	assertion.MustNoError(t, json.Unmarshal(raw, &m))
	assertion.Equal(t, m.MediaType, types.OCIManifestSchema1)
	assertion.Equal(t, m.ArtifactType, "application/vnd.mono.build.k8s-manifest")
	assertion.Equal(t, m.Config.MediaType, types.MediaType("application/vnd.oci.empty.v1+json"))
	assertion.Len(t, m.Layers, 1)
	assertion.Equal(t, m.Layers[0].MediaType, types.MediaType("application/yaml"))
	assertion.Equal(t, m.Layers[0].Annotations["org.opencontainers.image.title"], "kustomize.base.yaml")
	sum := sha256.Sum256(content)
	assertion.Equal(t, m.Layers[0].Digest.String(), "sha256:"+hex.EncodeToString(sum[:]))
}
