package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler(t *testing.T) {
	h := newHttpHandler()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/test1/feature/update-doc/-/docs/README.md", nil)
	h.ServeHTTP(nil, req)
}
