// Copyright (c) 2016, 2018, 2021, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

package dns

import (
	"github.com/oracle/oci-go-sdk/v32/common"
	"net/http"
)

// UpdateResolverEndpointRequest wrapper for the UpdateResolverEndpoint operation
//
// See also
//
// Click https://docs.cloud.oracle.com/en-us/iaas/tools/go-sdk-examples/latest/dns/UpdateResolverEndpoint.go.html to see an example of how to use UpdateResolverEndpointRequest.
type UpdateResolverEndpointRequest struct {

	// The OCID of the target resolver.
	ResolverId *string `mandatory:"true" contributesTo:"path" name:"resolverId"`

	// The name of the target resolver endpoint.
	ResolverEndpointName *string `mandatory:"true" contributesTo:"path" name:"resolverEndpointName"`

	// New data for the resolver endpoint.
	UpdateResolverEndpointDetails `contributesTo:"body"`

	// The `If-Match` header field makes the request method conditional on the
	// existence of at least one current representation of the target resource,
	// when the field-value is `*`, or having a current representation of the
	// target resource that has an entity-tag matching a member of the list of
	// entity-tags provided in the field-value.
	IfMatch *string `mandatory:"false" contributesTo:"header" name:"If-Match"`

	// The `If-Unmodified-Since` header field makes the request method
	// conditional on the selected representation's last modification date being
	// earlier than or equal to the date provided in the field-value.  This
	// field accomplishes the same purpose as If-Match for cases where the user
	// agent does not have an entity-tag for the representation.
	IfUnmodifiedSince *string `mandatory:"false" contributesTo:"header" name:"If-Unmodified-Since"`

	// Unique Oracle-assigned identifier for the request. If you need
	// to contact Oracle about a particular request, please provide
	// the request ID.
	OpcRequestId *string `mandatory:"false" contributesTo:"header" name:"opc-request-id"`

	// Specifies to operate only on resources that have a matching DNS scope.
	Scope UpdateResolverEndpointScopeEnum `mandatory:"false" contributesTo:"query" name:"scope" omitEmpty:"true"`

	// Metadata about the request. This information will not be transmitted to the service, but
	// represents information that the SDK will consume to drive retry behavior.
	RequestMetadata common.RequestMetadata
}

func (request UpdateResolverEndpointRequest) String() string {
	return common.PointerString(request)
}

// HTTPRequest implements the OCIRequest interface
func (request UpdateResolverEndpointRequest) HTTPRequest(method, path string) (http.Request, error) {
	return common.MakeDefaultHTTPRequestWithTaggedStruct(method, path, request)
}

// RetryPolicy implements the OCIRetryableRequest interface. This retrieves the specified retry policy.
func (request UpdateResolverEndpointRequest) RetryPolicy() *common.RetryPolicy {
	return request.RequestMetadata.RetryPolicy
}

// UpdateResolverEndpointResponse wrapper for the UpdateResolverEndpoint operation
type UpdateResolverEndpointResponse struct {

	// The underlying http response
	RawResponse *http.Response

	// The ResolverEndpoint instance
	ResolverEndpoint `presentIn:"body"`

	// The current version of the resource, ending with a
	// representation-specific suffix. This value may be used in If-Match
	// and If-None-Match headers for later requests of the same resource.
	Etag *string `presentIn:"header" name:"etag"`

	// Unique Oracle-assigned identifier for the request. If you need to
	// contact Oracle about a particular request, please provide the request ID.
	OpcRequestId *string `presentIn:"header" name:"opc-request-id"`

	// Unique Oracle-assigned identifier for the asynchronous request.
	// You can use this to query status of the asynchronous operation.
	OpcWorkRequestId *string `presentIn:"header" name:"opc-work-request-id"`
}

func (response UpdateResolverEndpointResponse) String() string {
	return common.PointerString(response)
}

// HTTPResponse implements the OCIResponse interface
func (response UpdateResolverEndpointResponse) HTTPResponse() *http.Response {
	return response.RawResponse
}

// UpdateResolverEndpointScopeEnum Enum with underlying type: string
type UpdateResolverEndpointScopeEnum string

// Set of constants representing the allowable values for UpdateResolverEndpointScopeEnum
const (
	UpdateResolverEndpointScopeGlobal  UpdateResolverEndpointScopeEnum = "GLOBAL"
	UpdateResolverEndpointScopePrivate UpdateResolverEndpointScopeEnum = "PRIVATE"
)

var mappingUpdateResolverEndpointScope = map[string]UpdateResolverEndpointScopeEnum{
	"GLOBAL":  UpdateResolverEndpointScopeGlobal,
	"PRIVATE": UpdateResolverEndpointScopePrivate,
}

// GetUpdateResolverEndpointScopeEnumValues Enumerates the set of values for UpdateResolverEndpointScopeEnum
func GetUpdateResolverEndpointScopeEnumValues() []UpdateResolverEndpointScopeEnum {
	values := make([]UpdateResolverEndpointScopeEnum, 0)
	for _, v := range mappingUpdateResolverEndpointScope {
		values = append(values, v)
	}
	return values
}
