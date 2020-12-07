package discovery

import (
	"io/ioutil"
	"testing"

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
