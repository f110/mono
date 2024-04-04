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
    container = "example.com/bazel:latest",
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
	exclusive = True,
	config_name = "ci",
    secrets = [
        secret(mount_path = "/var/vault/provider", vault_mount = "secrets/", vault_path = "github", vault_key = "token"),
        secret(mount_path = "/var/vault/provider", vault_mount = "secrets/", vault_path = "provider", vault_key = "access_key"),
        secret(mount_path = "/var/vault/provider", vault_mount = "secrets/", vault_path = "provider", vault_key = "secret_key"),
		registry_secret(host = "index.docker.io", vault_mount = "secrets/", vault_path = "provider", vault_key = "password"),
    ],
	env = {
        "FOOBAR": "env var",
        "BAZ": 1,
    },
)`

	conf, err := Read(strings.NewReader(data), "", "")
	require.NoError(t, err)
	require.Len(t, conf.Jobs, 1)

	assert.Equal(t, "test_all", conf.Jobs[0].Name)
	assert.Equal(t, []EventType{EventPush}, conf.Jobs[0].Event)
	assert.True(t, conf.Jobs[0].AllRevision)
	assert.True(t, conf.Jobs[0].GitHubStatus)
	assert.Equal(t, "test", conf.Jobs[0].Command)
	assert.Equal(t, "example.com/bazel:latest", conf.Jobs[0].Container)
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
	assert.Contains(t, conf.Jobs[0].Env, "FOOBAR")
	assert.Equal(t, "env var", conf.Jobs[0].Env["FOOBAR"])
	assert.Contains(t, conf.Jobs[0].Env, "BAZ")

	require.Len(t, conf.Jobs[0].Secrets, 4)
	assert.Equal(t, conf.Jobs[0].Secrets[0].(*Secret).VaultMount, "secrets/")
	assert.Equal(t, conf.Jobs[0].Secrets[0].(*Secret).VaultPath, "github")
	assert.Equal(t, conf.Jobs[0].Secrets[0].(*Secret).VaultKey, "token")
	assert.Equal(t, conf.Jobs[0].Secrets[1].(*Secret).VaultMount, "secrets/")
	assert.Equal(t, conf.Jobs[0].Secrets[1].(*Secret).VaultPath, "provider")
	assert.Equal(t, conf.Jobs[0].Secrets[1].(*Secret).VaultKey, "access_key")
	assert.Equal(t, conf.Jobs[0].Secrets[2].(*Secret).MountPath, "/var/vault/provider")
	assert.Equal(t, conf.Jobs[0].Secrets[2].(*Secret).VaultMount, "secrets/")
	assert.Equal(t, conf.Jobs[0].Secrets[2].(*Secret).VaultPath, "provider")
	assert.Equal(t, conf.Jobs[0].Secrets[2].(*Secret).VaultKey, "secret_key")
	assert.Equal(t, conf.Jobs[0].Secrets[3].(*RegistrySecret).Host, "index.docker.io")
	assert.Equal(t, conf.Jobs[0].Secrets[3].(*RegistrySecret).VaultMount, "secrets/")
	assert.Equal(t, conf.Jobs[0].Secrets[3].(*RegistrySecret).VaultPath, "provider")
	assert.Equal(t, conf.Jobs[0].Secrets[3].(*RegistrySecret).VaultKey, "password")
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

func TestMarshalJob(t *testing.T) {
	raw := `job(
    name = "publish_zoekt_indexer",
    command = "run",
    container = "registry.f110.dev/tools/zoekt-indexer-builder:latest",
    targets = ["//containers/zoekt-indexer:push"],
    platforms = [
        "@io_bazel_rules_go//go/toolchain:linux_amd64",
    ],
    secrets = [
        registry_secret(host = "registry.f110.dev", vault_mount = "secrets", vault_path = "registry.f110.dev/build", vault_key = "robot"),
    ],
    cpu_limit = "2000m",
    event = ["manual"],
)`
	config, err := Read(strings.NewReader(raw), "", "")
	require.NoError(t, err)
	job := config.Jobs[0]

	encoded, err := MarshalJob(job)
	require.NoError(t, err)

	decodedJob := &Job{}
	err = UnmarshalJob(encoded, decodedJob)
	require.NoError(t, err)
	assert.Equal(t, "publish_zoekt_indexer", decodedJob.Name)
	if assert.IsType(t, &RegistrySecret{}, decodedJob.Secrets[0]) {
		assert.Equal(t, "registry.f110.dev", decodedJob.Secrets[0].(*RegistrySecret).Host)
		assert.Equal(t, "secrets", decodedJob.Secrets[0].(*RegistrySecret).VaultMount)
		assert.Equal(t, "registry.f110.dev/build", decodedJob.Secrets[0].(*RegistrySecret).VaultPath)
		assert.Equal(t, "robot", decodedJob.Secrets[0].(*RegistrySecret).VaultKey)
	}
}

func TestUnmarshalJob(t *testing.T) {
	raw := `job(
    name = "test_all",
    command = "test",
    all_revision = True,
    github_status = True,
    targets = [
        "//...",
        "-//vendor/github.com/JuulLabs-OSS/cbgo:cbgo",
        "-//third_party/universal-ctags/ctags:ctags",
        "-//containers/zoekt-indexer/...",
        "-//vendor/github.com/go-enry/go-oniguruma/...",
    ],
    platforms = [
        "@io_bazel_rules_go//go/toolchain:linux_amd64",
    ],
    cpu_limit = "2000m",
    memory_limit = "8192Mi",
    event = ["push"],
)`
	config, err := Read(strings.NewReader(raw), "", "")
	require.NoError(t, err)
	jsonJob, err := json.Marshal(config.Jobs[0])
	require.NoError(t, err)
	gobJob, err := MarshalJob(config.Jobs[0])
	require.NoError(t, err)

	decodedJob := &Job{}
	err = UnmarshalJob(jsonJob, decodedJob)
	require.NoError(t, err)
	assert.Equal(t, "test_all", decodedJob.Name)
	decodedJob = &Job{}
	err = UnmarshalJob(gobJob, decodedJob)
	require.NoError(t, err)
	assert.Equal(t, "test_all", decodedJob.Name)
}
