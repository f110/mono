// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package frontend

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"golang.org/x/pkgsite/internal"
	"golang.org/x/pkgsite/internal/derrors"
	"golang.org/x/pkgsite/internal/log"
)

// serveModulePage serves details pages for the module specified by modulePath
// and version.
func (s *Server) serveModulePage(w http.ResponseWriter, r *http.Request, modulePath, requestedVersion string) error {
	// This function handles top level behavior related to the existence of the
	// requested modulePath@version:
	// TODO: fix
	//   1. If the module version exists, serve it.
	//   2. else if we got any unexpected error, serve a server error
	//   3. else if the error is NotFound, serve the directory page
	//   3. else, we didn't find the module so there are two cases:
	//     a. We don't know anything about this module: just serve a 404
	//     b. We have valid versions for this module path, but `version` isn't
	//        one of them. Serve a 404 but recommend the other versions.
	ctx := r.Context()
	mi, err := s.ds.GetModuleInfo(ctx, modulePath, requestedVersion)
	if err == nil {
		return s.serveModulePageWithModule(ctx, w, r, mi, requestedVersion)
	}
	if !errors.Is(err, derrors.NotFound) {
		return err
	}
	if requestedVersion != internal.LatestVersion {
		_, err = s.ds.GetModuleInfo(ctx, modulePath, internal.LatestVersion)
		if err == nil {
			return pathFoundAtLatestError(ctx, "module", modulePath, displayVersion(requestedVersion, modulePath))
		}
		if !errors.Is(err, derrors.NotFound) {
			log.Errorf(ctx, "error checking for latest module: %v", err)
		}
	}
	return pathNotFoundError(ctx, "module", modulePath, requestedVersion)
}

func (s *Server) serveModulePageWithModule(ctx context.Context, w http.ResponseWriter, r *http.Request, mi *internal.LegacyModuleInfo, requestedVersion string) error {
	licenses, err := s.ds.GetModuleLicenses(ctx, mi.ModulePath, mi.Version)
	if err != nil {
		return err
	}

	modHeader := createModule(&mi.ModuleInfo, licensesToMetadatas(licenses), requestedVersion == internal.LatestVersion)
	tab := r.FormValue("tab")
	settings, ok := moduleTabLookup[tab]
	if !ok {
		tab = "overview"
		settings = moduleTabLookup["overview"]
	}
	canShowDetails := modHeader.IsRedistributable || settings.AlwaysShowDetails
	var details interface{}
	if canShowDetails {
		var err error
		details, err = fetchDetailsForModule(ctx, r, tab, s.ds, mi, licenses)
		if err != nil {
			return fmt.Errorf("error fetching page for %q: %v", tab, err)
		}
	}
	page := &DetailsPage{
		basePage:       s.newBasePage(r, moduleHTMLTitle(mi.ModulePath)),
		Title:          moduleTitle(mi.ModulePath),
		Settings:       settings,
		Header:         modHeader,
		BreadcrumbPath: breadcrumbPath(modHeader.ModulePath, modHeader.ModulePath, modHeader.LinkVersion),
		Details:        details,
		CanShowDetails: canShowDetails,
		Tabs:           moduleTabSettings,
		PageType:       "mod",
	}
	s.servePage(ctx, w, settings.TemplateName, page)
	return nil
}
