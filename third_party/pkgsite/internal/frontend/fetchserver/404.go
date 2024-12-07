// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fetchserver

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/safehtml/template/uncheckedconversions"
	"golang.org/x/pkgsite/internal"
	"golang.org/x/pkgsite/internal/cookie"
	"golang.org/x/pkgsite/internal/derrors"
	"golang.org/x/pkgsite/internal/experiment"
	"golang.org/x/pkgsite/internal/frontend/page"
	"golang.org/x/pkgsite/internal/frontend/serrors"
	"golang.org/x/pkgsite/internal/frontend/urlinfo"
	"golang.org/x/pkgsite/internal/frontend/versions"
	"golang.org/x/pkgsite/internal/log"
	"golang.org/x/pkgsite/internal/stdlib"
	"golang.org/x/pkgsite/internal/version"
)

// servePathNotFoundPage serves a 404 page for the requested path, or redirects
// the user to an appropriate location.
func (s *FetchServer) ServePathNotFoundPage(w http.ResponseWriter, r *http.Request,
	db internal.PostgresDB, fullPath, modulePath, requestedVersion string) (err error) {
	defer derrors.Wrap(&err, "servePathNotFoundPage(w, r, %q, %q)", fullPath, requestedVersion)

	ctx := r.Context()

	if stdlib.Contains(fullPath) {
		var path string
		path, err = stdlibPathForShortcut(ctx, db, fullPath)
		if err != nil {
			// Log the error, but prefer a "path not found" error for a
			// better user experience.
			log.Error(ctx, err)
		}
		if path != "" {
			http.Redirect(w, r, fmt.Sprintf("/%s", path), http.StatusFound)
			return
		}

		if experiment.IsActive(ctx, internal.ExperimentEnableStdFrontendFetch) {
			return &serrors.ServerError{
				Status: http.StatusNotFound,
				Epage: &page.ErrorPage{
					TemplateName: "fetch",
					MessageData:  stdlib.ModulePath,
				},
			}
		}
		return &serrors.ServerError{Status: http.StatusNotFound}
	}

	fr, err := previousFetchStatusAndResponse(ctx, db, fullPath, modulePath, requestedVersion)
	if err != nil {
		// If an error occurred, it means that we have never tried to fetch
		// this path before or an error occurred when we tried to
		// gather data about this 404.
		//
		// If the latter, log the error.
		// In either case, give the user the option to fetch that path.
		if !errors.Is(err, derrors.NotFound) && !errors.Is(err, derrors.InvalidArgument) {
			log.Error(ctx, err)
		}
		return pathNotFoundError(ctx, fullPath, requestedVersion)
	}

	// If we've reached this point, we know that we've seen this path before.
	// Show a relevant page or redirect the use based on the previous fetch
	// response.
	switch fr.status {
	case http.StatusOK, derrors.ToStatus(derrors.HasIncompletePackages):
		// We will only reach a 2xx status if we found a row in version_map
		// matching exactly the requested path.
		if fr.resolvedVersion != requestedVersion {
			u := versions.ConstructUnitURL(fullPath, fr.goModPath, fr.resolvedVersion)
			http.Redirect(w, r, u, http.StatusFound)
			return
		}
		// For some reason version_map is telling us that the path@version
		// exists, but earlier in this flow we didn't find it in the units
		// table.
		//
		// Return the fetch page so the user can try requesting again, and log
		// an error.
		log.Errorf(ctx, "version_map reports that %s@%s has status=%d, but this was not found before reaching servePathNotFoundPage",
			fullPath, requestedVersion, fr.status)
		return pathNotFoundError(ctx, fullPath, requestedVersion)
	case http.StatusFound, derrors.ToStatus(derrors.AlternativeModule):
		if fr.goModPath == fullPath {
			// The redirectPath and the fullpath are the same. Do not redirect
			// to avoid ending up in a loop.
			return serrors.ErrUnitNotFoundWithoutFetch
		}
		vm, err := db.GetVersionMap(ctx, fr.goModPath, version.Latest)
		if (err != nil && !errors.Is(err, derrors.NotFound)) ||
			(vm != nil && vm.Status != http.StatusOK) {
			// We attempted to fetch the canonical module path before and were
			// not successful. Do not redirect this request.
			return serrors.ErrUnitNotFoundWithoutFetch
		}
		u := versions.ConstructUnitURL(fr.goModPath, fr.goModPath, version.Latest)
		cookie.Set(w, cookie.AlternativeModuleFlash, fullPath, u)
		http.Redirect(w, r, u, http.StatusFound)
		return nil
	case http.StatusInternalServerError:
		return pathNotFoundError(ctx, fullPath, requestedVersion)
	default:
		if u := githubPathRedirect(fullPath); u != "" {
			http.Redirect(w, r, u, http.StatusFound)
			return
		}

		// If a module has a status of 404, but s.taskIDChangeInterval has
		// passed, allow the module to be refetched.
		if fr.status == http.StatusNotFound && time.Since(fr.updatedAt) > s.TaskIDChangeInterval {
			return pathNotFoundError(ctx, fullPath, requestedVersion)
		}

		// Redirect to the search result page for an empty directory that is above nested modules.
		// See https://golang.org/issue/43725 for context.
		nm, err := db.GetNestedModules(ctx, fullPath)
		if err == nil && len(nm) > 0 {
			http.Redirect(w, r, "/search?q="+url.QueryEscape(fullPath), http.StatusFound)
			return nil
		}
		return &serrors.ServerError{
			Status: fr.status,
			Epage: &page.ErrorPage{
				MessageTemplate: uncheckedconversions.TrustedTemplateFromStringKnownToSatisfyTypeContract(`
					    <h3 class="Error-message">{{.StatusText}}</h3>
					    <p class="Error-message">` + html.UnescapeString(fr.responseText) + `</p>`),
				MessageData: struct{ StatusText string }{http.StatusText(fr.status)},
			},
		}
	}
}

// githubRegexp is regex to match a GitHub URL scheme containing a "/blob" or
// "/tree" element.
var githubRegexp = regexp.MustCompile(`(blob|tree)(/[^/]+)?`)

func githubPathRedirect(fullPath string) string {
	parts := strings.Split(fullPath, "/")
	if len(parts) <= 3 || parts[0] != "github.com" {
		return ""
	}
	m := strings.Split(fullPath, "/"+githubRegexp.FindString(fullPath))
	if len(m) != 2 {
		return ""
	}
	p := m[0]
	if m[1] != "" {
		p = m[0] + m[1]
	}
	return versions.ConstructUnitURL(p, p, version.Latest)
}

// pathNotFoundError returns a page with an option on how to
// add a package or module to the site.
func pathNotFoundError(ctx context.Context, fullPath, requestedVersion string) error {
	if !urlinfo.IsSupportedVersion(fullPath, requestedVersion) {
		return serrors.InvalidVersionError(fullPath, requestedVersion)
	}
	if stdlib.Contains(fullPath) {
		if experiment.IsActive(ctx, internal.ExperimentEnableStdFrontendFetch) {
			return &serrors.ServerError{
				Status: http.StatusNotFound,
				Epage: &page.ErrorPage{
					TemplateName: "fetch",
					MessageData:  stdlib.ModulePath,
				},
			}
		}
		return &serrors.ServerError{Status: http.StatusNotFound}
	}
	path := fullPath
	if requestedVersion != version.Latest {
		path = fmt.Sprintf("%s@%s", fullPath, requestedVersion)
	}
	return &serrors.ServerError{
		Status: http.StatusNotFound,
		Epage: &page.ErrorPage{
			TemplateName: "fetch",
			MessageData:  path,
		},
	}
}

// previousFetchStatusAndResponse returns the fetch result from a
// previous fetch of the fullPath and requestedVersion.
func previousFetchStatusAndResponse(ctx context.Context, db internal.PostgresDB,
	fullPath, modulePath, requestedVersion string) (_ *fetchResult, err error) {
	defer derrors.Wrap(&err, "previousFetchStatusAndResponse(w, r, %q, %q)", fullPath, requestedVersion)

	// Get all candidate module paths for this path.
	paths, err := modulePathsToFetch(ctx, db, fullPath, modulePath)
	if err != nil {
		var serr *serrors.ServerError
		if errors.As(err, &serr) && serr.Status == http.StatusBadRequest {
			// Return this as an invalid argument so that we don't log it in
			// servePathNotFoundPage above.
			return nil, derrors.InvalidArgument
		}
		return nil, err
	}
	// Check if a row exists in the version_map table for the longest candidate
	// path and version.
	//
	// If we have not fetched the path before, a derrors.NotFound will be
	// returned.
	vm, err := db.GetVersionMap(ctx, paths[0], requestedVersion)
	if err != nil {
		return nil, err
	}
	// If the row has been fetched before, and the result was either a 490,
	// 491, or 5xx, return that result, since it is a final state.
	if vm != nil {
		fr := &fetchResult{
			modulePath: vm.ModulePath,
			goModPath:  vm.GoModPath,
			status:     vm.Status,
			err:        errors.New(vm.Error),
		}
		if vm.Status >= 200 && vm.Status < 300 {
			fr.resolvedVersion = vm.ResolvedVersion
			return fr, nil
		}
		if vm.Status >= 500 ||
			vm.Status == derrors.ToStatus(derrors.AlternativeModule) ||
			vm.Status == derrors.ToStatus(derrors.BadModule) {
			return resultFromFetchRequest([]*fetchResult{fr}, fullPath, requestedVersion)
		}
	}

	// Check if the unit path exists at a higher major version.
	// For example, my.module might not exist, but my.module/v3 might.
	// Similarly, my.module/foo might not exist, but my.module/v3/foo might.
	// In either case, the user will be redirected to the highest major version
	// of the path.
	//
	// Do not bother to look for a specific version if this case. If
	// my.module/foo@v2.1.0 was requested, and my.module/foo/v2 exists, just
	// return the latest version of my.module/foo/v2.
	//
	// Only redirect if the majPath returned is different from the fullPath, and
	// the majPath is not at v1. We don't want to redirect my.module/foo/v3 to
	// my.module/foo, or my.module/foo@v1.5.2 to my.module/foo@v1.0.0.
	majPath, maj, err := db.GetLatestMajorPathForV1Path(ctx, fullPath)
	if err != nil && err != derrors.NotFound {
		return nil, err
	}
	if majPath != fullPath && maj != 1 && majPath != "" {
		return &fetchResult{
			modulePath: majPath,
			goModPath:  majPath,
			status:     http.StatusFound,
		}, nil
	}
	vms, err := db.GetVersionMaps(ctx, paths, requestedVersion)
	if err != nil {
		return nil, err
	}
	var fetchResults []*fetchResult
	for _, vm := range vms {
		fr := fetchResultFromVersionMap(vm)
		fetchResults = append(fetchResults, fr)
		if vm.Status == http.StatusOK || vm.Status == 290 {
			fr.err = errPathDoesNotExistInModule
		}
	}
	if len(fetchResults) == 0 {
		return nil, derrors.NotFound
	}
	return resultFromFetchRequest(fetchResults, fullPath, requestedVersion)
}

func fetchResultFromVersionMap(vm *internal.VersionMap) *fetchResult {
	var err error
	if vm.Error != "" {
		err = errors.New(vm.Error)
	}
	return &fetchResult{
		modulePath: vm.ModulePath,
		goModPath:  vm.GoModPath,
		status:     vm.Status,
		updatedAt:  vm.UpdatedAt,
		err:        err,
	}
}

// stdlibPathForShortcut returns a path in the stdlib that shortcut should redirect to,
// or the empty string if there is no such path.
func stdlibPathForShortcut(ctx context.Context, db internal.PostgresDB, shortcut string) (path string, err error) {
	defer derrors.Wrap(&err, "stdlibPathForShortcut(ctx, %q)", shortcut)
	if !stdlib.Contains(shortcut) {
		return "", nil
	}
	matches, err := db.GetStdlibPathsWithSuffix(ctx, shortcut)
	if err != nil {
		return "", err
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	// No matches, or ambiguous.
	return "", nil
}
