package config

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"testing/fstest"

	"github.com/google/go-github/v85/github"
	"github.com/stretchr/testify/require"
	"go.f110.dev/githubmock"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/testing/assertion"
)

// fakeGitDataClient captures the repository key GetTree is called with. Every
// other method is inherited from the embedded interface and panics if used.
type fakeGitDataClient struct {
	git.GitDataClient
	treeRepo string
}

func (f *fakeGitDataClient) GetTree(_ context.Context, in *git.RequestGetTree, _ ...grpc.CallOption) (*git.ResponseGetTree, error) {
	f.treeRepo = in.GetRepo()
	return nil, errors.New("stub: stop here")
}

// ReadFromGitDataService must query git-data-service with the registered key
// (gitDataRepo), not the bare repoName.
func TestReadFromGitDataServiceUsesGitDataRepoKey(t *testing.T) {
	fake := &fakeGitDataClient{}
	_, err := ReadFromGitDataService(context.Background(), fake, "example/public-app", "example", "public-app", "HEAD")
	assertion.MustError(t, err)
	assertion.Equal(t, fake.treeRepo, "example/public-app")
}

func TestParseFile(t *testing.T) {
	cases := []struct {
		Name string
		File string
		Job  *JobV2
	}{
		{
			Name: "Valid",
			File: `jobs: test: {
	command: "run"
	targets: ["//..."]
	event: ["manual"]
	platforms: ["linux_amd64"]
	args: ["--verbose"]
	secrets: [
		{mount_path: "/path/to/secret", vault_mount: "secrets", vault_path: "secret/path", vault_key: "secret_key"},
		{host: "registry.f110.dev", vault_mount: "secrets", vault_path: "registry.f110.dev/build", vault_key: "robot"}
	]
}`,
			Job: &JobV2{
				Name:      "test",
				Command:   "run",
				Targets:   []string{"//..."},
				Event:     []EventType{EventManual},
				Platforms: []string{"linux_amd64"},
				Args:      []string{"--verbose"},
				Secrets: []*Secret{
					{VaultMount: "secrets", VaultPath: "secret/path", VaultKey: "secret_key", MountPath: "/path/to/secret"},
					{Host: "registry.f110.dev", VaultMount: "secrets", VaultPath: "registry.f110.dev/build", VaultKey: "robot"},
				},
			},
		},
		{
			Name: "Valid: test with cache_test_results",
			File: `jobs: test: {
	command: "test"
	targets: ["//..."]
	event: ["manual"]
	platforms: ["linux_amd64"]
	cache_test_results: true
}`,
			Job: &JobV2{
				Name:             "test",
				Command:          "test",
				Targets:          []string{"//..."},
				Event:            []EventType{EventManual},
				Platforms:        []string{"linux_amd64"},
				Args:             []string{},
				CacheTestResults: true,
			},
		},
		{
			Name: "Valid: run with params",
			File: `jobs: test: {
	command: "run"
	targets: ["//..."]
	event: ["manual"]
	platforms: ["linux_amd64"]
	params: version: {description: "Release version", required: true}
}`,
			Job: &JobV2{
				Name:      "test",
				Command:   "run",
				Targets:   []string{"//..."},
				Event:     []EventType{EventManual},
				Platforms: []string{"linux_amd64"},
				Params:    map[string]*Param{"version": {Description: "Release version", Required: true}},
			},
		},
		{
			Name: "Invalid: test with params",
			File: `jobs: test: {
	command: "test"
	targets: ["//..."]
	event: ["manual"]
	platforms: ["linux_amd64"]
	params: version: {required: true}
}`,
		},
		{
			Name: "Invalid: run with cache_test_results",
			File: `jobs: test: {
	command: "run"
	targets: ["//..."]
	event: ["manual"]
	platforms: ["linux_amd64"]
	cache_test_results: true
}`,
		},
		{
			Name: "Invalid: test with args",
			File: `jobs: test: {
	command: "test"
	targets: ["//..."]
	event: ["manual"]
	platforms: ["linux_amd64"]
	args: ["--verbose"]
}`,
		},
		{
			Name: "Invalid: no targets",
			File: `jobs: test: {
	command: "test"
	event: ["manual"]
	platforms: ["linux_amd64"]
}`,
		},
		{
			Name: "Invalid: no platforms",
			File: `jobs: test: {
	command: "test"
	targets: ["//..."]
	event: ["manual"]
}`,
		},
		{
			Name: "Invalid: no event",
			File: `jobs: test: {
	command: "run"
	targets: ["//..."]
	platforms: ["linux_amd64"]
	args: ["--verbose"]
}`,
		},
		{
			Name: "Invalid: run with multiple targets",
			File: `jobs: test: {
	command: "run"
	targets: ["//pkg1", "//pkg2"]
	platforms: ["linux_amd64"]
}`,
		},
		{
			Name: "Invalid: without mount_path",
			File: `jobs: test: {
	command: "run"
	targets: ["//..."]
	event: ["manual"]
	platforms: ["linux_amd64"]
	args: ["--verbose"]
	secrets: [
		{vault_mount: "secrets", vault_path: "secret/path", vault_key: "secret_key"}
	]
}`,
		},
		{
			Name: "Invalid: there are both mount_path and host",
			File: `jobs: test: {
	command: "run"
	targets: ["//..."]
	event: ["manual"]
	platforms: ["linux_amd64"]
	args: ["--verbose"]
	secrets: [
		{mount_path: "/path/to/secret", host: "registry.example.com", vault_mount: "secrets", vault_path: "secret/path", vault_key: "secret_key"}
	]
}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			mapFS := fstest.MapFS(map[string]*fstest.MapFile{
				".build/config.cue": {Data: []byte(tc.File)},
			})
			f, err := mapFS.Open(".build/config.cue")
			assertion.MustNoError(t, err)
			jobs, err := ParseFile(f)
			if tc.Job != nil {
				assertion.MustNoError(t, err)
				assertion.Equal(t, tc.Job, jobs[0])
			} else {
				if testing.Verbose() && err != nil {
					t.Log(err)
				}
				assertion.MustError(t, err)
			}
		})
	}
}

func TestParseArtifactPublishedRules(t *testing.T) {
	cases := []struct {
		Name  string
		File  string
		Rules []*ArtifactPublishedRule
	}{
		{
			Name: "Valid",
			File: `on_artifact_published: {
	issue_base: {
		artifact: "repo.center.x.f110.dev/issue/manifests"
		replace: path: "clusters/globemaster/issue/base/kustomize.base.yaml"
	}
	issue_server_dev: {
		artifact: "repo.center.x.f110.dev/issue/server"
		kustomize_image: path: "clusters/globemaster/issue/dev/kustomization.yaml"
	}
	issue_frontend_dev: {
		artifact: "repo.center.x.f110.dev/issue/frontend"
		kustomize_image: {
			path: "clusters/globemaster/issue/dev/kustomization.yaml"
			name: "issue-frontend"
		}
	}
}`,
			Rules: []*ArtifactPublishedRule{
				{
					Name:     "issue_base",
					Artifact: "repo.center.x.f110.dev/issue/manifests",
					Replace:  &ReplaceAction{Path: "clusters/globemaster/issue/base/kustomize.base.yaml"},
				},
				{
					Name:           "issue_frontend_dev",
					Artifact:       "repo.center.x.f110.dev/issue/frontend",
					KustomizeImage: &KustomizeImageAction{Path: "clusters/globemaster/issue/dev/kustomization.yaml", Name: "issue-frontend"},
				},
				{
					Name:     "issue_server_dev",
					Artifact: "repo.center.x.f110.dev/issue/server",
					KustomizeImage: &KustomizeImageAction{
						Path: "clusters/globemaster/issue/dev/kustomization.yaml",
						Name: "repo.center.x.f110.dev/issue/server",
					},
				},
			},
		},
		{
			Name: "Valid: with jobs",
			File: `jobs: test: {
	command: "test"
	targets: ["//..."]
	event: ["push"]
	platforms: ["linux_amd64"]
}
on_artifact_published: base: {
	artifact: "repo.example.com/manifests"
	replace: path: "base.yaml"
}`,
			Rules: []*ArtifactPublishedRule{
				{Name: "base", Artifact: "repo.example.com/manifests", Replace: &ReplaceAction{Path: "base.yaml"}},
			},
		},
		{
			Name: "Valid: no rules",
			File: `jobs: test: {
	command: "test"
	targets: ["//..."]
	event: ["push"]
	platforms: ["linux_amd64"]
}`,
			Rules: []*ArtifactPublishedRule{},
		},
		{
			Name: "Invalid: no artifact",
			File: `on_artifact_published: base: replace: path: "base.yaml"`,
		},
		{
			Name: "Invalid: no action",
			File: `on_artifact_published: base: artifact: "repo.example.com/manifests"`,
		},
		{
			Name: "Invalid: both actions",
			File: `on_artifact_published: base: {
	artifact: "repo.example.com/manifests"
	replace: path: "base.yaml"
	kustomize_image: path: "kustomization.yaml"
}`,
		},
		{
			Name: "Invalid: no path",
			File: `on_artifact_published: base: {
	artifact: "repo.example.com/server"
	kustomize_image: name: "server"
}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			mapFS := fstest.MapFS(map[string]*fstest.MapFile{
				".build/config.cue": {Data: []byte(tc.File)},
			})
			f, err := mapFS.Open(".build/config.cue")
			assertion.MustNoError(t, err)
			rules, err := ParseArtifactPublishedRules(f)
			if tc.Rules != nil {
				assertion.MustNoError(t, err)
				assertion.Equal(t, tc.Rules, rules)
			} else {
				if testing.Verbose() && err != nil {
					t.Log(err)
				}
				assertion.MustError(t, err)
			}
		})
	}
}

func TestReadArtifactPublishedRulesFromBuildDir(t *testing.T) {
	mapFS := fstest.MapFS(map[string]*fstest.MapFile{
		".build/a.cue": {Data: []byte(`on_artifact_published: a: {
	artifact: "repo.example.com/a"
	replace: path: "a.yaml"
}`)},
		".build/b.cue": {Data: []byte(`on_artifact_published: b: {
	artifact: "repo.example.com/b"
	kustomize_image: path: "kustomization.yaml"
}`)},
		".build/README.md": {Data: []byte("readme")},
	})
	rules, err := ReadArtifactPublishedRulesFromBuildDir(mapFS)
	assertion.MustNoError(t, err)
	assertion.Equal(t, []*ArtifactPublishedRule{
		{Name: "a", Artifact: "repo.example.com/a", Replace: &ReplaceAction{Path: "a.yaml"}},
		{Name: "b", Artifact: "repo.example.com/b", KustomizeImage: &KustomizeImageAction{Path: "kustomization.yaml", Name: "repo.example.com/b"}},
	}, rules)
}

func TestReadFromProvider(t *testing.T) {
	mapFS := fstest.MapFS(map[string]*fstest.MapFile{
		".bazelversion": {Data: []byte("8.0.0\n")},
		".build/artifact.cue": {Data: []byte(`on_artifact_published: a: {
	artifact: "repo.example.com/a"
	replace: path: "a.yaml"
}`)},
	})
	conf, err := readFromProvider(mapFS, "example", "app")
	assertion.MustNoError(t, err)
	assertion.Len(t, conf.Jobs, 0)
	assertion.Equal(t, []*ArtifactPublishedRule{
		{Name: "a", Artifact: "repo.example.com/a", Replace: &ReplaceAction{Path: "a.yaml"}},
	}, conf.ArtifactPublishedRules)
}

func TestNormalizeGitPath(t *testing.T) {
	// git-data-service expects tree paths without a leading slash and "" for
	// the repository root (ReadDir maps "" to "/"). Anything else makes
	// go-git's Tree/FindEntry fail with "path is not found".
	cases := []struct {
		In   string
		Want string
	}{
		{In: "", Want: ""},
		{In: ".", Want: ""},
		{In: "/", Want: ""},
		{In: ".build", Want: ".build"},
		{In: "/.build", Want: ".build"},
		{In: ".bazelversion", Want: ".bazelversion"},
		{In: ".build/config.cue", Want: ".build/config.cue"},
		{In: "/.build/config.cue", Want: ".build/config.cue"},
	}

	for _, tc := range cases {
		t.Run(tc.In, func(t *testing.T) {
			assertion.Equal(t, normalizeGitPath(tc.In), tc.Want)
		})
	}
}

func TestGithubProvider(t *testing.T) {
	ghMock := githubmock.NewMock()
	repo := ghMock.Repository("f110/gh-test")
	err := repo.Commits(githubmock.NewCommit().
		IsHead().
		Files(
			&githubmock.File{Name: ".build/test.cue", Body: []byte(`jobs: test_all: {}`)},
			&githubmock.File{Name: ".build/mirror.cue"},
		),
	)
	require.NoError(t, err)

	ghClient := github.NewClient(&http.Client{Transport: ghMock.Transport()})
	commit, _, err := ghClient.Git.GetCommit(t.Context(), "f110", "gh-test", "HEAD")
	require.NoError(t, err)
	provider, err := newGitHubProvider(t.Context(), ghClient, "f110", "gh-test", commit.GetTree().GetSHA())
	assertion.MustNoError(t, err)
	entries, err := provider.ReadDir(".build")
	assertion.MustNoError(t, err)
	assertion.Len(t, entries, 2)

	f, err := provider.Open(".build/test.cue")
	assertion.MustNoError(t, err)
	fileBody, err := io.ReadAll(f)
	assertion.MustNoError(t, err)
	assertion.Equal(t, []byte(`jobs: test_all: {}`), fileBody)
}
