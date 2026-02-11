package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJob(t *testing.T) {
	valid := &Job{
		Name:      t.Name(),
		Event:     []EventType{EventPush},
		Command:   "test",
		Platforms: []string{"@rules_go//go/toolchain:linux_amd64"},
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
        "@rules_go//go/toolchain:linux_amd64",
    ],
    secrets = [
        registry_secret(host = "registry.f110.dev", vault_mount = "secrets", vault_path = "registry.f110.dev/build", vault_key = "robot"),
    ],
    cpu_limit = "2000m",
    event = ["manual"],
	args = ["--verbose"],
)`
	config, err := Read(strings.NewReader(raw), "", "")
	require.NoError(t, err)
	job := config.Jobs[0]

	encoded, err := MarshalJob(job)
	require.NoError(t, err)

	decodedJob := &JobV2{}
	err = UnmarshalJobV2(encoded, decodedJob, "", "")
	require.NoError(t, err)
	assert.Equal(t, "publish_zoekt_indexer", decodedJob.Name)
	if assert.IsType(t, &Secret{}, decodedJob.Secrets[0]) {
		assert.Equal(t, "registry.f110.dev", decodedJob.Secrets[0].Host)
		assert.Equal(t, "secrets", decodedJob.Secrets[0].VaultMount)
		assert.Equal(t, "registry.f110.dev/build", decodedJob.Secrets[0].VaultPath)
		assert.Equal(t, "robot", decodedJob.Secrets[0].VaultKey)
		assert.Equal(t, []string{"--verbose"}, decodedJob.Args)
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
        "@rules_go//go/toolchain:linux_amd64",
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
	decodedJobV2 := &JobV2{}
	err = UnmarshalJobV2(gobJob, decodedJobV2, "", "")
	require.NoError(t, err)
	assert.Equal(t, "test_all", decodedJobV2.Name)
}
