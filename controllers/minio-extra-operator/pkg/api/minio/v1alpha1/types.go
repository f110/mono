package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MinIOBucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MinIOBucketSpec   `json:"spec,omitempty"`
	Status MinIOBucketStatus `json:"status,omitempty"`
}

type MinIOBucketSpec struct {
	// Selector is a selector of MinioInstance
	Selector metav1.LabelSelector `json:"selector"`
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
