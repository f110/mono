package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type HarborProject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HarborProjectSpec   `json:"spec,omitempty"`
	Status HarborProjectStatus `json:"status,omitempty"`
}

type HarborProjectSpec struct {
	// Public is a access level of project.
	// If Public sets true, then anyone can read.
	Public bool `json:"public,omitempty"`
}

type HarborProjectStatus struct {
	Ready bool `json:"ready,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type HarborProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []HarborProject `json:"items"`
}
