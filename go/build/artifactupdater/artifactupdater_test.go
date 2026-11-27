package artifactupdater

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/build/api"
	"go.f110.dev/mono/go/build/model"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/testing/assertion"
)

const (
	testKustomization = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../base

images:
  # server
  - name: repo.example.com/app/server
    digest: sha256:0000
  - name: repo.example.com/app/frontend
    digest: sha256:1111
`
)

func TestMain(m *testing.M) {
	logger.Init()
	client.InstallProtocol("file", server.DefaultServer)
	os.Exit(m.Run())
}

func TestUpdate(t *testing.T) {
	t.Run("KustomizeImage", func(t *testing.T) {
		remote := newRemote(t)
		buildClient := newFakeBuildClient(testRules(remote), imageArtifact(1, 10, "sha256:aaaa"))
		u := NewUpdater("bot", "bot@example.com", nil, buildClient, &fakeRegistry{})

		err := u.Update(t.Context(), event(1, 10))
		assertion.MustNoError(t, err)

		assertion.Equal(t, readRemoteFile(t, remote, "app/dev/kustomization.yaml"), `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../base

images:
  # server
  - name: repo.example.com/app/server
    digest: sha256:aaaa
  - name: repo.example.com/app/frontend
    digest: sha256:1111
`)
		assertion.Equal(t, headCommit(t, remote).Message, "Update repo.example.com/app/server\n\nArtifact-Rule: server_dev\nBuild-Task: 10\n")
	})

	t.Run("Replace", func(t *testing.T) {
		remote := newRemote(t)
		buildClient := newFakeBuildClient(testRules(remote), fileArtifact(2, 10, "sha256:bbbb"))
		registry := &fakeRegistry{files: map[string]string{"repo.example.com/app/manifests@sha256:bbbb": "kind: Deployment\n"}}
		u := NewUpdater("bot", "bot@example.com", nil, buildClient, registry)

		err := u.Update(t.Context(), event(2, 10))
		assertion.MustNoError(t, err)

		assertion.Equal(t, readRemoteFile(t, remote, "app/base/kustomize.base.yaml"), "kind: Deployment\n")
	})

	t.Run("SameEventTwice", func(t *testing.T) {
		remote := newRemote(t)
		buildClient := newFakeBuildClient(testRules(remote), imageArtifact(1, 10, "sha256:aaaa"))
		u := NewUpdater("bot", "bot@example.com", nil, buildClient, &fakeRegistry{})

		err := u.Update(t.Context(), event(1, 10))
		assertion.MustNoError(t, err)
		first := headCommit(t, remote).Hash

		err = u.Update(t.Context(), event(1, 10))
		assertion.MustNoError(t, err)
		assertion.Equal(t, headCommit(t, remote).Hash, first)
	})

	t.Run("OlderTask", func(t *testing.T) {
		remote := newRemote(t)
		buildClient := newFakeBuildClient(testRules(remote), imageArtifact(1, 10, "sha256:aaaa"), imageArtifact(2, 9, "sha256:9999"))
		u := NewUpdater("bot", "bot@example.com", nil, buildClient, &fakeRegistry{})

		err := u.Update(t.Context(), event(1, 10))
		assertion.MustNoError(t, err)
		first := headCommit(t, remote).Hash

		err = u.Update(t.Context(), event(2, 9))
		assertion.MustNoError(t, err)
		assertion.Equal(t, headCommit(t, remote).Hash, first)
	})

	t.Run("NonFastForward", func(t *testing.T) {
		remote := newRemote(t)
		buildClient := newFakeBuildClient(testRules(remote), fileArtifact(2, 10, "sha256:bbbb"))
		registry := &fakeRegistry{
			files: map[string]string{"repo.example.com/app/manifests@sha256:bbbb": "kind: Deployment\n"},
			onFetch: func(calls int) {
				if calls == 1 {
					pushFile(t, remote, "README.md", "readme\n")
				}
			},
		}
		u := NewUpdater("bot", "bot@example.com", nil, buildClient, registry)

		err := u.Update(t.Context(), event(2, 10))
		assertion.MustNoError(t, err)

		assertion.Equal(t, registry.calls, 2)
		assertion.Equal(t, readRemoteFile(t, remote, "README.md"), "readme\n")
		assertion.Equal(t, readRemoteFile(t, remote, "app/base/kustomize.base.yaml"), "kind: Deployment\n")
	})

	t.Run("MultipleRepositories", func(t *testing.T) {
		remote1 := newRemote(t)
		remote2 := newRemote(t)
		rules := append(testRules(remote1), testRules(remote2)...)
		u := NewUpdater("bot", "bot@example.com", nil, newFakeBuildClient(rules, imageArtifact(1, 10, "sha256:aaaa")), &fakeRegistry{})

		err := u.Update(t.Context(), event(1, 10))
		assertion.MustNoError(t, err)

		for _, remote := range []string{remote1, remote2} {
			assertion.Equal(t, headCommit(t, remote).Message, "Update repo.example.com/app/server\n\nArtifact-Rule: server_dev\nBuild-Task: 10\n")
		}
	})

	t.Run("NoRules", func(t *testing.T) {
		remote := newRemote(t)
		initial := headCommit(t, remote).Hash
		u := NewUpdater("bot", "bot@example.com", nil, newFakeBuildClient(nil, imageArtifact(1, 10, "sha256:aaaa")), &fakeRegistry{})

		err := u.Update(t.Context(), event(1, 10))
		assertion.MustNoError(t, err)
		assertion.Equal(t, headCommit(t, remote).Hash, initial)
	})

	t.Run("ReplaceWithContainerImage", func(t *testing.T) {
		remote := newRemote(t)
		artifact := imageArtifact(1, 10, "sha256:aaaa")
		artifact.SetRepository("repo.example.com/app/manifests")
		u := NewUpdater("bot", "bot@example.com", nil, newFakeBuildClient(testRules(remote), artifact), &fakeRegistry{})

		err := u.Update(t.Context(), event(1, 10))
		assertion.MustError(t, err)
	})

	t.Run("ArtifactNotFound", func(t *testing.T) {
		remote := newRemote(t)
		u := NewUpdater("bot", "bot@example.com", nil, newFakeBuildClient(testRules(remote)), &fakeRegistry{})

		err := u.Update(t.Context(), event(1, 10))
		assertion.MustError(t, err)
	})
}

func TestUpdateKustomizeImage(t *testing.T) {
	cases := []struct {
		Name string
		In   string
		Out  string
	}{
		{
			Name: "Plain",
			In:   testKustomization,
			Out: `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../base

images:
  # server
  - name: repo.example.com/app/server
    digest: sha256:ffff
  - name: repo.example.com/app/frontend
    digest: sha256:1111
`,
		},
		{
			Name: "Quoted",
			In:   "images:\n- digest: \"sha256:0000\" # current\n  name: repo.example.com/app/server\n",
			Out:  "images:\n- digest: \"sha256:ffff\" # current\n  name: repo.example.com/app/server\n",
		},
		{
			Name: "NotFound",
			In:   "images:\n- name: repo.example.com/app/frontend\n  digest: sha256:1111\n",
		},
		{
			Name: "NoDigest",
			In:   "images:\n- name: repo.example.com/app/server\n  newTag: v1\n",
		},
		{
			Name: "NoImages",
			In:   "resources:\n- ../base\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			out, err := updateKustomizeImage([]byte(tc.In), "repo.example.com/app/server", "sha256:ffff")
			if tc.Out == "" {
				assertion.MustError(t, err)
				return
			}
			assertion.MustNoError(t, err)
			assertion.Equal(t, string(out), tc.Out)
		})
	}
}

type fakeBuildClient struct {
	api.APIClient
	rules     []*model.ArtifactPublishedRule
	artifacts []*model.Artifact
}

func newFakeBuildClient(rules []*model.ArtifactPublishedRule, artifacts ...*model.Artifact) *fakeBuildClient {
	return &fakeBuildClient{rules: rules, artifacts: artifacts}
}

func (f *fakeBuildClient) ListArtifactPublishedRules(_ context.Context, in *api.RequestListArtifactPublishedRules, _ ...grpc.CallOption) (*api.ResponseListArtifactPublishedRules, error) {
	var rules []*model.ArtifactPublishedRule
	for _, v := range f.rules {
		if v.GetArtifact() == in.GetArtifact() {
			rules = append(rules, v)
		}
	}
	return api.ResponseListArtifactPublishedRules_builder{Rules: rules}.Build(), nil
}

func (f *fakeBuildClient) ListArtifacts(_ context.Context, in *api.RequestListArtifacts, _ ...grpc.CallOption) (*api.ResponseListArtifacts, error) {
	var artifacts []*model.Artifact
	for _, v := range f.artifacts {
		if v.GetTaskId() == in.GetTaskId() {
			artifacts = append(artifacts, v)
		}
	}
	return api.ResponseListArtifacts_builder{Artifacts: artifacts}.Build(), nil
}

type fakeRegistry struct {
	files   map[string]string
	onFetch func(calls int)
	calls   int
}

func (f *fakeRegistry) FetchFile(_ context.Context, repository, digest string) ([]byte, error) {
	f.calls++
	if f.onFetch != nil {
		f.onFetch(f.calls)
	}
	return []byte(f.files[repository+"@"+digest]), nil
}

func event(artifactID, taskID int32) *model.ArtifactPublishedEvent {
	return model.ArtifactPublishedEvent_builder{ArtifactId: new(artifactID), TaskId: new(taskID)}.Build()
}

func imageArtifact(id, taskID int32, hash string) *model.Artifact {
	return model.Artifact_builder{
		Id:         new(id),
		TaskId:     new(taskID),
		Type:       new(model.ArtifactType_ARTIFACT_TYPE_CONTAINER_IMAGE),
		Hash:       new(hash),
		Repository: new("repo.example.com/app/server"),
	}.Build()
}

func fileArtifact(id, taskID int32, hash string) *model.Artifact {
	return model.Artifact_builder{
		Id:         new(id),
		TaskId:     new(taskID),
		Type:       new(model.ArtifactType_ARTIFACT_TYPE_K8S_MANIFEST),
		Hash:       new(hash),
		Repository: new("repo.example.com/app/manifests"),
	}.Build()
}

// newRemote creates the bare repository which has the configuration and the kustomization, and returns the URL.
func newRemote(t *testing.T) string {
	remote := filepath.Join(t.TempDir(), "remote.git")
	_, err := git.PlainInit(remote, true)
	assertion.MustNoError(t, err)

	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	assertion.MustNoError(t, err)
	_, err = repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{remote}})
	assertion.MustNoError(t, err)
	files := map[string]string{
		"app/base/kustomize.base.yaml": "kind: ConfigMap\n",
		"app/dev/kustomization.yaml":   testKustomization,
	}
	tree, err := repo.Worktree()
	assertion.MustNoError(t, err)
	for path, content := range files {
		assertion.MustNoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0755))
		assertion.MustNoError(t, os.WriteFile(filepath.Join(dir, path), []byte(content), 0644))
		_, err := tree.Add(path)
		assertion.MustNoError(t, err)
	}
	_, err = tree.Commit("Initial commit", &git.CommitOptions{Author: testSignature()})
	assertion.MustNoError(t, err)
	assertion.MustNoError(t, repo.Push(&git.PushOptions{RefSpecs: []gitconfig.RefSpec{"refs/heads/master:refs/heads/master"}}))
	return remote
}

func pushFile(t *testing.T, remote, path, content string) {
	dir := t.TempDir()
	repo, err := git.PlainClone(dir, false, &git.CloneOptions{URL: remote})
	assertion.MustNoError(t, err)
	assertion.MustNoError(t, os.WriteFile(filepath.Join(dir, path), []byte(content), 0644))
	tree, err := repo.Worktree()
	assertion.MustNoError(t, err)
	_, err = tree.Add(path)
	assertion.MustNoError(t, err)
	_, err = tree.Commit("Add "+path, &git.CommitOptions{Author: testSignature()})
	assertion.MustNoError(t, err)
	assertion.MustNoError(t, repo.Push(&git.PushOptions{}))
}

func readRemoteFile(t *testing.T, remote, path string) string {
	c := headCommit(t, remote)
	f, err := c.File(path)
	assertion.MustNoError(t, err)
	content, err := f.Contents()
	assertion.MustNoError(t, err)
	return content
}

func headCommit(t *testing.T, remote string) *object.Commit {
	repo, err := git.PlainOpen(remote)
	assertion.MustNoError(t, err)
	ref, err := repo.Reference("refs/heads/master", true)
	assertion.MustNoError(t, err)
	c, err := repo.CommitObject(ref.Hash())
	assertion.MustNoError(t, err)
	return c
}

func testSignature() *object.Signature {
	return &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()}
}

// testRules returns the rules which the repository at remote defines.
func testRules(remote string) []*model.ArtifactPublishedRule {
	repo := model.Repository_builder{CloneUrl: new(remote)}.Build()
	return []*model.ArtifactPublishedRule{
		model.ArtifactPublishedRule_builder{
			Repository: repo,
			Name:       new("base"),
			Artifact:   new("repo.example.com/app/manifests"),
			Replace:    model.ArtifactPublishedRuleReplace_builder{Path: new("app/base/kustomize.base.yaml")}.Build(),
		}.Build(),
		model.ArtifactPublishedRule_builder{
			Repository: repo,
			Name:       new("server_dev"),
			Artifact:   new("repo.example.com/app/server"),
			KustomizeImage: model.ArtifactPublishedRuleKustomizeImage_builder{
				Path: new("app/dev/kustomization.yaml"),
				Name: new("repo.example.com/app/server"),
			}.Build(),
		}.Build(),
	}
}
