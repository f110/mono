// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package worker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/google/safehtml/template"
	"golang.org/x/pkgsite/internal"
	"golang.org/x/pkgsite/internal/config"
	"golang.org/x/pkgsite/internal/derrors"
	"golang.org/x/pkgsite/internal/log"
	"golang.org/x/pkgsite/internal/memory"
	"golang.org/x/pkgsite/internal/middleware"
	"golang.org/x/pkgsite/internal/postgres"
	"golang.org/x/sync/errgroup"
)

type annotation struct {
	error
	msg string
}

var startTime = time.Now()

// doIndexPage writes the status page. On error it returns the error and a short
// string to be written back to the client.
func (s *Server) doIndexPage(w http.ResponseWriter, r *http.Request) (err error) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return nil
	}
	defer derrors.Wrap(&err, "doIndexPage")
	var experiments []*internal.Experiment
	if s.getExperiments != nil {
		experiments = s.getExperiments()
	}
	ctx := r.Context()
	gms := memory.ReadRuntimeStats()
	sms, err := memory.ReadSystemStats()
	if err != nil {
		log.Errorf(ctx, "could not get system stats: %v", err)
	}
	pms, err := memory.ReadProcessStats()
	if err != nil {
		log.Errorf(ctx, "could not get process stats: %v", err)
	}
	cms, err := memory.ReadCgroupStats()
	if err != nil {
		log.Warningf(ctx, "could not get cgroup stats: %v", err)
	}

	// Display requests that aren't fetches separately.
	// Don't include the request for this page itself.
	var nonFetchRequests []*internal.RequestInfo
	for _, ri := range middleware.ActiveRequests() {
		if ri.TrimmedURL != "/" && !strings.HasPrefix(ri.TrimmedURL, "/fetch/") {
			nonFetchRequests = append(nonFetchRequests, ri)
		}
	}

	page := struct {
		Config          *config.Config
		Env             string
		ResourcePrefix  string
		LatestTimestamp *time.Time
		LocationID      string
		Hostname        string
		StartTime       time.Time
		Experiments     []*internal.Experiment
		LoadShedStats   LoadShedStats
		GoMemStats      runtime.MemStats
		ProcessStats    memory.ProcessStats
		SystemStats     memory.SystemStats
		CgroupStats     map[string]uint64
		Fetches         []*FetchInfo
		OtherRequests   []*internal.RequestInfo
		DBInfo          *postgres.UserInfo
	}{
		Config:         s.cfg,
		Env:            env(s.cfg),
		ResourcePrefix: strings.ToLower(env(s.cfg)) + "-",
		LocationID:     s.cfg.LocationID,
		Hostname:       os.Getenv("HOSTNAME"),
		StartTime:      startTime,
		Experiments:    experiments,
		LoadShedStats:  s.ZipLoadShedStats(),
		GoMemStats:     gms,
		ProcessStats:   pms,
		SystemStats:    sms,
		CgroupStats:    cms,
		Fetches:        FetchInfos(),
		OtherRequests:  nonFetchRequests,
		DBInfo:         s.workerDBInfo(),
	}
	return renderPage(ctx, w, page, s.templates[indexTemplate])
}

func (s *Server) doVersionsPage(w http.ResponseWriter, r *http.Request) (err error) {
	defer derrors.Wrap(&err, "doVersionsPage")
	const pageSize = 20
	g, ctx := errgroup.WithContext(r.Context())
	var (
		next, failures, recents []*internal.ModuleVersionState
		stats                   *postgres.VersionStats
	)
	g.Go(func() error {
		var err error
		next, err = s.db.GetNextModulesToFetch(ctx, pageSize)
		if err != nil {
			return annotation{err, "error fetching next versions"}
		}
		return nil
	})
	g.Go(func() error {
		var err error
		failures, err = s.db.GetRecentFailedVersions(ctx, pageSize)
		if err != nil {
			return annotation{err, "error fetching recent failures"}
		}
		return nil
	})
	g.Go(func() error {
		var err error
		recents, err = s.db.GetRecentVersions(ctx, pageSize)
		if err != nil {
			return annotation{err, "error fetching recent versions"}
		}
		return nil
	})
	g.Go(func() error {
		var err error
		stats, err = s.db.GetVersionStats(ctx)
		if err != nil {
			return annotation{err, "error fetching stats"}
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		var e annotation
		if errors.As(err, &e) {
			log.Errorf(ctx, e.msg, err)
		}
		return err
	}

	type count struct {
		Code  int
		Desc  string
		Count int
	}
	var counts []*count
	for code, n := range stats.VersionCounts {
		c := &count{Code: code, Count: n}
		if e := derrors.FromStatus(code, ""); e != nil && e != derrors.Unknown {
			c.Desc = e.Error()
		}
		counts = append(counts, c)
	}
	sort.Slice(counts, func(i, j int) bool { return counts[i].Code < counts[j].Code })
	page := struct {
		Next, Recent, RecentFailures []*internal.ModuleVersionState
		Config                       *config.Config
		Env                          string
		ResourcePrefix               string
		LatestTimestamp              *time.Time
		Counts                       []*count
	}{
		Next:            next,
		Recent:          recents,
		RecentFailures:  failures,
		Config:          s.cfg,
		Env:             env(s.cfg),
		ResourcePrefix:  strings.ToLower(env(s.cfg)) + "-",
		LatestTimestamp: &stats.LatestTimestamp,
		Counts:          counts,
	}
	return renderPage(ctx, w, page, s.templates[versionsTemplate])
}
func (s *Server) doExcludedPage(w http.ResponseWriter, r *http.Request) (err error) {
	excluded, err := s.db.GetExcludedPatterns(r.Context())
	if err != nil {
		return annotation{err, "error fetching excluded"}
	}
	page := struct {
		Env      string
		Excluded []string
	}{
		Env:      env(s.cfg),
		Excluded: excluded,
	}
	return renderPage(r.Context(), w, page, s.templates[excludedTemplate])
}

func env(cfg *config.Config) string {
	e := cfg.DeploymentEnvironment()
	return strings.ToUpper(e[:1]) + e[1:]
}

func renderPage(ctx context.Context, w http.ResponseWriter, page any, tmpl *template.Template) (err error) {
	defer derrors.Wrap(&err, "renderPage")
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, page); err != nil {
		return err
	}
	if _, err := io.Copy(w, &buf); err != nil {
		log.Errorf(ctx, "Error copying buffer to ResponseWriter: %v", err)
		return err
	}
	return nil
}
