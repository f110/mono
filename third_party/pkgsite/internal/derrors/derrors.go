// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package derrors defines internal error values to categorize the different
// types error semantics we support.
package derrors

import (
	"errors"
	"fmt"
	"net/http"
)

//lint:file-ignore ST1012 prefixing error values with Err would stutter

var (
	// HasIncompletePackages indicates a module containing packages that
	// were processed with a 60x error code.
	HasIncompletePackages = errors.New("has incomplete packages")

	// NotFound indicates that a requested entity was not found (HTTP 404).
	NotFound = errors.New("not found")
	// InvalidArgument indicates that the input into the request is invalid in
	// some way (HTTP 400).
	InvalidArgument = errors.New("invalid argument")
	// BadModule indicates a problem with a module.
	BadModule = errors.New("bad module")
	// Excluded indicates that the module is excluded. (See internal/postgres/excluded.go.)
	Excluded = errors.New("excluded")

	// AlternativeModule indicates that the path of the module zip file differs
	// from the path specified in the go.mod file.
	AlternativeModule = errors.New("alternative module")

	// Unknown indicates that the error has unknown semantics.
	Unknown = errors.New("unknown")

	// PackageBuildContextNotSupported indicates that the build context for the
	// package is not supported.
	PackageBuildContextNotSupported = errors.New("package build context not supported")
	// PackageMaxImportsLimitExceeded indicates that the package has too many
	// imports.
	PackageMaxImportsLimitExceeded = errors.New("package max imports limit exceeded")
	// PackageMaxFileSizeLimitExceeded indicates that the package contains a file
	// that exceeds fetch.MaxFileSize.
	PackageMaxFileSizeLimitExceeded = errors.New("package max file size limit exceeded")
	// PackageDocumentationHTMLTooLarge indicates that the rendered documentation
	// HTML size exceeded the specified limit for dochtml.RenderOptions.
	PackageDocumentationHTMLTooLarge = errors.New("package documentation HTML is too large")
	// PackageBadImportPath represents an error loading a package because its
	// contents do not make up a valid package. This can happen, for
	// example, if the .go files fail to parse or declare different package
	// names.
	// Go files were found in a directory, but the resulting import path is invalid.
	PackageBadImportPath = errors.New("package bad import path")
	// PackageInvalidContents represents an error loading a package because
	// its contents do not make up a valid package. This can happen, for
	// example, if the .go files fail to parse or declare different package
	// names.
	PackageInvalidContents = errors.New("package invalid contents")

	// DBModuleInsertInvalid represents a module that was successfully
	// fetched but could not be inserted due to invalid arguments to
	// postgres.InsertModule.
	DBModuleInsertInvalid = errors.New("db module insert invalid")

	// ReprocessStatusOK indicates that the module to be reprocessed
	// previously had a status of http.StatusOK.
	ReprocessStatusOK = errors.New("reprocess status ok")
	// ReprocessHasIncompletePackages indicates that the module to be reprocessed
	// previously had a status of 290.
	ReprocessHasIncompletePackages = errors.New("reprocess has incomplete packages")
	// ReprocessBadModule indicates that the module to be reprocessed
	// previously had a status of derrors.BadModule.
	ReprocessBadModule = errors.New("reprocess bad module")
	// ReprocessAlternativeModule indicates that the module to be reprocessed
	// previously had a status of derrors.AlternativeModule.
	ReprocessAlternative = errors.New("reprocess alternative module")
)

var httpCodes = []struct {
	err  error
	code int
}{
	{NotFound, http.StatusNotFound},
	{InvalidArgument, http.StatusBadRequest},
	{Excluded, http.StatusForbidden},

	// Since the following aren't HTTP statuses, pick unused codes.
	{HasIncompletePackages, 290},
	{DBModuleInsertInvalid, 480},
	{BadModule, 490},
	{AlternativeModule, 491},

	// 52x errors represents modules that need to be reprocessed, and the
	// previous status code the module had. Note that the status code
	// matters for determining reprocessing order.
	{ReprocessStatusOK, 520},
	{ReprocessHasIncompletePackages, 521},
	{ReprocessBadModule, 540},
	{ReprocessAlternative, 541},

	// 60x errors represents errors that occurred when processing a
	// package.
	{PackageBuildContextNotSupported, 600},
	{PackageMaxImportsLimitExceeded, 601},
	{PackageMaxFileSizeLimitExceeded, 602},
	{PackageDocumentationHTMLTooLarge, 603},
	{PackageInvalidContents, 604},
	{PackageBadImportPath, 605},
}

// FromHTTPStatus generates an error according to the HTTP semantics for the given
// status code. It uses the given format string and arguments to create the
// error string according to the fmt package. If format is the empty string,
// then the error corresponding to the code is returned unwrapped.
//
// If HTTP semantics indicate success, it returns nil.
func FromHTTPStatus(code int, format string, args ...interface{}) error {
	if code >= 200 && code < 300 {
		return nil
	}
	var innerErr = Unknown
	for _, e := range httpCodes {
		if e.code == code {
			innerErr = e.err
			break
		}
	}
	if format == "" {
		return innerErr
	}
	return fmt.Errorf(format+": %w", append(args, innerErr)...)
}

// ToHTTPStatus returns an HTTP status code corresponding to err.
func ToHTTPStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	for _, e := range httpCodes {
		if errors.Is(err, e.err) {
			return e.code
		}
	}
	return http.StatusInternalServerError
}

// ToReprocessStatus returns the reprocess status code corresponding to the
// provided status.
func ToReprocessStatus(status int) int {
	switch status {
	case http.StatusOK:
		return ToHTTPStatus(ReprocessStatusOK)
	case ToHTTPStatus(HasIncompletePackages):
		return ToHTTPStatus(ReprocessHasIncompletePackages)
	case ToHTTPStatus(BadModule):
		return ToHTTPStatus(ReprocessBadModule)
	case ToHTTPStatus(AlternativeModule):
		return ToHTTPStatus(ReprocessAlternative)
	default:
		return status
	}
}

// Add adds context to the error.
// The result cannot be unwrapped to recover the original error.
// It does nothing when *errp == nil.
//
// Example:
//
//	defer derrors.Add(&err, "copy(%s, %s)", src, dst)
//
// See Wrap for an equivalent function that allows
// the result to be unwrapped.
func Add(errp *error, format string, args ...interface{}) {
	if *errp != nil {
		*errp = fmt.Errorf("%s: %v", fmt.Sprintf(format, args...), *errp)
	}
}

// Wrap adds context to the error and allows
// unwrapping the result to recover the original error.
//
// Example:
//
//	defer derrors.Wrap(&err, "copy(%s, %s)", src, dst)
//
// See Add for an equivalent function that does not allow
// the result to be unwrapped.
func Wrap(errp *error, format string, args ...interface{}) {
	if *errp != nil {
		*errp = fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), *errp)
	}
}
