package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandler(t *testing.T) {
	cases := []struct {
		URL      string
		Repo     string
		Ref      string
		FilePath string
	}{
		{
			URL:      "http://example.com/test1/master/-/docs/README.md",
			Repo:     "test1",
			Ref:      "master",
			FilePath: "docs/README.md",
		},
		{
			URL:      "http://example.com/test1/feature/update-doc/-/docs/README.md",
			Repo:     "test1",
			Ref:      "feature/update-doc",
			FilePath: "docs/README.md",
		},
		{
			URL:      "http://example.com/test1/8e6e2933140691846d824231bde4af011200cf5a/-/docs/README.md",
			Repo:     "test1",
			Ref:      "8e6e2933140691846d824231bde4af011200cf5a",
			FilePath: "docs/README.md",
		},
	}

	h := newHttpHandler(nil)
	for _, tc := range cases {
		t.Run(tc.URL, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.URL, nil)
			repo, ref, filepath := h.parsePath(req)
			assert.Equal(t, tc.Repo, repo)
			assert.Equal(t, tc.Ref, ref)
			assert.Equal(t, tc.FilePath, filepath)
		})
	}
}
