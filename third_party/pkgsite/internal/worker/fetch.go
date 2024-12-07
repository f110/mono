// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package worker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
	"golang.org/x/mod/semver"
	"golang.org/x/pkgsite/internal"
	"golang.org/x/pkgsite/internal/cache"
	"golang.org/x/pkgsite/internal/dcensus"
	"golang.org/x/pkgsite/internal/derrors"
	"golang.org/x/pkgsite/internal/experiment"
	"golang.org/x/pkgsite/internal/fetch"
	"golang.org/x/pkgsite/internal/log"
	"golang.org/x/pkgsite/internal/log/stackdriverlogger"
	"golang.org/x/pkgsite/internal/postgres"
	"golang.org/x/pkgsite/internal/proxy"
	"golang.org/x/pkgsite/internal/source"
	"golang.org/x/pkgsite/internal/stdlib"
)

var (
	fetchesShedded = stats.Int64(
		"go-discovery/worker/fetch-shedded",
		"Count of shedded fetches.",
		stats.UnitDimensionless,
	)
	fetchLatency = stats.Float64(
		"go-discovery/worker/fetch-latency",
		"Latency of a fetch request.",
		stats.UnitSeconds,
	)
	fetchedPackages = stats.Int64(
		"go-discovery/worker/fetch-package-count",
		"Count of successfully fetched packages.",
		stats.UnitDimensionless,
	)

	// FetchLatencyDistribution aggregates frontend fetch request
	// latency by status code. It does not count shedded requests.
	FetchLatencyDistribution = &view.View{
		Name:        "go-discovery/worker/fetch-latency",
		Measure:     fetchLatency,
		Aggregation: ochttp.DefaultLatencyDistribution,
		Description: "Fetch latency by result status.",
		TagKeys:     []tag.Key{dcensus.KeyStatus},
	}
	// FetchResponseCount counts fetch responses by status.
	FetchResponseCount = &view.View{
		Name:        "go-discovery/worker/fetch-count",
		Measure:     fetchLatency,
		Aggregation: view.Count(),
		Description: "Fetch request count by result status",
		TagKeys:     []tag.Key{dcensus.KeyStatus},
	}
	// FetchPackageCount counts how many packages were successfully fetched.
	FetchPackageCount = &view.View{
		Name:        "go-discovery/worker/fetch-package-count",
		Measure:     fetchedPackages,
		Aggregation: view.Count(),
		Description: "Count of packages successfully fetched",
	}

	// SheddedFetchCount counts the number of fetches that were shedded.
	SheddedFetchCount = &view.View{
		Name:        "go-discovery/worker/fetch-shedded",
		Measure:     fetchesShedded,
		Aggregation: view.Count(),
		Description: "Count of shedded fetches",
	}
)

// fetchTask represents the result of a fetch task that was processed.
type fetchTask struct {
	fetch.FetchResult
	MainVersion   string
	MasterVersion string
	timings       map[string]time.Duration
}

// A Fetcher holds state for fetching modules.
type Fetcher struct {
	ProxyClient  *proxy.Client
	SourceClient *source.Client
	DB           *postgres.DB
	Cache        *cache.Cache
	loadShedder  *loadShedder
	Source       string
}

// FetchAndUpdateState fetches and processes a module version, and then updates
// the module_version_states table according to the result. It returns an HTTP
// status code representing the result of the fetch operation, and a non-nil
// error if this status code is not 200.
func (f *Fetcher) FetchAndUpdateState(ctx context.Context, modulePath, requestedVersion, appVersionLabel string) (status int, resolvedVersion string, err error) {
	defer derrors.Wrap(&err, "FetchAndUpdateState(%q, %q, %q)", modulePath, requestedVersion, appVersionLabel)
	tctx, span := trace.StartSpan(ctx, "FetchAndUpdateState")
	ctx = experiment.NewContext(tctx, experiment.FromContext(ctx).Active()...)
	ctx = stackdriverlogger.NewContextWithLabel(ctx, "fetch", modulePath+"@"+requestedVersion)

	start := time.Now()
	var nPackages int64
	defer func() {
		latency := float64(time.Since(start).Seconds())
		dcensus.RecordWithTag(ctx, dcensus.KeyStatus, strconv.Itoa(status), fetchLatency.M(latency))
		if status < 300 {
			stats.Record(ctx, fetchedPackages.M(nPackages))
		}
	}()

	if !utf8.ValidString(modulePath) {
		log.Errorf(ctx, "module path %q is not valid UTF-8", modulePath)
	}
	if modulePath == internal.UnknownModulePath {
		return http.StatusInternalServerError, "", errors.New("called with internal.UnknownModulePath")
	}
	if !utf8.ValidString(requestedVersion) {
		log.Errorf(ctx, "requested version %q is not valid UTF-8", requestedVersion)
	}
	span.AddAttributes(
		trace.StringAttribute("modulePath", modulePath),
		trace.StringAttribute("version", requestedVersion))
	defer span.End()

	// Begin by hitting the proxy's info endpoint. We need the resolved version
	// to do load-shedding, but it's also important to make the proxy aware
	// of the version if it isn't already, as can happen when we arrive here via
	// frontend fetch.
	//
	// Don't fail on a non-nil error. If we return here, we won't record
	// the error state in the DB.
	info, err := getInfo(ctx, modulePath, requestedVersion, f.ProxyClient)
	if err == nil {
		// If we're overloaded, shed load by not processing this module.
		// The zip endpoint requires a resolved version.
		deferFunc, zipSize, err := f.maybeShed(ctx, modulePath, info.Version)
		defer deferFunc()
		if err != nil {
			return derrors.ToStatus(err), "", err
		}

		fi := &FetchInfo{
			RequestInfo: internal.RequestInfoFromContext(ctx),
			ModulePath:  modulePath,
			Version:     requestedVersion,
			ZipSize:     uint64(zipSize),
			Start:       time.Now(),
		}
		startFetchInfo(fi)
		defer func() { finishFetchInfo(fi, status, err) }()

		// If this is a valid module, insert it into module_version_states.
		//
		// In case something happens later on, this will make sure we retry. Also,
		// modules that are introduced to pkgsite for the first time via frontend
		// fetch and not index.golang.org won't have a row in
		// module_version_states, so that ensures the logic below works properly as
		// well.
		//
		// Leave the index_timestamp as empty. This will be populated when the
		// module appears in the index.
		if err := f.DB.InsertNewModuleVersionFromFrontendFetch(ctx, modulePath, info.Version); err != nil {
			return derrors.ToStatus(err), "", err
		}
	}

	// Get the latest-version information first, and update the DB. We'll need
	// it to determine if the current module version is the latest good one for
	// its path.
	lmv, err := f.FetchAndUpdateLatest(ctx, modulePath)
	// The only errors possible here should be DB failures.
	if err != nil {
		return derrors.ToStatus(err), "", err
	}
	ft := f.fetchAndInsertModule(ctx, modulePath, requestedVersion, lmv)
	nPackages = int64(len(ft.PackageVersionStates))
	span.AddAttributes(trace.Int64Attribute("numPackages", nPackages))

	// If there were any errors processing the module then we didn't insert it.
	// Delete it in case we are reprocessing an existing module.
	// However, don't delete if the error was internal, or we are shedding load.
	if ft.Status >= 400 && ft.Status < 500 {
		if err := deleteModule(ctx, f.DB, ft); err != nil {
			log.Error(ctx, err)
			ft.Error = err
			ft.Status = http.StatusInternalServerError
		}
	}
	// Regardless of what the status code is, insert the result into
	// version_map, so that a response can be returned for frontend_fetch.
	if err := updateVersionMap(ctx, f.DB, ft); err != nil {
		log.Error(ctx, err)
		if ft.Status != http.StatusInternalServerError {
			ft.Error = err
			ft.Status = http.StatusInternalServerError
		}
		// Do not return an error here, because we want to insert into
		// module_version_states below.
	}
	if !semver.IsValid(ft.ResolvedVersion) {
		// If the requestedVersion was not successfully resolved to a semantic
		// version, then at this point it will be the same as the
		// resolvedVersion. This fetch request does not need to be recorded in
		// module_version_states, since that table is only used to track
		// modules that have been published to index.golang.org.
		return ft.Status, ft.ResolvedVersion, ft.Error
	}
	// Return an error here if a row does not exist in module_version_states.
	// This can happen if the source is frontend fetch, since we don't insert
	// rows to avoid cluttering module_version_states.
	if _, err := f.DB.GetModuleVersionState(ctx, modulePath, ft.ResolvedVersion); err != nil {
		if errors.Is(err, derrors.NotFound) {
			return ft.Status, "", ft.Error
		}
		return http.StatusInternalServerError, "", err
	}

	// Make sure the latest version of the module is the one in search_documents
	// and imports_unique.
	if err := f.DB.ReconcileSearch(ctx, modulePath, ft.ResolvedVersion, ft.Status); err != nil {
		log.Error(ctx, err)
		if ft.Status != http.StatusInternalServerError {
			ft.Error = err
			ft.Status = http.StatusInternalServerError
		}
		// Do not return an error here, because we want to insert into
		// module_version_states below.
	}

	// Update the module_version_states table with the new status of
	// module@version. This must happen last, because if it succeeds with a
	// code < 500 but a later action fails, we will never retry the later
	// action.
	startUpdate := time.Now()
	mvs := &postgres.ModuleVersionStateForUpdate{
		ModulePath:           ft.ModulePath,
		Version:              ft.ResolvedVersion,
		AppVersion:           appVersionLabel,
		Status:               ft.Status,
		HasGoMod:             ft.HasGoMod,
		GoModPath:            ft.GoModPath,
		FetchErr:             ft.Error,
		PackageVersionStates: ft.PackageVersionStates,
	}
	err = f.DB.UpdateModuleVersionState(ctx, mvs)
	ft.timings["db.UpdateModuleVersionState"] = time.Since(startUpdate)
	if err != nil {
		log.Error(ctx, err)
		ft.Status = http.StatusInternalServerError
		if ft.Error != nil {
			ft.Error = fmt.Errorf("db.UpdateModuleVersionState: %v, original error: %v", err, ft.Error)
		} else {
			ft.Error = err
		}
		logTaskResult(ctx, ft, "Failed to update module version state")
		return http.StatusInternalServerError, ft.ResolvedVersion, ft.Error
	}
	logTaskResult(ctx, ft, "Updated module version state")
	return ft.Status, ft.ResolvedVersion, ft.Error
}

func getInfo(ctx context.Context, modulePath, requestedVersion string, prox *proxy.Client) (_ *proxy.VersionInfo, err error) {
	if modulePath == stdlib.ModulePath {
		var resolvedVersion string
		resolvedVersion, err = stdlib.ZipInfo(requestedVersion)
		if err != nil {
			return nil, err
		}
		return &proxy.VersionInfo{Version: resolvedVersion}, nil
	}
	return prox.Info(ctx, modulePath, requestedVersion)
}

// fetchAndInsertModule fetches the given module version from the module proxy
// or (in the case of the standard library) from the Go repo and writes the
// resulting data to the database.
func (f *Fetcher) fetchAndInsertModule(ctx context.Context, modulePath, requestedVersion string, lmv *internal.LatestModuleVersions) *fetchTask {
	ft := &fetchTask{
		FetchResult: fetch.FetchResult{
			ModulePath:       modulePath,
			RequestedVersion: requestedVersion,
		},
		timings: map[string]time.Duration{},
	}
	defer func() {
		derrors.Wrap(&ft.Error, "fetchAndInsertModule(%q, %q)", modulePath, requestedVersion)
		if ft.Error != nil {
			ft.Status = derrors.ToStatus(ft.Error)
			ft.ResolvedVersion = requestedVersion
		}
	}()

	if f.DB.IsExcluded(ctx, modulePath, requestedVersion) {
		ft.Error = derrors.Excluded
		return ft
	}

	moduleGetter := fetch.NewProxyModuleGetter(f.ProxyClient, f.SourceClient)
	if modulePath == "std" {
		moduleGetter = fetch.NewStdlibZipModuleGetter()
	}
	// Fetch the module, and the current @main and @master version of this module.
	// The @main and @master version will be used to update the version_map
	// target if applicable.
	done := internal.RequestState(ctx, "fetching")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		fr := fetch.FetchModule(ctx, modulePath, requestedVersion, moduleGetter)
		if fr == nil {
			panic("fetch.FetchModule should never return a nil FetchResult")
		}
		ft.FetchResult = *fr
		ft.timings["fetch.FetchModule"] = time.Since(start)
	}()
	// Do not resolve the @main and @master version if proxy fetch is disabled.
	var main string
	wg.Add(1)
	go func() {
		defer wg.Done()
		if !f.ProxyClient.FetchDisabled() {
			main = resolvedVersion(ctx, modulePath, internal.MainVersion, moduleGetter)
		}
	}()
	var master string
	wg.Add(1)
	go func() {
		defer wg.Done()
		if !f.ProxyClient.FetchDisabled() {
			master = resolvedVersion(ctx, modulePath, internal.MasterVersion, moduleGetter)
		}
	}()
	wg.Wait()
	done()
	ft.MainVersion = main
	ft.MasterVersion = master

	// There was an error fetching this module.
	if ft.Error != nil {
		logf := log.Infof
		if ft.Status >= 500 && ft.Status != derrors.ToStatus(derrors.ProxyTimedOut) {
			logf = log.Warningf
		}
		logf(ctx, "Error executing fetch: %v (code %d)", ft.Error, ft.Status)
		return ft
	}

	// The module was successfully fetched.
	log.Debugf(ctx, "fetch.FetchModule succeeded for %s@%s", ft.ModulePath, ft.RequestedVersion)

	// Determine the current latest-version information for this module.

	start := time.Now()
	isLatest, err := f.DB.InsertModule(ctx, ft.Module, lmv)
	ft.timings["db.InsertModule"] = time.Since(start)
	if err != nil {
		ft.Status = derrors.ToStatus(err)
		ft.Error = err
		if ft.Status >= 500 {
			log.Error(ctx, err)
		} else {
			log.Info(ctx, err)
		}
		return ft
	}
	log.Debugf(ctx, "db.InsertModule succeeded for %s@%s", ft.ModulePath, ft.RequestedVersion)
	// Invalidate the cache if we just processed the latest version of a module.
	if isLatest {
		if err := f.invalidateCache(ctx, ft.ModulePath); err != nil {
			// Failure to invalidate the cache is not that serious; at worst it means some pages will be stale.
			// (Cache TTLs for details pages configured in internal/frontend/server.go must not be too long,
			// to account for this possibility.)
			log.Errorf(ctx, "failed to invalidate cache for %s: %v", ft.ModulePath, err)
		} else {
			log.Debugf(ctx, "invalidated cache for %s", ft.ModulePath)
		}
	}
	return ft
}

// invalidateCache deletes the series path for modulePath, as well as any
// possible URL path of which it is a componentwise prefix. That is, it deletes
// example.com/mod, example.com/mod@v1.2.3 and example.com/mod/pkg, but not the
// unrelated example.com/module.
//
// We delete the series path, not the module path, because adding a v2 module
// can affect v1 pages. For example, the first v2 module will add a "higher
// major version" banner to all v1 pages. While adding a v1 version won't
// currently affect v2 pages, that could change some day (for instance, if we
// decide to provide history). So it's better to be safe and delete all paths in
// the series.
func (f *Fetcher) invalidateCache(ctx context.Context, modulePath string) error {
	if f.Cache == nil {
		return nil
	}
	var errs []error
	seriesPath := internal.SeriesPathForModule(modulePath)
	// All cache keys are request URLs, so they begin with "/".
	if err := f.Cache.Delete(ctx, "/"+seriesPath); err != nil {
		errs = append(errs, err)
	}
	// Delete all suffixes of the series path followed by a character that marks its end.
	for _, end := range "/@?#" {
		if err := f.Cache.DeletePrefix(ctx, fmt.Sprintf("/%s%c", seriesPath, end)); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d errors, first is %w", len(errs), errs[0])
	}
	return nil
}

func resolvedVersion(ctx context.Context, modulePath, requestedVersion string, getter fetch.ModuleGetter) string {
	if modulePath == stdlib.ModulePath && requestedVersion == internal.MainVersion {
		return ""
	}
	info, err := fetch.GetInfo(ctx, modulePath, requestedVersion, getter)
	if err != nil {
		if !errors.Is(err, derrors.NotFound) {
			// If an error occurs, log it as a warning and insert the module as normal.
			log.Warningf(ctx, "fetch.GetInfo(ctx, %v, %q, getter, false): %v", modulePath, requestedVersion, err)
		}
		return ""
	}
	return info.Version
}

func updateVersionMap(ctx context.Context, db *postgres.DB, ft *fetchTask) (err error) {
	start := time.Now()
	defer func() {
		ft.timings["worker.updatedVersionMap"] = time.Since(start)
		derrors.Wrap(&err, "updateVersionMap(%q, %q, %q, %d, %v)",
			ft.ModulePath, ft.RequestedVersion, ft.ResolvedVersion, ft.Status, ft.Error)
	}()
	ctx, span := trace.StartSpan(ctx, "worker.updateVersionMap")
	defer span.End()

	var errMsg string
	if ft.Error != nil {
		errMsg = ft.Error.Error()
	}

	// If the resolved version for the this module version is also the resolved
	// version for @main or @master, update version_map to match.
	requestedVersions := []string{ft.RequestedVersion}
	if ft.MainVersion == ft.ResolvedVersion {
		requestedVersions = append(requestedVersions, internal.MainVersion)
	}
	if ft.MasterVersion == ft.ResolvedVersion {
		requestedVersions = append(requestedVersions, internal.MasterVersion)
	}
	for _, v := range requestedVersions {
		v := v
		vm := &internal.VersionMap{
			ModulePath:       ft.ModulePath,
			RequestedVersion: v,
			ResolvedVersion:  ft.ResolvedVersion,
			Status:           ft.Status,
			GoModPath:        ft.GoModPath,
			Error:            errMsg,
		}
		if err := db.UpsertVersionMap(ctx, vm); err != nil {
			return err
		}
	}
	return nil
}

func deleteModule(ctx context.Context, db *postgres.DB, ft *fetchTask) (err error) {
	defer internal.RequestState(ctx, "deleting module")()
	start := time.Now()
	defer func() {
		ft.timings["worker.deleteModule"] = time.Since(start)
		derrors.Wrap(&err, "deleteModule(%q, %q, %q, %d, %v)",
			ft.ModulePath, ft.RequestedVersion, ft.ResolvedVersion, ft.Status, ft.Error)
	}()
	ctx, span := trace.StartSpan(ctx, "worker.deleteModule")
	defer span.End()

	log.Infof(ctx, "%s@%s: code=%d, deleting", ft.ModulePath, ft.ResolvedVersion, ft.Status)
	if err := db.DeleteModule(ctx, ft.ModulePath, ft.ResolvedVersion); err != nil {
		return err
	}

	// Update the latest good version for this module, because deleting this
	// version may have changed it.
	return db.UpdateLatestGoodVersion(ctx, ft.ModulePath)
}

func logTaskResult(ctx context.Context, ft *fetchTask, prefix string) {
	var times []string
	for k, v := range ft.timings {
		times = append(times, fmt.Sprintf("%s=%.3fs", k, v.Seconds()))
	}
	sort.Strings(times)
	msg := strings.Join(times, ", ")
	logf := log.Infof
	if ft.Status == http.StatusInternalServerError {
		logf = log.Errorf
	}
	logf(ctx, "%s for %s@%s: code=%d, num_packages=%d, err=%v; timings: %s",
		prefix, ft.ModulePath, ft.ResolvedVersion, ft.Status, len(ft.PackageVersionStates), ft.Error, msg)
}

// FetchAndUpdateLatest fetches information about the latest versions from the proxy,
// and updates the database if the version has changed.
// It returns the most recent good information, which may be what it just fetched or
// may be what is already in the DB.
// It does not update the latest good version; that happens inside InsertModule, because
// it must be protected by the module-path advisory lock.
func (f *Fetcher) FetchAndUpdateLatest(ctx context.Context, modulePath string) (_ *internal.LatestModuleVersions, err error) {
	defer derrors.Wrap(&err, "FetchAndUpdateLatest(%q)", modulePath)
	defer internal.RequestState(ctx, "dealing with latest versions")()

	lmv, err := fetch.LatestModuleVersions(ctx, modulePath, f.ProxyClient, func(v string) (bool, error) {
		return f.DB.HasGoMod(ctx, modulePath, v)
	})
	var status int
	switch {
	case lmv != nil:
		status = 200
	case err == nil:
		// There may be no version information for the module, even if it exists.
		// In that case, we insert a 404 into the DB.
		status = 404
	default:
		status = derrors.ToStatus(err)
	}
	if status != 200 {
		return nil, f.DB.UpdateLatestModuleVersionsStatus(ctx, modulePath, status)
	}
	return f.DB.UpdateLatestModuleVersions(ctx, lmv)
}

func (f *Fetcher) maybeShed(ctx context.Context, modulePath, version string) (func(), int64, error) {
	if f.loadShedder == nil {
		return func() {}, 0, nil
	}
	zipSize, err := getZipSize(ctx, modulePath, version, f.ProxyClient)
	if err != nil {
		return func() {}, 0, err
	}
	// Load shed or mark module as too large.
	// We treat zip size as a proxy for the total memory consumed by
	// processing a module, and use it to decide whether we can currently
	// afford to process a module.
	shouldShed, deferFunc := f.loadShedder.shouldShed(uint64(zipSize))
	if shouldShed {
		stats.Record(ctx, fetchesShedded.M(1))
		return deferFunc, 0, fmt.Errorf("%w: size=%dMi", derrors.SheddingLoad, zipSize/mib)
	}
	if zipSize > maxModuleZipSize {
		log.Warningf(ctx, "FetchModule: %s@%s zip size %dMi exceeds max %dMi",
			modulePath, version, zipSize/mib, maxModuleZipSize/mib)
		return deferFunc, 0, derrors.ModuleTooLarge
	}
	return deferFunc, zipSize, nil
}

func getZipSize(ctx context.Context, modulePath, resolvedVersion string, prox *proxy.Client) (_ int64, err error) {
	if modulePath == stdlib.ModulePath {
		return stdlib.EstimatedZipSize, nil
	}
	return prox.ZipSize(ctx, modulePath, resolvedVersion)
}
