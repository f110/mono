package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadConfig(t *testing.T) {
	data := `job(
    name = "test_all",
    event = ["push"],
    all_revision = True,
    command = "test",
    cpu_limit = "2000m",
    github_status = True,
    memory_limit = "8Gi",
    platforms = [
        "@io_bazel_rules_go//go/toolchain:linux_amd64",
    ],
    targets = [
        "//...",
        "-//vendor/github.com/JuulLabs-OSS/cbgo:cbgo",
        "-//third_party/universal-ctags/ctags:ctags",
        "-//containers/zoekt-indexer/...",
        "-//vendor/github.com/go-enry/go-oniguruma/...",
    ],
    args = ["--verbose"],
	exclusive = True,
	config_name = "ci",
    secrets = [
        secret(env_name = "GITHUB_API", vault_path = "/secrets/github", vault_key = "token"),
        secret(env_name = "ACCESS_KEY", vault_path = "/secrets/provider", vault_key = "access_key"),
        secret(mount_path = "/var/vault/provider", vault_path = "/secrets/provider", vault_key = "secret_key"),
    ],
)`

	conf, err := Read(strings.NewReader(data), "", "")
	require.NoError(t, err)
	require.Len(t, conf.Jobs, 1)

	assert.Equal(t, "test_all", conf.Jobs[0].Name)
	assert.Equal(t, []EventType{EventPush}, conf.Jobs[0].Event)
	assert.True(t, conf.Jobs[0].AllRevision)
	assert.True(t, conf.Jobs[0].GitHubStatus)
	assert.Equal(t, "test", conf.Jobs[0].Command)
	assert.Equal(t, "2000m", conf.Jobs[0].CPULimit)
	assert.Equal(t, "8Gi", conf.Jobs[0].MemoryLimit)
	assert.Equal(t, "ci", conf.Jobs[0].ConfigName)
	assert.True(t, conf.Jobs[0].Exclusive)
	assert.Equal(t, []string{"@io_bazel_rules_go//go/toolchain:linux_amd64"}, conf.Jobs[0].Platforms)
	assert.Equal(t,
		[]string{
			"//...",
			"-//vendor/github.com/JuulLabs-OSS/cbgo:cbgo",
			"-//third_party/universal-ctags/ctags:ctags",
			"-//containers/zoekt-indexer/...",
			"-//vendor/github.com/go-enry/go-oniguruma/...",
		},
		conf.Jobs[0].Targets,
	)

	require.Len(t, conf.Jobs[0].Secrets, 3)
	assert.Equal(t, conf.Jobs[0].Secrets[0].EnvVarName, "GITHUB_API")
	assert.Equal(t, conf.Jobs[0].Secrets[0].VaultPath, "/secrets/github")
	assert.Equal(t, conf.Jobs[0].Secrets[0].VaultKey, "token")
	assert.Equal(t, conf.Jobs[0].Secrets[1].EnvVarName, "ACCESS_KEY")
	assert.Equal(t, conf.Jobs[0].Secrets[1].VaultPath, "/secrets/provider")
	assert.Equal(t, conf.Jobs[0].Secrets[1].VaultKey, "access_key")
	assert.Equal(t, conf.Jobs[0].Secrets[2].MountPath, "/var/vault/provider")
	assert.Equal(t, conf.Jobs[0].Secrets[2].VaultPath, "/secrets/provider")
	assert.Equal(t, conf.Jobs[0].Secrets[2].VaultKey, "secret_key")
}

func TestRead_AllRequiredFieldsAreNotPresent(t *testing.T) {
	data := `job(
    name = "test_all",
    all_revision = True,
    command = "test",
    cpu_limit = "2000m",
    github_status = True,
    memory_limit = "8Gi",
    targets = [
        "//...",
        "-//vendor/github.com/JuulLabs-OSS/cbgo:cbgo",
        "-//third_party/universal-ctags/ctags:ctags",
        "-//containers/zoekt-indexer/...",
        "-//vendor/github.com/go-enry/go-oniguruma/...",
    ],
	exclusive = True,
	config_name = "ci",
)`

	_, err := Read(strings.NewReader(data), "", "")
	require.Error(t, err)
}

func TestJob(t *testing.T) {
	valid := &Job{
		Name:      t.Name(),
		Event:     []EventType{EventPush},
		Command:   "test",
		Platforms: []string{"@io_bazel_rules_go//go/toolchain:linux_amd64"},
		Targets:   []string{"//:test"},
	}

	cases := []struct {
		Job func(*Job)
	}{
		{Job: func(j *Job) { j.Name = "" }},
		{Job: func(j *Job) { j.Event = nil }},
		{Job: func(j *Job) { j.Command = "" }},
		{Job: func(j *Job) { j.Command = "get" }},
		{Job: func(j *Job) { j.Platforms = nil }},
		{Job: func(j *Job) { j.Targets = nil }},
		{Job: func(j *Job) { j.Command = "test"; j.Args = []string{"--verbose"} }},
		{Job: func(j *Job) { j.Command = "run"; j.Targets = []string{"//:test", "//:run"} }},
	}

	require.Nil(t, valid.IsValid())
	buf, err := json.Marshal(valid)
	require.NoError(t, err)
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			var j Job
			err = json.Unmarshal(buf, &j)
			require.NoError(t, err)
			tc.Job(&j)
			assert.NotNil(t, j.IsValid())
		})
	}
}
