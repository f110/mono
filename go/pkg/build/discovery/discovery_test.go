package discovery

import (
	"bytes"
	"context"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscovery(t *testing.T) {
	buf, err := ioutil.ReadFile("testdata/proto.json")
	require.NoError(t, err)
	job, err := Discovery(buf)
	require.NoError(t, err)

	require.Len(t, job, 1)
	assert.Equal(t, "pkg", job[0].Target)
	assert.Equal(t, "unissh", job[0].Name)
	assert.Equal(t, "//tools/unissh", job[0].Package)
	assert.True(t, job[0].AllRevision)
	assert.Equal(t, "build", job[0].Command)
	assert.True(t, job[0].GithubStatus)
	assert.True(t, job[0].Exclusive)
	assert.Equal(t, []string{"//..."}, job[0].Targets)
}

func TestDiscoveryScript(t *testing.T) {
	temp, err := template.New("").Parse(discoveryJobScript)
	require.NoError(t, err)

	t.Run("Success", func(t *testing.T) {
		dir := t.TempDir()
		err = ioutil.WriteFile(
			filepath.Join(dir, "bazel"),
			[]byte(`#!/usr/bin/env bash
echo "foobar"
echo "baz" >&2
exit 0`),
			0755,
		)
		require.NoError(t, err)

		buf := new(bytes.Buffer)
		err = temp.Execute(buf, struct {
			Bazel string
		}{
			Bazel: filepath.Join(dir, "bazel"),
		})
		require.NoError(t, err)

		out := new(bytes.Buffer)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		cmd := exec.CommandContext(ctx, "sh", "-c", buf.String())
		cmd.Stdout = out
		err = cmd.Run()
		require.NoError(t, err)
		assert.NoError(t, ctx.Err())
		cancel()

		assert.Equal(t, "foobar\n", out.String())
	})

	t.Run("Failure", func(t *testing.T) {
		dir := t.TempDir()
		err = ioutil.WriteFile(
			filepath.Join(dir, "bazel"),
			[]byte(`#!/usr/bin/env bash
echo "foobar"
echo "baz" >&2
exit 1`),
			0755,
		)
		require.NoError(t, err)

		buf := new(bytes.Buffer)
		err = temp.Execute(buf, struct {
			Bazel string
		}{
			Bazel: filepath.Join(dir, "bazel"),
		})
		require.NoError(t, err)

		out := new(bytes.Buffer)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		cmd := exec.CommandContext(ctx, "sh", "-c", buf.String())
		cmd.Stdout = out
		err = cmd.Run()
		require.Error(t, err)
		assert.NoError(t, ctx.Err())
		cancel()

		assert.Equal(t, "baz\n", out.String())
	})
}

func TestDuplicateJob(t *testing.T) {
	buf, err := ioutil.ReadFile("testdata/duplicate.json")
	require.NoError(t, err)
	job, err := Discovery(buf)
	require.NoError(t, err)

	assert.Len(t, job, 1)
}

func TestMultipleJob(t *testing.T) {
	buf, err := ioutil.ReadFile("testdata/sandbox.json")
	require.NoError(t, err)
	jobs, err := Discovery(buf)
	require.NoError(t, err)

	assert.Len(t, jobs, 2)
}
