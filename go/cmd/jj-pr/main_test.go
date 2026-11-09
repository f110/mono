package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/logger"
)

func TestMain(m *testing.M) {
	logger.Init()
	m.Run()
}

func TestFindPullRequestTemplate(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".github/PULL_REQUEST_TEMPLATE"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".github/PULL_REQUEST_TEMPLATE.md"), nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".github/PULL_REQUEST_TEMPLATE/change.md"), nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".github/CODEOWNERS"), nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "docs/pull_request_template.md"), nil, 0644))

	c := newSubmitCommand()
	templates, err := c.findPullRequestTemplate(dir)
	require.NoError(t, err)
	assert.Len(t, templates, 3)
}
