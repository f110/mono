package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BucketFinalizePolicy string

const (
	BucketDelete BucketFinalizePolicy = "delete"
	BucketKeep   BucketFinalizePolicy = "keep"
)

type BucketPolicy string

const (
	PolicyPublic   BucketPolicy = "public"
	PolicyReadOnly BucketPolicy = "readOnly"
	PolicyPrivate  BucketPolicy = "private"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="ready",type="string",JSONPath=".status.ready",description="Ready",format="byte",priority=0
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp",description="age",format="date",priority=0

type MinIOBucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MinIOBucketSpec   `json:"spec,omitempty"`
	Status MinIOBucketStatus `json:"status,omitempty"`
}

type MinIOBucketSpec struct {
	// Selector is a selector of MinioInstance
	Selector metav1.LabelSelector `json:"selector"`
	// FinalizePolicy is a policy when deleted CR Object.
	// If FinalizePolicy is an empty string, then it is the same as "keep"
	FinalizePolicy BucketFinalizePolicy `json:"bucketFinalizePolicy,omitempty"`
	// Policy is the policy of bucket. One of public, readOnly, private.
	// If you don't want to give public access, set private or an empty value.
	// If it is an empty value, The bucket will not have any policy.
	// Currently, MinIOBucket can't use prefix based policy.
	Policy BucketPolicy `json:"policy,omitempty"`
}

type MinIOBucketStatus struct {
	Ready bool `json:"ready,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MinIOBucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []MinIOBucket `json:"items"`
}
