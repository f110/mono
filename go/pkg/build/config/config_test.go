package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadConfig(t *testing.T) {
	data := `job(
    name = "test_all",
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
	exclusive = True,
	config_name = "ci",
)`

	conf, err := Read(strings.NewReader(data), "", "")
	require.NoError(t, err)

	assert.Equal(t, "test_all", conf.Jobs[0].Name)
	assert.True(t, conf.Jobs[0].AllRevision)
	assert.True(t, conf.Jobs[0].GitHubStatus)
	assert.Equal(t, "test", conf.Jobs[0].Command)
	assert.Equal(t, "2000m", conf.Jobs[0].CPULimit)
	assert.Equal(t, "8Gi", conf.Jobs[0].MemoryLimit)
	assert.Equal(t, "ci", conf.Jobs[0].ConfigName)
	assert.True(t, conf.Jobs[0].Exclusive)
	if assert.Len(t, conf.Jobs[0].Platforms, 1) {
		assert.Equal(t, "@io_bazel_rules_go//go/toolchain:linux_amd64", conf.Jobs[0].Platforms[0])
	}
	if assert.Len(t, conf.Jobs[0].Targets, 5) {
		assert.Equal(t, "//...", conf.Jobs[0].Targets[0])
		assert.Equal(t, "-//vendor/github.com/JuulLabs-OSS/cbgo:cbgo", conf.Jobs[0].Targets[1])
		assert.Equal(t, "-//third_party/universal-ctags/ctags:ctags", conf.Jobs[0].Targets[2])
		assert.Equal(t, "-//containers/zoekt-indexer/...", conf.Jobs[0].Targets[3])
		assert.Equal(t, "-//vendor/github.com/go-enry/go-oniguruma/...", conf.Jobs[0].Targets[4])
	}
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
