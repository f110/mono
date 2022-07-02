package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandler(t *testing.T) {
	h := newHttpHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/test1/feature/update-doc/-/docs/README.md", nil)
	repo, ref, filepath := h.parsePath(req)
	assert.Equal(t, "test1", repo)
	assert.Equal(t, "feature/update-doc", ref)
	assert.Equal(t, "docs/README.md", filepath)
}
