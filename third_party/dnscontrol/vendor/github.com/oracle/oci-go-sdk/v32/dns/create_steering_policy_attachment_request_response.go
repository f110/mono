// Copyright (c) 2016, 2018, 2021, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

package dns

import (
	"github.com/oracle/oci-go-sdk/v32/common"
	"net/http"
)

// CreateSteeringPolicyAttachmentRequest wrapper for the CreateSteeringPolicyAttachment operation
//
// See also
//
// Click https://docs.cloud.oracle.com/en-us/iaas/tools/go-sdk-examples/latest/dns/CreateSteeringPolicyAttachment.go.html to see an example of how to use CreateSteeringPolicyAttachmentRequest.
type CreateSteeringPolicyAttachmentRequest struct {

	// Details for creating a new steering policy attachment.
	CreateSteeringPolicyAttachmentDetails `contributesTo:"body"`

	// A token that uniquely identifies a request so it can be retried in case
	// of a timeout or server error without risk of executing that same action
	// again. Retry tokens expire after 24 hours, but can be invalidated before
	// then due to conflicting operations (for example, if a resource has been
	// deleted and purged from the system, then a retry of the original creation
	// request may be rejected).
	OpcRetryToken *string `mandatory:"false" contributesTo:"header" name:"opc-retry-token"`

	// Unique Oracle-assigned identifier for the request. If you need
	// to contact Oracle about a particular request, please provide
	// the request ID.
	OpcRequestId *string `mandatory:"false" contributesTo:"header" name:"opc-request-id"`

	// Specifies to operate only on resources that have a matching DNS scope.
	Scope CreateSteeringPolicyAttachmentScopeEnum `mandatory:"false" contributesTo:"query" name:"scope" omitEmpty:"true"`

	// Metadata about the request. This information will not be transmitted to the service, but
	// represents information that the SDK will consume to drive retry behavior.
	RequestMetadata common.RequestMetadata
}

func (request CreateSteeringPolicyAttachmentRequest) String() string {
	return common.PointerString(request)
}

// HTTPRequest implements the OCIRequest interface
func (request CreateSteeringPolicyAttachmentRequest) HTTPRequest(method, path string) (http.Request, error) {
	return common.MakeDefaultHTTPRequestWithTaggedStruct(method, path, request)
}

// RetryPolicy implements the OCIRetryableRequest interface. This retrieves the specified retry policy.
func (request CreateSteeringPolicyAttachmentRequest) RetryPolicy() *common.RetryPolicy {
	return request.RequestMetadata.RetryPolicy
}

// CreateSteeringPolicyAttachmentResponse wrapper for the CreateSteeringPolicyAttachment operation
type CreateSteeringPolicyAttachmentResponse struct {

	// The underlying http response
	RawResponse *http.Response

	// The SteeringPolicyAttachment instance
	SteeringPolicyAttachment `presentIn:"body"`

	// The current version of the resource, ending with a
	// representation-specific suffix. This value may be used in If-Match
	// and If-None-Match headers for later requests of the same resource.
	ETag *string `presentIn:"header" name:"etag"`

	// The full URI of the resource related to the request.
	Location *string `presentIn:"header" name:"location"`

	// Unique Oracle-assigned identifier for the request. If you need to
	// contact Oracle about a particular request, please provide the request
	// ID.
	OpcRequestId *string `presentIn:"header" name:"opc-request-id"`
}

func (response CreateSteeringPolicyAttachmentResponse) String() string {
	return common.PointerString(response)
}

// HTTPResponse implements the OCIResponse interface
func (response CreateSteeringPolicyAttachmentResponse) HTTPResponse() *http.Response {
	return response.RawResponse
}

// CreateSteeringPolicyAttachmentScopeEnum Enum with underlying type: string
type CreateSteeringPolicyAttachmentScopeEnum string

// Set of constants representing the allowable values for CreateSteeringPolicyAttachmentScopeEnum
const (
	CreateSteeringPolicyAttachmentScopeGlobal  CreateSteeringPolicyAttachmentScopeEnum = "GLOBAL"
	CreateSteeringPolicyAttachmentScopePrivate CreateSteeringPolicyAttachmentScopeEnum = "PRIVATE"
)

var mappingCreateSteeringPolicyAttachmentScope = map[string]CreateSteeringPolicyAttachmentScopeEnum{
	"GLOBAL":  CreateSteeringPolicyAttachmentScopeGlobal,
	"PRIVATE": CreateSteeringPolicyAttachmentScopePrivate,
}

// GetCreateSteeringPolicyAttachmentScopeEnumValues Enumerates the set of values for CreateSteeringPolicyAttachmentScopeEnum
func GetCreateSteeringPolicyAttachmentScopeEnumValues() []CreateSteeringPolicyAttachmentScopeEnum {
	values := make([]CreateSteeringPolicyAttachmentScopeEnum, 0)
	for _, v := range mappingCreateSteeringPolicyAttachmentScope {
		values = append(values, v)
	}
	return values
}
