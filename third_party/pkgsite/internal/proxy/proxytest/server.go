// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proxytest

import (
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"
	"golang.org/x/pkgsite/internal/proxy"
	"golang.org/x/pkgsite/internal/testing/testhelper"
	"golang.org/x/pkgsite/internal/version"
)

// Server represents a proxy server containing the specified modules.
type Server struct {
	mu          sync.Mutex
	modules     map[string][]*Module
	mux         *http.ServeMux
	zipRequests int // number of .zip endpoint requests, for testing
}

// NewServer returns a proxy Server that serves the provided modules.
func NewServer(modules []*Module) *Server {
	s := &Server{
		mux:     http.NewServeMux(),
		modules: map[string][]*Module{},
	}
	for _, m := range modules {
		s.AddModule(m)
	}
	return s
}

// handleInfo creates an info endpoint for the specified module version.
func (s *Server) handleInfo(modulePath, resolvedVersion string, uncached bool) {
	urlPath := fmt.Sprintf("/%s/@v/%s.info", modulePath, resolvedVersion)
	s.mux.HandleFunc(urlPath, func(w http.ResponseWriter, r *http.Request) {
		if uncached && r.Header.Get(proxy.DisableFetchHeader) == "true" {
			http.Error(w, "not found: temporarily unavailable", http.StatusGone)
			return
		}
		http.ServeContent(w, r, modulePath, time.Now(), defaultInfo(resolvedVersion))
	})
}

// handleLatest creates an info endpoint for the specified module at the latest
// version.
func (s *Server) handleLatest(modulePath, urlPath string) {
	s.mux.HandleFunc(urlPath, func(w http.ResponseWriter, r *http.Request) {
		modules := s.modules[modulePath]
		resolvedVersion := modules[len(modules)-1].Version
		http.ServeContent(w, r, modulePath, time.Now(), defaultInfo(resolvedVersion))
	})
}

// handleMod creates a mod endpoint for the specified module version.
func (s *Server) handleMod(m *Module) {
	defaultGoMod := func(modulePath string) string {
		// defaultGoMod creates a bare-bones go.mod contents.
		return fmt.Sprintf("module %s\n\ngo 1.12", modulePath)
	}
	goMod := m.Files["go.mod"]
	if goMod == "" {
		goMod = defaultGoMod(m.ModulePath)
	}
	s.mux.HandleFunc(fmt.Sprintf("/%s/@v/%s.mod", m.ModulePath, m.Version),
		func(w http.ResponseWriter, r *http.Request) {
			http.ServeContent(w, r, m.ModulePath, time.Now(), strings.NewReader(goMod))
		})
}

// handleZip creates a zip endpoint for the specified module version.
func (s *Server) handleZip(m *Module) {
	s.mux.HandleFunc(fmt.Sprintf("/%s/@v/%s.zip", m.ModulePath, m.Version),
		func(w http.ResponseWriter, r *http.Request) {
			s.mu.Lock()
			s.zipRequests++
			s.mu.Unlock()
			http.ServeContent(w, r, m.ModulePath, time.Now(), bytes.NewReader(m.zip))
		})
}

// handleList creates a list endpoint for the specified modulePath.
func (s *Server) handleList(modulePath string) {
	s.mux.HandleFunc(fmt.Sprintf("/%s/@v/list", modulePath), func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()

		var vList []string
		if modules, ok := s.modules[modulePath]; ok {
			for _, v := range modules {
				if !version.IsPseudo(v.Version) {
					vList = append(vList, v.Version)
				}
			}
		}
		http.ServeContent(w, r, modulePath, time.Now(), strings.NewReader(strings.Join(vList, "\n")))
	})
}

// AddRoute adds an additional handler to the server.
func (s *Server) AddRoute(route string, fn func(w http.ResponseWriter, r *http.Request)) {
	s.mux.HandleFunc(route, fn)
}

// AddModule adds an additional module to the server.
func (s *Server) AddModule(m *Module) {
	s.addModule(m, true)
}

// AddModuleNoVersions adds a module to the server, but the @v/list endpoint will
// return nothing and @latest endpoint will serve a 410.
// For testing the unusual case where a module exists but there is no version information.
func (s *Server) AddModuleNoVersions(m *Module) {
	s.addModule(m, false)
}

func (s *Server) addModule(m *Module, hasVersions bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	m = cleanModule(m)

	if _, ok := s.modules[m.ModulePath]; !ok {
		if hasVersions {
			s.handleList(m.ModulePath)
			s.handleLatest(m.ModulePath, fmt.Sprintf("/%s/@latest", m.ModulePath))
			// TODO(https://golang.org/issue/39985): Add endpoint for handling
			// master and main versions.
			if m.Version != "master" {
				s.handleLatest(m.ModulePath, fmt.Sprintf("/%s/@v/master.info", m.ModulePath))
			}
			if m.Version != "main" {
				s.handleLatest(m.ModulePath, fmt.Sprintf("/%s/@v/main.info", m.ModulePath))
			}
		} else {
			s.mux.HandleFunc(fmt.Sprintf("/%s/@v/list", m.ModulePath), func(w http.ResponseWriter, r *http.Request) {
				http.ServeContent(w, r, m.ModulePath, time.Now(), strings.NewReader(""))
			})
			s.mux.HandleFunc(fmt.Sprintf("/%s/@latest", m.ModulePath), func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "not found", http.StatusGone)
			})
		}
	}
	s.handleInfo(m.ModulePath, m.Version, m.NotCached)
	s.handleMod(m)
	s.handleZip(m)

	s.modules[m.ModulePath] = append(s.modules[m.ModulePath], m)
	sort.Slice(s.modules[m.ModulePath], func(i, j int) bool {
		// Return the modules in order of decreasing semver.
		return semver.Compare(s.modules[m.ModulePath][i].Version, s.modules[m.ModulePath][j].Version) < 0
	})
}

func (s *Server) ZipRequests() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.zipRequests
}

// CommitTime is the time returned by all calls to the .info endpoint.
var CommitTime = time.Date(2019, time.January, 30, 0, 0, 0, 0, time.UTC)

func cleanModule(m *Module) *Module {
	if m.Version == "" {
		m.Version = "v1.0.0"
	}

	files := map[string]string{}
	for path, contents := range m.Files {
		p := m.ModulePath + "@" + m.Version + "/" + path
		files[p] = contents
	}
	zip, err := testhelper.ZipContents(files)
	if err != nil {
		panic(err)
	}
	m.zip = zip
	return m
}

func defaultInfo(resolvedVersion string) *strings.Reader {
	return strings.NewReader(fmt.Sprintf("{\n\t\"Version\": %q,\n\t\"Time\": %q\n}",
		resolvedVersion, CommitTime.Format(time.RFC3339)))
}
