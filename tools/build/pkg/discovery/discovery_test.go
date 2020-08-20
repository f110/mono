package discovery

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiscovery(t *testing.T) {
	buf, err := ioutil.ReadFile("testdata/proto.json")
	if err != nil {
		t.Fatal(err)
	}
	job, err := Discovery(buf)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, job, 1)
	assert.Equal(t, "pkg", job[0].Target)
	assert.Equal(t, "//tools/unissh", job[0].Package)
	assert.True(t, job[0].AllRevision)
	assert.Equal(t, "build", job[0].Command)
	assert.True(t, job[0].GithubStatus)
	assert.True(t, job[0].Synchronized)
}

func TestDuplicateJob(t *testing.T) {
	buf, err := ioutil.ReadFile("testdata/duplicate.json")
	if err != nil {
		t.Fatal(err)
	}
	job, err := Discovery(buf)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, job, 1)
}
