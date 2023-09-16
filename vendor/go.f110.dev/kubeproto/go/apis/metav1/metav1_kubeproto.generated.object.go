package metav1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

type CauseType string

const (
	CauseTypeFieldValueNotFound       CauseType = "FieldValueNotFound"
	CauseTypeFieldValueRequired       CauseType = "FieldValueRequired"
	CauseTypeFieldValueDuplicate      CauseType = "FieldValueDuplicate"
	CauseTypeFieldValueInvalid        CauseType = "FieldValueInvalid"
	CauseTypeFieldValueNotSupported   CauseType = "FieldValueNotSupported"
	CauseTypeUnexpectedServerResponse CauseType = "UnexpectedServerResponse"
	CauseTypeFieldManagerConflict     CauseType = "FieldManagerConflict"
	CauseTypeResourceVersionTooLarge  CauseType = "ResourceVersionTooLarge"
)

type ConditionStatus string

const (
	ConditionStatusTrue    ConditionStatus = "True"
	ConditionStatusFalse   ConditionStatus = "False"
	ConditionStatusUnknown ConditionStatus = "Unknown"
)

type DeletionPropagation string

const (
	DeletionPropagationOrphan     DeletionPropagation = "Orphan"
	DeletionPropagationBackground DeletionPropagation = "Background"
	DeletionPropagationForeground DeletionPropagation = "Foreground"
)

type IncludeObjectPolicy string

const (
	IncludeObjectPolicyNone     IncludeObjectPolicy = "None"
	IncludeObjectPolicyMetadata IncludeObjectPolicy = "Metadata"
	IncludeObjectPolicyObject   IncludeObjectPolicy = "Object"
)

type LabelSelectorOperator string

const (
	LabelSelectorOperatorIn           LabelSelectorOperator = "In"
	LabelSelectorOperatorNotIn        LabelSelectorOperator = "NotIn"
	LabelSelectorOperatorExists       LabelSelectorOperator = "Exists"
	LabelSelectorOperatorDoesNotExist LabelSelectorOperator = "DoesNotExist"
)

type ManagedFieldsOperationType string

const (
	ManagedFieldsOperationTypeApply  ManagedFieldsOperationType = "Apply"
	ManagedFieldsOperationTypeUpdate ManagedFieldsOperationType = "Update"
)

type ResourceVersionMatch string

const (
	ResourceVersionMatchNotOlderThan ResourceVersionMatch = "NotOlderThan"
	ResourceVersionMatchExact        ResourceVersionMatch = "Exact"
)

type RowConditionType string

const (
	RowConditionTypeCompleted RowConditionType = "Completed"
)

type StatusReason string

const (
	StatusReasonUNKNOWN               StatusReason = "UNKNOWN"
	StatusReasonUnauthorized          StatusReason = "Unauthorized"
	StatusReasonForbidden             StatusReason = "Forbidden"
	StatusReasonNotFound              StatusReason = "NotFound"
	StatusReasonAlreadyExists         StatusReason = "AlreadyExists"
	StatusReasonConflict              StatusReason = "Conflict"
	StatusReasonGone                  StatusReason = "Gone"
	StatusReasonInvalid               StatusReason = "Invalid"
	StatusReasonServerTimeout         StatusReason = "ServerTimeout"
	StatusReasonTimeout               StatusReason = "Timeout"
	StatusReasonTooManyRequests       StatusReason = "TooManyRequests"
	StatusReasonBadRequest            StatusReason = "BadRequest"
	StatusReasonMethodNotAllowed      StatusReason = "MethodNotAllowed"
	StatusReasonNotAcceptable         StatusReason = "NotAcceptable"
	StatusReasonRequestEntityTooLarge StatusReason = "RequestEntityTooLarge"
	StatusReasonUnsupportedMediaType  StatusReason = "UnsupportedMediaType"
	StatusReasonInternalError         StatusReason = "InternalError"
	StatusReasonExpired               StatusReason = "Expired"
	StatusReasonServiceUnavailable    StatusReason = "ServiceUnavailable"
)

type APIGroup struct {
	TypeMeta `json:",inline"`
	// name is the name of the group.
	Name string `json:"name"`
	// versions are the versions supported in this group.
	Versions []GroupVersionForDiscovery `json:"versions"`
	// preferredVersion is the version preferred by the API server, which
	// probably is the storage version.
	PreferredVersion *GroupVersionForDiscovery `json:"preferredVersion,omitempty"`
	// a map of client CIDR to server address that is serving this group.
	// This is to help clients reach servers in the most network-efficient way possible.
	// Clients can use the appropriate server address as per the CIDR that they match.
	// In case of multiple matches, clients should use the longest matching CIDR.
	// The server returns only those CIDRs that it thinks that the client can match.
	// For example: the master will return an internal IP CIDR only, if the client reaches the server using an internal IP.
	// Server looks at X-Forwarded-For header or X-Real-Ip header or request.RemoteAddr (in that order) to get the client IP.
	ServerAddressByClientCIDRs []ServerAddressByClientCIDR `json:"serverAddressByClientCIDRs"`
}

func (in *APIGroup) DeepCopyInto(out *APIGroup) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.Versions != nil {
		l := make([]GroupVersionForDiscovery, len(in.Versions))
		for i := range in.Versions {
			in.Versions[i].DeepCopyInto(&l[i])
		}
		out.Versions = l
	}
	if in.PreferredVersion != nil {
		in, out := &in.PreferredVersion, &out.PreferredVersion
		*out = new(GroupVersionForDiscovery)
		(*in).DeepCopyInto(*out)
	}
	if in.ServerAddressByClientCIDRs != nil {
		l := make([]ServerAddressByClientCIDR, len(in.ServerAddressByClientCIDRs))
		for i := range in.ServerAddressByClientCIDRs {
			in.ServerAddressByClientCIDRs[i].DeepCopyInto(&l[i])
		}
		out.ServerAddressByClientCIDRs = l
	}
}

func (in *APIGroup) DeepCopy() *APIGroup {
	if in == nil {
		return nil
	}
	out := new(APIGroup)
	in.DeepCopyInto(out)
	return out
}

func (in *APIGroup) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type APIGroupList struct {
	TypeMeta `json:",inline"`
	// groups is a list of APIGroup.
	Groups []APIGroup `json:"groups"`
}

func (in *APIGroupList) DeepCopyInto(out *APIGroupList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.Groups != nil {
		l := make([]APIGroup, len(in.Groups))
		for i := range in.Groups {
			in.Groups[i].DeepCopyInto(&l[i])
		}
		out.Groups = l
	}
}

func (in *APIGroupList) DeepCopy() *APIGroupList {
	if in == nil {
		return nil
	}
	out := new(APIGroupList)
	in.DeepCopyInto(out)
	return out
}

func (in *APIGroupList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type APIResource struct {
	// name is the plural name of the resource.
	Name string `json:"name"`
	// singularName is the singular name of the resource.  This allows clients to handle plural and singular opaquely.
	// The singularName is more correct for reporting status on a single item and both singular and plural are allowed
	// from the kubectl CLI interface.
	SingularName string `json:"singularName"`
	// namespaced indicates if a resource is namespaced or not.
	Namespaced bool `json:"namespaced"`
	// group is the preferred group of the resource.  Empty implies the group of the containing resource list.
	// For subresources, this may have a different value, for example: Scale".
	Group string `json:"group,omitempty"`
	// version is the preferred version of the resource.  Empty implies the version of the containing resource list
	// For subresources, this may have a different value, for example: v1 (while inside a v1beta1 version of the core resource's group)".
	Version string `json:"version,omitempty"`
	// kind is the kind for the resource (e.g. 'Foo' is the kind for a resource 'foo')
	Kind string `json:"kind"`
	// verbs is a list of supported kube verbs (this includes get, list, watch, create,
	// update, patch, delete, deletecollection, and proxy)
	Verbs Verbs `json:"verbs"`
	// shortNames is a list of suggested short names of the resource.
	ShortNames []string `json:"shortNames"`
	// categories is a list of the grouped resources this resource belongs to (e.g. 'all')
	Categories []string `json:"categories"`
	// The hash value of the storage version, the version this resource is
	// converted to when written to the data store. Value must be treated
	// as opaque by clients. Only equality comparison on the value is valid.
	// This is an alpha feature and may change or be removed in the future.
	// The field is populated by the apiserver only if the
	// StorageVersionHash feature gate is enabled.
	// This field will remain optional even if it graduates.
	StorageVersionHash string `json:"storageVersionHash,omitempty"`
}

func (in *APIResource) DeepCopyInto(out *APIResource) {
	*out = *in
	copy(out.Verbs, in.Verbs)
	if in.ShortNames != nil {
		t := make([]string, len(in.ShortNames))
		copy(t, in.ShortNames)
		out.ShortNames = t
	}
	if in.Categories != nil {
		t := make([]string, len(in.Categories))
		copy(t, in.Categories)
		out.Categories = t
	}
}

func (in *APIResource) DeepCopy() *APIResource {
	if in == nil {
		return nil
	}
	out := new(APIResource)
	in.DeepCopyInto(out)
	return out
}

type APIResourceList struct {
	TypeMeta `json:",inline"`
	// groupVersion is the group and version this APIResourceList is for.
	GroupVersion string `json:"groupVersion"`
	// resources contains the name of the resources and if they are namespaced.
	APIResources []APIResource `json:"resources"`
}

func (in *APIResourceList) DeepCopyInto(out *APIResourceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.APIResources != nil {
		l := make([]APIResource, len(in.APIResources))
		for i := range in.APIResources {
			in.APIResources[i].DeepCopyInto(&l[i])
		}
		out.APIResources = l
	}
}

func (in *APIResourceList) DeepCopy() *APIResourceList {
	if in == nil {
		return nil
	}
	out := new(APIResourceList)
	in.DeepCopyInto(out)
	return out
}

func (in *APIResourceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type APIVersions struct {
	TypeMeta `json:",inline"`
	// versions are the api versions that are available.
	Versions []string `json:"versions"`
	// a map of client CIDR to server address that is serving this group.
	// This is to help clients reach servers in the most network-efficient way possible.
	// Clients can use the appropriate server address as per the CIDR that they match.
	// In case of multiple matches, clients should use the longest matching CIDR.
	// The server returns only those CIDRs that it thinks that the client can match.
	// For example: the master will return an internal IP CIDR only, if the client reaches the server using an internal IP.
	// Server looks at X-Forwarded-For header or X-Real-Ip header or request.RemoteAddr (in that order) to get the client IP.
	ServerAddressByClientCIDRs []ServerAddressByClientCIDR `json:"serverAddressByClientCIDRs"`
}

func (in *APIVersions) DeepCopyInto(out *APIVersions) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.Versions != nil {
		t := make([]string, len(in.Versions))
		copy(t, in.Versions)
		out.Versions = t
	}
	if in.ServerAddressByClientCIDRs != nil {
		l := make([]ServerAddressByClientCIDR, len(in.ServerAddressByClientCIDRs))
		for i := range in.ServerAddressByClientCIDRs {
			in.ServerAddressByClientCIDRs[i].DeepCopyInto(&l[i])
		}
		out.ServerAddressByClientCIDRs = l
	}
}

func (in *APIVersions) DeepCopy() *APIVersions {
	if in == nil {
		return nil
	}
	out := new(APIVersions)
	in.DeepCopyInto(out)
	return out
}

func (in *APIVersions) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type ApplyOptions struct {
	TypeMeta `json:",inline"`
	// When present, indicates that modifications should not be
	// persisted. An invalid or unrecognized dryRun directive will
	// result in an error response and no further processing of the
	// request. Valid values are:
	// - All: all dry run stages will be processed
	DryRun []string `json:"dryRun"`
	// Force is going to "force" Apply requests. It means user will
	// re-acquire conflicting fields owned by other people.
	Force bool `json:"force"`
	// fieldManager is a name associated with the actor or entity
	// that is making these changes. The value must be less than or
	// 128 characters long, and only contain printable characters,
	// as defined by https://golang.org/pkg/unicode/#IsPrint. This
	// field is required.
	FieldManager string `json:"fieldManager"`
}

func (in *ApplyOptions) DeepCopyInto(out *ApplyOptions) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.DryRun != nil {
		t := make([]string, len(in.DryRun))
		copy(t, in.DryRun)
		out.DryRun = t
	}
}

func (in *ApplyOptions) DeepCopy() *ApplyOptions {
	if in == nil {
		return nil
	}
	out := new(ApplyOptions)
	in.DeepCopyInto(out)
	return out
}

func (in *ApplyOptions) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type Condition struct {
	// type of condition in CamelCase or in foo.example.com/CamelCase.
	// ---
	// Many .condition.type values are consistent across resources like Available, but because arbitrary conditions can be
	// useful (see .node.status.conditions), the ability to deconflict is important.
	// The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)
	Type string `json:"type"`
	// status of the condition, one of True, False, Unknown.
	Status ConditionStatus `json:"status"`
	// observedGeneration represents the .metadata.generation that the condition was set based upon.
	// For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
	// with respect to the current state of the instance.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// lastTransitionTime is the last time the condition transitioned from one status to another.
	// This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.
	LastTransitionTime Time `json:"lastTransitionTime"`
	// reason contains a programmatic identifier indicating the reason for the condition's last transition.
	// Producers of specific condition types may define expected values and meanings for this field,
	// and whether the values are considered a guaranteed API.
	// The value should be a CamelCase string.
	// This field may not be empty.
	Reason string `json:"reason"`
	// message is a human readable message indicating details about the transition.
	// This may be an empty string.
	Message string `json:"message"`
}

func (in *Condition) DeepCopyInto(out *Condition) {
	*out = *in
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
}

func (in *Condition) DeepCopy() *Condition {
	if in == nil {
		return nil
	}
	out := new(Condition)
	in.DeepCopyInto(out)
	return out
}

type CreateOptions struct {
	TypeMeta `json:",inline"`
	// When present, indicates that modifications should not be
	// persisted. An invalid or unrecognized dryRun directive will
	// result in an error response and no further processing of the
	// request. Valid values are:
	// - All: all dry run stages will be processed
	DryRun []string `json:"dryRun"`
	// fieldManager is a name associated with the actor or entity
	// that is making these changes. The value must be less than or
	// 128 characters long, and only contain printable characters,
	// as defined by https://golang.org/pkg/unicode/#IsPrint.
	FieldManager string `json:"fieldManager,omitempty"`
	// fieldValidation instructs the server on how to handle
	// objects in the request (POST/PUT/PATCH) containing unknown
	// or duplicate fields. Valid values are:
	// - Ignore: This will ignore any unknown fields that are silently
	// dropped from the object, and will ignore all but the last duplicate
	// field that the decoder encounters. This is the default behavior
	// prior to v1.23.
	// - Warn: This will send a warning via the standard warning response
	// header for each unknown field that is dropped from the object, and
	// for each duplicate field that is encountered. The request will
	// still succeed if there are no other errors, and will only persist
	// the last of any duplicate fields. This is the default in v1.23+
	// - Strict: This will fail the request with a BadRequest error if
	// any unknown fields would be dropped from the object, or if any
	// duplicate fields are present. The error returned from the server
	// will contain all unknown and duplicate fields encountered.
	FieldValidation string `json:"fieldValidation,omitempty"`
}

func (in *CreateOptions) DeepCopyInto(out *CreateOptions) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.DryRun != nil {
		t := make([]string, len(in.DryRun))
		copy(t, in.DryRun)
		out.DryRun = t
	}
}

func (in *CreateOptions) DeepCopy() *CreateOptions {
	if in == nil {
		return nil
	}
	out := new(CreateOptions)
	in.DeepCopyInto(out)
	return out
}

func (in *CreateOptions) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type DeleteOptions struct {
	TypeMeta `json:",inline"`
	// The duration in seconds before the object should be deleted. Value must be non-negative integer.
	// The value zero indicates delete immediately. If this value is nil, the default grace period for the
	// specified type will be used.
	// Defaults to a per object value if not specified. zero means delete immediately.
	GracePeriodSeconds int64 `json:"gracePeriodSeconds,omitempty"`
	// Must be fulfilled before a deletion is carried out. If not possible, a 409 Conflict status will be
	// returned.
	Preconditions *Preconditions `json:"preconditions,omitempty"`
	// Deprecated: please use the PropagationPolicy, this field will be deprecated in 1.7.
	// Should the dependent objects be orphaned. If true/false, the "orphan"
	// finalizer will be added to/removed from the object's finalizers list.
	// Either this field or PropagationPolicy may be set, but not both.
	OrphanDependents bool `json:"orphanDependents,omitempty"`
	// Whether and how garbage collection will be performed.
	// Either this field or OrphanDependents may be set, but not both.
	// The default policy is decided by the existing finalizer set in the
	// metadata.finalizers and the resource-specific default policy.
	// Acceptable values are: 'Orphan' - orphan the dependents; 'Background' -
	// allow the garbage collector to delete the dependents in the background;
	// 'Foreground' - a cascading policy that deletes all dependents in the
	// foreground.
	PropagationPolicy DeletionPropagation `json:"propagationPolicy,omitempty"`
	// When present, indicates that modifications should not be
	// persisted. An invalid or unrecognized dryRun directive will
	// result in an error response and no further processing of the
	// request. Valid values are:
	// - All: all dry run stages will be processed
	DryRun []string `json:"dryRun"`
}

func (in *DeleteOptions) DeepCopyInto(out *DeleteOptions) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.Preconditions != nil {
		in, out := &in.Preconditions, &out.Preconditions
		*out = new(Preconditions)
		(*in).DeepCopyInto(*out)
	}
	if in.DryRun != nil {
		t := make([]string, len(in.DryRun))
		copy(t, in.DryRun)
		out.DryRun = t
	}
}

func (in *DeleteOptions) DeepCopy() *DeleteOptions {
	if in == nil {
		return nil
	}
	out := new(DeleteOptions)
	in.DeepCopyInto(out)
	return out
}

func (in *DeleteOptions) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type Duration struct {
	Duration int64 `json:"duration"`
}

func (in *Duration) DeepCopyInto(out *Duration) {
	*out = *in
}

func (in *Duration) DeepCopy() *Duration {
	if in == nil {
		return nil
	}
	out := new(Duration)
	in.DeepCopyInto(out)
	return out
}

type FieldsV1 struct {
	// Raw is the underlying serialization of this object.
	Raw []byte `json:"-,omitempty"`
}

func (in *FieldsV1) DeepCopyInto(out *FieldsV1) {
	*out = *in
}

func (in *FieldsV1) DeepCopy() *FieldsV1 {
	if in == nil {
		return nil
	}
	out := new(FieldsV1)
	in.DeepCopyInto(out)
	return out
}

type GetOptions struct {
	TypeMeta `json:",inline"`
	// resourceVersion sets a constraint on what resource versions a request may be served from.
	// See https://kubernetes.io/docs/reference/using-api/api-concepts/#resource-versions for
	// details.
	// Defaults to unset
	ResourceVersion string `json:"resourceVersion,omitempty"`
}

func (in *GetOptions) DeepCopyInto(out *GetOptions) {
	*out = *in
	out.TypeMeta = in.TypeMeta
}

func (in *GetOptions) DeepCopy() *GetOptions {
	if in == nil {
		return nil
	}
	out := new(GetOptions)
	in.DeepCopyInto(out)
	return out
}

func (in *GetOptions) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type GroupKind struct {
	Group string `json:"group"`
	Kind  string `json:"kind"`
}

func (in *GroupKind) DeepCopyInto(out *GroupKind) {
	*out = *in
}

func (in *GroupKind) DeepCopy() *GroupKind {
	if in == nil {
		return nil
	}
	out := new(GroupKind)
	in.DeepCopyInto(out)
	return out
}

type GroupResource struct {
	Group    string `json:"group"`
	Resource string `json:"resource"`
}

func (in *GroupResource) DeepCopyInto(out *GroupResource) {
	*out = *in
}

func (in *GroupResource) DeepCopy() *GroupResource {
	if in == nil {
		return nil
	}
	out := new(GroupResource)
	in.DeepCopyInto(out)
	return out
}

type GroupVersion struct {
	Group   string `json:"group"`
	Version string `json:"version"`
}

func (in *GroupVersion) DeepCopyInto(out *GroupVersion) {
	*out = *in
}

func (in *GroupVersion) DeepCopy() *GroupVersion {
	if in == nil {
		return nil
	}
	out := new(GroupVersion)
	in.DeepCopyInto(out)
	return out
}

type GroupVersionForDiscovery struct {
	// groupVersion specifies the API group and version in the form "group/version"
	GroupVersion string `json:"groupVersion"`
	// version specifies the version in the form of "version". This is to save
	// the clients the trouble of splitting the GroupVersion.
	Version string `json:"version"`
}

func (in *GroupVersionForDiscovery) DeepCopyInto(out *GroupVersionForDiscovery) {
	*out = *in
}

func (in *GroupVersionForDiscovery) DeepCopy() *GroupVersionForDiscovery {
	if in == nil {
		return nil
	}
	out := new(GroupVersionForDiscovery)
	in.DeepCopyInto(out)
	return out
}

type GroupVersionKind struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

func (in *GroupVersionKind) DeepCopyInto(out *GroupVersionKind) {
	*out = *in
}

func (in *GroupVersionKind) DeepCopy() *GroupVersionKind {
	if in == nil {
		return nil
	}
	out := new(GroupVersionKind)
	in.DeepCopyInto(out)
	return out
}

type GroupVersionResource struct {
	Group    string `json:"group"`
	Version  string `json:"version"`
	Resource string `json:"resource"`
}

func (in *GroupVersionResource) DeepCopyInto(out *GroupVersionResource) {
	*out = *in
}

func (in *GroupVersionResource) DeepCopy() *GroupVersionResource {
	if in == nil {
		return nil
	}
	out := new(GroupVersionResource)
	in.DeepCopyInto(out)
	return out
}

type LabelSelector struct {
	// matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
	// map is equivalent to an element of matchExpressions, whose key field is "key", the
	// operator is "In", and the values array contains only "value". The requirements are ANDed.
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
	// matchExpressions is a list of label selector requirements. The requirements are ANDed.
	MatchExpressions []LabelSelectorRequirement `json:"matchExpressions"`
}

func (in *LabelSelector) DeepCopyInto(out *LabelSelector) {
	*out = *in
	if in.MatchLabels != nil {
		in, out := &in.MatchLabels, &out.MatchLabels
		*out = make(map[string]string, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.MatchExpressions != nil {
		l := make([]LabelSelectorRequirement, len(in.MatchExpressions))
		for i := range in.MatchExpressions {
			in.MatchExpressions[i].DeepCopyInto(&l[i])
		}
		out.MatchExpressions = l
	}
}

func (in *LabelSelector) DeepCopy() *LabelSelector {
	if in == nil {
		return nil
	}
	out := new(LabelSelector)
	in.DeepCopyInto(out)
	return out
}

type LabelSelectorRequirement struct {
	// key is the label key that the selector applies to.
	Key string `json:"key"`
	// operator represents a key's relationship to a set of values.
	// Valid operators are In, NotIn, Exists and DoesNotExist.
	Operator LabelSelectorOperator `json:"operator"`
	// values is an array of string values. If the operator is In or NotIn,
	// the values array must be non-empty. If the operator is Exists or DoesNotExist,
	// the values array must be empty. This array is replaced during a strategic
	// merge patch.
	Values []string `json:"values"`
}

func (in *LabelSelectorRequirement) DeepCopyInto(out *LabelSelectorRequirement) {
	*out = *in
	if in.Values != nil {
		t := make([]string, len(in.Values))
		copy(t, in.Values)
		out.Values = t
	}
}

func (in *LabelSelectorRequirement) DeepCopy() *LabelSelectorRequirement {
	if in == nil {
		return nil
	}
	out := new(LabelSelectorRequirement)
	in.DeepCopyInto(out)
	return out
}

type List struct {
	TypeMeta `json:",inline"`
	// Standard list metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	ListMeta *ListMeta `json:"metadata,omitempty"`
	// List of objects
	Items []runtime.RawExtension `json:"items"`
}

func (in *List) DeepCopyInto(out *List) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.ListMeta != nil {
		in, out := &in.ListMeta, &out.ListMeta
		*out = new(ListMeta)
		(*in).DeepCopyInto(*out)
	}
	if in.Items != nil {
		l := make([]runtime.RawExtension, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *List) DeepCopy() *List {
	if in == nil {
		return nil
	}
	out := new(List)
	in.DeepCopyInto(out)
	return out
}

func (in *List) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type ListMeta struct {
	// Deprecated: selfLink is a legacy read-only field that is no longer populated by the system.
	SelfLink string `json:"selfLink,omitempty"`
	// String that identifies the server's internal version of this object that
	// can be used by clients to determine when objects have changed.
	// Value must be treated as opaque by clients and passed unmodified back to the server.
	// Populated by the system.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency
	ResourceVersion string `json:"resourceVersion,omitempty"`
	// continue may be set if the user set a limit on the number of items returned, and indicates that
	// the server has more data available. The value is opaque and may be used to issue another request
	// to the endpoint that served this list to retrieve the next set of available objects. Continuing a
	// consistent list may not be possible if the server configuration has changed or more than a few
	// minutes have passed. The resourceVersion field returned when using this continue value will be
	// identical to the value in the first response, unless you have received this token from an error
	// message.
	Continue string `json:"continue,omitempty"`
	// remainingItemCount is the number of subsequent items in the list which are not included in this
	// list response. If the list request contained label or field selectors, then the number of
	// remaining items is unknown and the field will be left unset and omitted during serialization.
	// If the list is complete (either because it is not chunking or because this is the last chunk),
	// then there are no more remaining items and this field will be left unset and omitted during
	// serialization.
	// Servers older than v1.15 do not set this field.
	// The intended use of the remainingItemCount is *estimating* the size of a collection. Clients
	// should not rely on the remainingItemCount to be set or to be exact.
	RemainingItemCount int64 `json:"remainingItemCount,omitempty"`
}

func (in *ListMeta) DeepCopyInto(out *ListMeta) {
	*out = *in
}

func (in *ListMeta) DeepCopy() *ListMeta {
	if in == nil {
		return nil
	}
	out := new(ListMeta)
	in.DeepCopyInto(out)
	return out
}

type ListOptions struct {
	TypeMeta `json:",inline"`
	// A selector to restrict the list of returned objects by their labels.
	// Defaults to everything.
	LabelSelector string `json:"labelSelector,omitempty"`
	// A selector to restrict the list of returned objects by their fields.
	// Defaults to everything.
	FieldSelector string `json:"fieldSelector,omitempty"`
	// Watch for changes to the described resources and return them as a stream of
	// add, update, and remove notifications. Specify resourceVersion.
	Watch bool `json:"watch,omitempty"`
	// allowWatchBookmarks requests watch events with type "BOOKMARK".
	// Servers that do not implement bookmarks may ignore this flag and
	// bookmarks are sent at the server's discretion. Clients should not
	// assume bookmarks are returned at any specific interval, nor may they
	// assume the server will send any BOOKMARK event during a session.
	// If this is not a watch, this field is ignored.
	AllowWatchBookmarks bool `json:"allowWatchBookmarks,omitempty"`
	// resourceVersion sets a constraint on what resource versions a request may be served from.
	// See https://kubernetes.io/docs/reference/using-api/api-concepts/#resource-versions for
	// details.
	// Defaults to unset
	ResourceVersion string `json:"resourceVersion,omitempty"`
	// resourceVersionMatch determines how resourceVersion is applied to list calls.
	// It is highly recommended that resourceVersionMatch be set for list calls where
	// resourceVersion is set
	// See https://kubernetes.io/docs/reference/using-api/api-concepts/#resource-versions for
	// details.
	// Defaults to unset
	ResourceVersionMatch ResourceVersionMatch `json:"resourceVersionMatch,omitempty"`
	// Timeout for the list/watch call.
	// This limits the duration of the call, regardless of any activity or inactivity.
	TimeoutSeconds int64 `json:"timeoutSeconds,omitempty"`
	// limit is a maximum number of responses to return for a list call. If more items exist, the
	// server will set the `continue` field on the list metadata to a value that can be used with the
	// same initial query to retrieve the next set of results. Setting a limit may return fewer than
	// the requested amount of items (up to zero items) in the event all requested objects are
	// filtered out and clients should only use the presence of the continue field to determine whether
	// more results are available. Servers may choose not to support the limit argument and will return
	// all of the available results. If limit is specified and the continue field is empty, clients may
	// assume that no more results are available. This field is not supported if watch is true.
	// The server guarantees that the objects returned when using continue will be identical to issuing
	// a single list call without a limit - that is, no objects created, modified, or deleted after the
	// first request is issued will be included in any subsequent continued requests. This is sometimes
	// referred to as a consistent snapshot, and ensures that a client that is using limit to receive
	// smaller chunks of a very large result can ensure they see all possible objects. If objects are
	// updated during a chunked list the version of the object that was present at the time the first list
	// result was calculated is returned.
	Limit int64 `json:"limit,omitempty"`
	// The continue option should be set when retrieving more results from the server. Since this value is
	// server defined, clients may only use the continue value from a previous query result with identical
	// query parameters (except for the value of continue) and the server may reject a continue value it
	// does not recognize. If the specified continue value is no longer valid whether due to expiration
	// (generally five to fifteen minutes) or a configuration change on the server, the server will
	// respond with a 410 ResourceExpired error together with a continue token. If the client needs a
	// consistent list, it must restart their list without the continue field. Otherwise, the client may
	// send another list request with the token received with the 410 error, the server will respond with
	// a list starting from the next key, but from the latest snapshot, which is inconsistent from the
	// previous list results - objects that are created, modified, or deleted after the first list request
	// will be included in the response, as long as their keys are after the "next key".
	// This field is not supported when watch is true. Clients may start a watch from the last
	// resourceVersion value returned by the server and not miss any modifications.
	Continue string `json:"continue,omitempty"`
	// `sendInitialEvents=true` may be set together with `watch=true`.
	// In that case, the watch stream will begin with synthetic events to
	// produce the current state of objects in the collection. Once all such
	// events have been sent, a synthetic "Bookmark" event  will be sent.
	// The bookmark will report the ResourceVersion (RV) corresponding to the
	// set of objects, and be marked with `"k8s.io/initial-events-end": "true"` annotation.
	// Afterwards, the watch stream will proceed as usual, sending watch events
	// corresponding to changes (subsequent to the RV) to objects watched.
	// When `sendInitialEvents` option is set, we require `resourceVersionMatch`
	// option to also be set. The semantic of the watch request is as following:
	// - `resourceVersionMatch` = NotOlderThan
	// is interpreted as "data at least as new as the provided `resourceVersion`"
	// and the bookmark event is send when the state is synced
	// to a `resourceVersion` at least as fresh as the one provided by the ListOptions.
	// If `resourceVersion` is unset, this is interpreted as "consistent read" and the
	// bookmark event is send when the state is synced at least to the moment
	// when request started being processed.
	// - `resourceVersionMatch` set to any other value or unset
	// Invalid error is returned.
	// Defaults to true if `resourceVersion=""` or `resourceVersion="0"` (for backward
	// compatibility reasons) and to false otherwise.
	SendInitialEvents bool `json:"sendInitialEvents,omitempty"`
}

func (in *ListOptions) DeepCopyInto(out *ListOptions) {
	*out = *in
	out.TypeMeta = in.TypeMeta
}

func (in *ListOptions) DeepCopy() *ListOptions {
	if in == nil {
		return nil
	}
	out := new(ListOptions)
	in.DeepCopyInto(out)
	return out
}

func (in *ListOptions) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type ManagedFieldsEntry struct {
	// Manager is an identifier of the workflow managing these fields.
	Manager string `json:"manager,omitempty"`
	// Operation is the type of operation which lead to this ManagedFieldsEntry being created.
	// The only valid values for this field are 'Apply' and 'Update'.
	Operation ManagedFieldsOperationType `json:"operation,omitempty"`
	// APIVersion defines the version of this resource that this field set
	// applies to. The format is "group/version" just like the top-level
	// APIVersion field. It is necessary to track the version of a field
	// set because it cannot be automatically converted.
	APIVersion string `json:"apiVersion,omitempty"`
	// Time is the timestamp of when the ManagedFields entry was added. The
	// timestamp will also be updated if a field is added, the manager
	// changes any of the owned fields value or removes a field. The
	// timestamp does not update when a field is removed from the entry
	// because another manager took it over.
	Time *Time `json:"time,omitempty"`
	// FieldsType is the discriminator for the different fields format and version.
	// There is currently only one possible value: "FieldsV1"
	FieldsType string `json:"fieldsType,omitempty"`
	// Fieldsv1 holds the first JSON version format as described in the "FieldsV1" type.
	FieldsV1 *FieldsV1 `json:"fieldsV1,omitempty"`
	// Subresource is the name of the subresource used to update that object, or
	// empty string if the object was updated through the main resource. The
	// value of this field is used to distinguish between managers, even if they
	// share the same name. For example, a status update will be distinct from a
	// regular update using the same manager name.
	// Note that the APIVersion field is not related to the Subresource field and
	// it always corresponds to the version of the main resource.
	Subresource string `json:"subresource,omitempty"`
}

func (in *ManagedFieldsEntry) DeepCopyInto(out *ManagedFieldsEntry) {
	*out = *in
	if in.Time != nil {
		in, out := &in.Time, &out.Time
		*out = new(Time)
		(*in).DeepCopyInto(*out)
	}
	if in.FieldsV1 != nil {
		in, out := &in.FieldsV1, &out.FieldsV1
		*out = new(FieldsV1)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ManagedFieldsEntry) DeepCopy() *ManagedFieldsEntry {
	if in == nil {
		return nil
	}
	out := new(ManagedFieldsEntry)
	in.DeepCopyInto(out)
	return out
}

type MicroTime struct {
	// Represents seconds of UTC time since Unix epoch
	// 1970-01-01T00:00:00Z. Must be from 0001-01-01T00:00:00Z to
	// 9999-12-31T23:59:59Z inclusive.
	Seconds int64 `json:"seconds"`
	// Non-negative fractions of a second at nanosecond resolution. Negative
	// second values with fractions must still have non-negative nanos values
	// that count forward in time. Must be from 0 to 999,999,999
	// inclusive. This field may be limited in precision depending on context.
	Nanos int `json:"nanos"`
}

func (in *MicroTime) DeepCopyInto(out *MicroTime) {
	*out = *in
}

func (in *MicroTime) DeepCopy() *MicroTime {
	if in == nil {
		return nil
	}
	out := new(MicroTime)
	in.DeepCopyInto(out)
	return out
}

type ObjectMeta struct {
	// Name must be unique within a namespace. Is required when creating resources, although
	// some resources may allow a client to request the generation of an appropriate name
	// automatically. Name is primarily intended for creation idempotence and configuration
	// definition.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names#names
	Name string `json:"name,omitempty"`
	// GenerateName is an optional prefix, used by the server, to generate a unique
	// name ONLY IF the Name field has not been provided.
	// If this field is used, the name returned to the client will be different
	// than the name passed. This value will also be combined with a unique suffix.
	// The provided value has the same validation rules as the Name field,
	// and may be truncated by the length of the suffix required to make the value
	// unique on the server.
	// If this field is specified and the generated name exists, the server will return a 409.
	// Applied only if Name is not specified.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#idempotency
	GenerateName string `json:"generateName,omitempty"`
	// Namespace defines the space within which each name must be unique. An empty namespace is
	// equivalent to the "default" namespace, but "default" is the canonical representation.
	// Not all objects are required to be scoped to a namespace - the value of this field for
	// those objects will be empty.
	// Must be a DNS_LABEL.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces
	Namespace string `json:"namespace,omitempty"`
	// Deprecated: selfLink is a legacy read-only field that is no longer populated by the system.
	SelfLink string `json:"selfLink,omitempty"`
	// UID is the unique in time and space value for this object. It is typically generated by
	// the server on successful creation of a resource and is not allowed to change on PUT
	// operations.
	// Populated by the system.
	// Read-only.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names#uids
	UID string `json:"uid,omitempty"`
	// An opaque value that represents the internal version of this object that can
	// be used by clients to determine when objects have changed. May be used for optimistic
	// concurrency, change detection, and the watch operation on a resource or set of resources.
	// Clients must treat these values as opaque and passed unmodified back to the server.
	// They may only be valid for a particular resource or set of resources.
	// Populated by the system.
	// Read-only.
	// Value must be treated as opaque by clients and .
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency
	ResourceVersion string `json:"resourceVersion,omitempty"`
	// A sequence number representing a specific generation of the desired state.
	// Populated by the system. Read-only.
	Generation int64 `json:"generation,omitempty"`
	// CreationTimestamp is a timestamp representing the server time when this object was
	// created. It is not guaranteed to be set in happens-before order across separate operations.
	// Clients may not set this value. It is represented in RFC3339 form and is in UTC.
	// Populated by the system.
	// Read-only.
	// Null for lists.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	CreationTimestamp *Time `json:"creationTimestamp,omitempty"`
	// DeletionTimestamp is RFC 3339 date and time at which this resource will be deleted. This
	// field is set by the server when a graceful deletion is requested by the user, and is not
	// directly settable by a client. The resource is expected to be deleted (no longer visible
	// from resource lists, and not reachable by name) after the time in this field, once the
	// finalizers list is empty. As long as the finalizers list contains items, deletion is blocked.
	// Once the deletionTimestamp is set, this value may not be unset or be set further into the
	// future, although it may be shortened or the resource may be deleted prior to this time.
	// For example, a user may request that a pod is deleted in 30 seconds. The Kubelet will react
	// by sending a graceful termination signal to the containers in the pod. After that 30 seconds,
	// the Kubelet will send a hard termination signal (SIGKILL) to the container and after cleanup,
	// remove the pod from the API. In the presence of network partitions, this object may still
	// exist after this timestamp, until an administrator or automated process can determine the
	// resource is fully terminated.
	// If not set, graceful deletion of the object has not been requested.
	// Populated by the system when a graceful deletion is requested.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	DeletionTimestamp *Time `json:"deletionTimestamp,omitempty"`
	// Number of seconds allowed for this object to gracefully terminate before
	// it will be removed from the system. Only set when deletionTimestamp is also set.
	// May only be shortened.
	// Read-only.
	DeletionGracePeriodSeconds int64 `json:"deletionGracePeriodSeconds,omitempty"`
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations
	Annotations map[string]string `json:"annotations,omitempty"`
	// List of objects depended by this object. If ALL objects in the list have
	// been deleted, this object will be garbage collected. If this object is managed by a controller,
	// then an entry in this list will point to this controller, with the controller field set to true.
	// There cannot be more than one managing controller.
	OwnerReferences []OwnerReference `json:"ownerReferences"`
	// Must be empty before the object is deleted from the registry. Each entry
	// is an identifier for the responsible component that will remove the entry
	// from the list. If the deletionTimestamp of the object is non-nil, entries
	// in this list can only be removed.
	// Finalizers may be processed and removed in any order.  Order is NOT enforced
	// because it introduces significant risk of stuck finalizers.
	// finalizers is a shared field, any actor with permission can reorder it.
	// If the finalizer list is processed in order, then this can lead to a situation
	// in which the component responsible for the first finalizer in the list is
	// waiting for a signal (field value, external system, or other) produced by a
	// component responsible for a finalizer later in the list, resulting in a deadlock.
	// Without enforced ordering finalizers are free to order amongst themselves and
	// are not vulnerable to ordering changes in the list.
	Finalizers []string `json:"finalizers"`
	// ManagedFields maps workflow-id and version to the set of fields
	// that are managed by that workflow. This is mostly for internal
	// housekeeping, and users typically shouldn't need to set or
	// understand this field. A workflow can be the user's name, a
	// controller's name, or the name of a specific apply path like
	// "ci-cd". The set of fields is always in the version that the
	// workflow used when modifying the object.
	ManagedFields []ManagedFieldsEntry `json:"managedFields"`
}

func (in *ObjectMeta) DeepCopyInto(out *ObjectMeta) {
	*out = *in
	if in.CreationTimestamp != nil {
		in, out := &in.CreationTimestamp, &out.CreationTimestamp
		*out = new(Time)
		(*in).DeepCopyInto(*out)
	}
	if in.DeletionTimestamp != nil {
		in, out := &in.DeletionTimestamp, &out.DeletionTimestamp
		*out = new(Time)
		(*in).DeepCopyInto(*out)
	}
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.OwnerReferences != nil {
		l := make([]OwnerReference, len(in.OwnerReferences))
		for i := range in.OwnerReferences {
			in.OwnerReferences[i].DeepCopyInto(&l[i])
		}
		out.OwnerReferences = l
	}
	if in.Finalizers != nil {
		t := make([]string, len(in.Finalizers))
		copy(t, in.Finalizers)
		out.Finalizers = t
	}
	if in.ManagedFields != nil {
		l := make([]ManagedFieldsEntry, len(in.ManagedFields))
		for i := range in.ManagedFields {
			in.ManagedFields[i].DeepCopyInto(&l[i])
		}
		out.ManagedFields = l
	}
}

func (in *ObjectMeta) DeepCopy() *ObjectMeta {
	if in == nil {
		return nil
	}
	out := new(ObjectMeta)
	in.DeepCopyInto(out)
	return out
}

type OwnerReference struct {
	// API version of the referent.
	APIVersion string `json:"apiVersion"`
	// Kind of the referent.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	Kind string `json:"kind"`
	// Name of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names#names
	Name string `json:"name"`
	// UID of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names#uids
	UID string `json:"uid"`
	// If true, this reference points to the managing controller.
	Controller bool `json:"controller,omitempty"`
	// If true, AND if the owner has the "foregroundDeletion" finalizer, then
	// the owner cannot be deleted from the key-value store until this
	// reference is removed.
	// See https://kubernetes.io/docs/concepts/architecture/garbage-collection/#foreground-deletion
	// for how the garbage collector interacts with this field and enforces the foreground deletion.
	// Defaults to false.
	// To set this field, a user needs "delete" permission of the owner,
	// otherwise 422 (Unprocessable Entity) will be returned.
	BlockOwnerDeletion bool `json:"blockOwnerDeletion,omitempty"`
}

func (in *OwnerReference) DeepCopyInto(out *OwnerReference) {
	*out = *in
}

func (in *OwnerReference) DeepCopy() *OwnerReference {
	if in == nil {
		return nil
	}
	out := new(OwnerReference)
	in.DeepCopyInto(out)
	return out
}

type PartialObjectMetadata struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata"`
}

func (in *PartialObjectMetadata) DeepCopyInto(out *PartialObjectMetadata) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
}

func (in *PartialObjectMetadata) DeepCopy() *PartialObjectMetadata {
	if in == nil {
		return nil
	}
	out := new(PartialObjectMetadata)
	in.DeepCopyInto(out)
	return out
}

func (in *PartialObjectMetadata) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type PartialObjectMetadataList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:"metadata"`
	Items    []PartialObjectMetadata `json:"items"`
}

func (in *PartialObjectMetadataList) DeepCopyInto(out *PartialObjectMetadataList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]PartialObjectMetadata, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *PartialObjectMetadataList) DeepCopy() *PartialObjectMetadataList {
	if in == nil {
		return nil
	}
	out := new(PartialObjectMetadataList)
	in.DeepCopyInto(out)
	return out
}

func (in *PartialObjectMetadataList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type Patch struct {
}

func (in *Patch) DeepCopyInto(out *Patch) {
	*out = *in
}

func (in *Patch) DeepCopy() *Patch {
	if in == nil {
		return nil
	}
	out := new(Patch)
	in.DeepCopyInto(out)
	return out
}

type PatchOptions struct {
	TypeMeta `json:",inline"`
	// When present, indicates that modifications should not be
	// persisted. An invalid or unrecognized dryRun directive will
	// result in an error response and no further processing of the
	// request. Valid values are:
	// - All: all dry run stages will be processed
	DryRun []string `json:"dryRun"`
	// Force is going to "force" Apply requests. It means user will
	// re-acquire conflicting fields owned by other people. Force
	// flag must be unset for non-apply patch requests.
	Force bool `json:"force,omitempty"`
	// fieldManager is a name associated with the actor or entity
	// that is making these changes. The value must be less than or
	// 128 characters long, and only contain printable characters,
	// as defined by https://golang.org/pkg/unicode/#IsPrint. This
	// field is required for apply requests
	// (application/apply-patch) but optional for non-apply patch
	// types (JsonPatch, MergePatch, StrategicMergePatch).
	FieldManager string `json:"fieldManager,omitempty"`
	// fieldValidation instructs the server on how to handle
	// objects in the request (POST/PUT/PATCH) containing unknown
	// or duplicate fields. Valid values are:
	// - Ignore: This will ignore any unknown fields that are silently
	// dropped from the object, and will ignore all but the last duplicate
	// field that the decoder encounters. This is the default behavior
	// prior to v1.23.
	// - Warn: This will send a warning via the standard warning response
	// header for each unknown field that is dropped from the object, and
	// for each duplicate field that is encountered. The request will
	// still succeed if there are no other errors, and will only persist
	// the last of any duplicate fields. This is the default in v1.23+
	// - Strict: This will fail the request with a BadRequest error if
	// any unknown fields would be dropped from the object, or if any
	// duplicate fields are present. The error returned from the server
	// will contain all unknown and duplicate fields encountered.
	FieldValidation string `json:"fieldValidation,omitempty"`
}

func (in *PatchOptions) DeepCopyInto(out *PatchOptions) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.DryRun != nil {
		t := make([]string, len(in.DryRun))
		copy(t, in.DryRun)
		out.DryRun = t
	}
}

func (in *PatchOptions) DeepCopy() *PatchOptions {
	if in == nil {
		return nil
	}
	out := new(PatchOptions)
	in.DeepCopyInto(out)
	return out
}

func (in *PatchOptions) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type Preconditions struct {
	// Specifies the target UID.
	UID string `json:"uid,omitempty"`
	// Specifies the target ResourceVersion
	ResourceVersion string `json:"resourceVersion,omitempty"`
}

func (in *Preconditions) DeepCopyInto(out *Preconditions) {
	*out = *in
}

func (in *Preconditions) DeepCopy() *Preconditions {
	if in == nil {
		return nil
	}
	out := new(Preconditions)
	in.DeepCopyInto(out)
	return out
}

type RootPaths struct {
	// paths are the paths available at root.
	Paths []string `json:"paths"`
}

func (in *RootPaths) DeepCopyInto(out *RootPaths) {
	*out = *in
	if in.Paths != nil {
		t := make([]string, len(in.Paths))
		copy(t, in.Paths)
		out.Paths = t
	}
}

func (in *RootPaths) DeepCopy() *RootPaths {
	if in == nil {
		return nil
	}
	out := new(RootPaths)
	in.DeepCopyInto(out)
	return out
}

type ServerAddressByClientCIDR struct {
	// The CIDR with which clients can match their IP to figure out the server address that they should use.
	ClientCIDR string `json:"clientCIDR"`
	// Address of this server, suitable for a client that matches the above CIDR.
	// This can be a hostname, hostname:port, IP or IP:port.
	ServerAddress string `json:"serverAddress"`
}

func (in *ServerAddressByClientCIDR) DeepCopyInto(out *ServerAddressByClientCIDR) {
	*out = *in
}

func (in *ServerAddressByClientCIDR) DeepCopy() *ServerAddressByClientCIDR {
	if in == nil {
		return nil
	}
	out := new(ServerAddressByClientCIDR)
	in.DeepCopyInto(out)
	return out
}

type Status struct {
	TypeMeta `json:",inline"`
	// Standard list metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	ListMeta *ListMeta `json:"metadata,omitempty"`
	// Status of the operation.
	// One of: "Success" or "Failure".
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Status string `json:"status,omitempty"`
	// A human-readable description of the status of this operation.
	Message string `json:"message,omitempty"`
	// A machine-readable description of why this operation is in the
	// "Failure" status. If this value is empty there
	// is no information available. A Reason clarifies an HTTP status
	// code but does not override it.
	Reason StatusReason `json:"reason,omitempty"`
	// Extended data associated with the reason.  Each reason may define its
	// own extended details. This field is optional and the data returned
	// is not guaranteed to conform to any schema except that defined by
	// the reason type.
	Details *StatusDetails `json:"details,omitempty"`
	// Suggested HTTP return code for this status, 0 if not set.
	Code int `json:"code,omitempty"`
}

func (in *Status) DeepCopyInto(out *Status) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.ListMeta != nil {
		in, out := &in.ListMeta, &out.ListMeta
		*out = new(ListMeta)
		(*in).DeepCopyInto(*out)
	}
	if in.Details != nil {
		in, out := &in.Details, &out.Details
		*out = new(StatusDetails)
		(*in).DeepCopyInto(*out)
	}
}

func (in *Status) DeepCopy() *Status {
	if in == nil {
		return nil
	}
	out := new(Status)
	in.DeepCopyInto(out)
	return out
}

func (in *Status) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type StatusCause struct {
	// A machine-readable description of the cause of the error. If this value is
	// empty there is no information available.
	Type CauseType `json:"reason,omitempty"`
	// A human-readable description of the cause of the error.  This field may be
	// presented as-is to a reader.
	Message string `json:"message,omitempty"`
	// The field of the resource that has caused this error, as named by its JSON
	// serialization. May include dot and postfix notation for nested attributes.
	// Arrays are zero-indexed.  Fields may appear more than once in an array of
	// causes due to fields having multiple errors.
	// Optional.
	// Examples:
	// "name" - the field "name" on the current resource
	// "items[0].name" - the field "name" on the first array entry in "items"
	Field string `json:"field,omitempty"`
}

func (in *StatusCause) DeepCopyInto(out *StatusCause) {
	*out = *in
}

func (in *StatusCause) DeepCopy() *StatusCause {
	if in == nil {
		return nil
	}
	out := new(StatusCause)
	in.DeepCopyInto(out)
	return out
}

type StatusDetails struct {
	// The name attribute of the resource associated with the status StatusReason
	// (when there is a single name which can be described).
	Name string `json:"name,omitempty"`
	// The group attribute of the resource associated with the status StatusReason.
	Group string `json:"group,omitempty"`
	// The kind attribute of the resource associated with the status StatusReason.
	// On some operations may differ from the requested resource Kind.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	Kind string `json:"kind,omitempty"`
	// UID of the resource.
	// (when there is a single resource which can be described).
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names#uids
	UID string `json:"uid,omitempty"`
	// The Causes array includes more details associated with the StatusReason
	// failure. Not all StatusReasons may provide detailed causes.
	Causes []StatusCause `json:"causes"`
	// If specified, the time in seconds before the operation should be retried. Some errors may indicate
	// the client must take an alternate action - for those errors this field may indicate how long to wait
	// before taking the alternate action.
	RetryAfterSeconds int `json:"retryAfterSeconds,omitempty"`
}

func (in *StatusDetails) DeepCopyInto(out *StatusDetails) {
	*out = *in
	if in.Causes != nil {
		l := make([]StatusCause, len(in.Causes))
		for i := range in.Causes {
			in.Causes[i].DeepCopyInto(&l[i])
		}
		out.Causes = l
	}
}

func (in *StatusDetails) DeepCopy() *StatusDetails {
	if in == nil {
		return nil
	}
	out := new(StatusDetails)
	in.DeepCopyInto(out)
	return out
}

type TableOptions struct {
	TypeMeta `json:",inline"`
	// includeObject decides whether to include each object along with its columnar information.
	// Specifying "None" will return no object, specifying "Object" will return the full object contents, and
	// specifying "Metadata" (the default) will return the object's metadata in the PartialObjectMetadata kind
	// in version v1beta1 of the meta.k8s.io API group.
	IncludeObject IncludeObjectPolicy `json:"includeObject,omitempty"`
}

func (in *TableOptions) DeepCopyInto(out *TableOptions) {
	*out = *in
	out.TypeMeta = in.TypeMeta
}

func (in *TableOptions) DeepCopy() *TableOptions {
	if in == nil {
		return nil
	}
	out := new(TableOptions)
	in.DeepCopyInto(out)
	return out
}

func (in *TableOptions) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type Time struct {
	// Represents seconds of UTC time since Unix epoch
	// 1970-01-01T00:00:00Z. Must be from 0001-01-01T00:00:00Z to
	// 9999-12-31T23:59:59Z inclusive.
	Seconds int64 `json:"seconds"`
	// Non-negative fractions of a second at nanosecond resolution. Negative
	// second values with fractions must still have non-negative nanos values
	// that count forward in time. Must be from 0 to 999,999,999
	// inclusive. This field may be limited in precision depending on context.
	Nanos int `json:"nanos"`
}

func (in *Time) DeepCopyInto(out *Time) {
	*out = *in
}

func (in *Time) DeepCopy() *Time {
	if in == nil {
		return nil
	}
	out := new(Time)
	in.DeepCopyInto(out)
	return out
}

type Timestamp struct {
	// Represents seconds of UTC time since Unix epoch
	// 1970-01-01T00:00:00Z. Must be from 0001-01-01T00:00:00Z to
	// 9999-12-31T23:59:59Z inclusive.
	Seconds int64 `json:"seconds"`
	// Non-negative fractions of a second at nanosecond resolution. Negative
	// second values with fractions must still have non-negative nanos values
	// that count forward in time. Must be from 0 to 999,999,999
	// inclusive. This field may be limited in precision depending on context.
	Nanos int `json:"nanos"`
}

func (in *Timestamp) DeepCopyInto(out *Timestamp) {
	*out = *in
}

func (in *Timestamp) DeepCopy() *Timestamp {
	if in == nil {
		return nil
	}
	out := new(Timestamp)
	in.DeepCopyInto(out)
	return out
}

type TypeMeta struct {
	// Kind is a string value representing the REST resource this object represents.
	// Servers may infer this from the endpoint the client submits requests to.
	// Cannot be updated.
	// In CamelCase.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	Kind string `json:"kind,omitempty"`
	// APIVersion defines the versioned schema of this representation of an object.
	// Servers should convert recognized schemas to the latest internal value, and
	// may reject unrecognized values.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
	APIVersion string `json:"apiVersion,omitempty"`
}

func (in *TypeMeta) DeepCopyInto(out *TypeMeta) {
	*out = *in
}

func (in *TypeMeta) DeepCopy() *TypeMeta {
	if in == nil {
		return nil
	}
	out := new(TypeMeta)
	in.DeepCopyInto(out)
	return out
}

type UpdateOptions struct {
	TypeMeta `json:",inline"`
	// When present, indicates that modifications should not be
	// persisted. An invalid or unrecognized dryRun directive will
	// result in an error response and no further processing of the
	// request. Valid values are:
	// - All: all dry run stages will be processed
	DryRun []string `json:"dryRun"`
	// fieldManager is a name associated with the actor or entity
	// that is making these changes. The value must be less than or
	// 128 characters long, and only contain printable characters,
	// as defined by https://golang.org/pkg/unicode/#IsPrint.
	FieldManager string `json:"fieldManager,omitempty"`
	// fieldValidation instructs the server on how to handle
	// objects in the request (POST/PUT/PATCH) containing unknown
	// or duplicate fields. Valid values are:
	// - Ignore: This will ignore any unknown fields that are silently
	// dropped from the object, and will ignore all but the last duplicate
	// field that the decoder encounters. This is the default behavior
	// prior to v1.23.
	// - Warn: This will send a warning via the standard warning response
	// header for each unknown field that is dropped from the object, and
	// for each duplicate field that is encountered. The request will
	// still succeed if there are no other errors, and will only persist
	// the last of any duplicate fields. This is the default in v1.23+
	// - Strict: This will fail the request with a BadRequest error if
	// any unknown fields would be dropped from the object, or if any
	// duplicate fields are present. The error returned from the server
	// will contain all unknown and duplicate fields encountered.
	FieldValidation string `json:"fieldValidation,omitempty"`
}

func (in *UpdateOptions) DeepCopyInto(out *UpdateOptions) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.DryRun != nil {
		t := make([]string, len(in.DryRun))
		copy(t, in.DryRun)
		out.DryRun = t
	}
}

func (in *UpdateOptions) DeepCopy() *UpdateOptions {
	if in == nil {
		return nil
	}
	out := new(UpdateOptions)
	in.DeepCopyInto(out)
	return out
}

func (in *UpdateOptions) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type Verbs []string

type WatchEvent struct {
	Type string `json:"type"`
	// Object is:
	// * If Type is Added or Modified: the new state of the object.
	// * If Type is Deleted: the state of the object immediately before deletion.
	// * If Type is Error: *Status is recommended; other types may make sense
	// depending on context.
	Object runtime.RawExtension `json:"object"`
}

func (in *WatchEvent) DeepCopyInto(out *WatchEvent) {
	*out = *in
	in.Object.DeepCopyInto(&out.Object)
}

func (in *WatchEvent) DeepCopy() *WatchEvent {
	if in == nil {
		return nil
	}
	out := new(WatchEvent)
	in.DeepCopyInto(out)
	return out
}
