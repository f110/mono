package bff

import (
	"testing"

	"go.f110.dev/mono/go/build/model"
	"go.f110.dev/mono/go/testing/assertion"
)

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
			Hash:      new("deadbeef"),         // files store a bare hex sha256
			Reference: new("sha256:deadbeef"),  // addressed by content digest
		}.Build()

		got := apiArtifactToBFFArtifact(a)
		assertion.Equal(t, "sha256:deadbeef", got.GetDigest())
		assertion.Equal(t, "", got.GetTag())
	})
}
