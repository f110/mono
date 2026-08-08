package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"

	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/database/dao/daotest"
	"go.f110.dev/mono/go/testing/assertion"
)

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
	fileRepo, fileTag, fileMediaType, fileName string
	fileContent                                []byte
	fileRef                                    string
	imageRef                                   string
}

func (f *fakeRegistry) PushImage(_ context.Context, _, _, _ string, _ io.Reader) (string, error) {
	return f.imageRef, nil
}

func (f *fakeRegistry) PushFile(_ context.Context, repository, tag, mediaType, filename string, content []byte) (string, error) {
	f.fileRepo, f.fileTag, f.fileMediaType, f.fileName, f.fileContent = repository, tag, mediaType, filename, content
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
		assertion.Equal(t, "application/vnd.mono.build.binary", reg.fileMediaType)
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
