package config

import (
	"io"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/githubutil"
	"go.f110.dev/mono/go/testing/assertion"
)

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

func TestGithubProvider(t *testing.T) {
	ghMock := githubutil.NewMock()
	repo := ghMock.Repository("f110/gh-test")
	err := repo.Commits(&githubutil.Commit{
		IsHead: true,
		Files: []*githubutil.File{
			{Name: ".build/test.cue", Body: []byte(`jobs: test_all: {}`)},
			{Name: ".build/mirror.cue"},
		},
	})
	require.NoError(t, err)

	commit, _, err := ghMock.Client().Git.GetCommit(t.Context(), "f110", "gh-test", "HEAD")
	require.NoError(t, err)
	provider, err := newGitHubProvider(t.Context(), ghMock.Client(), "f110", "gh-test", commit.GetTree().GetSHA())
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
