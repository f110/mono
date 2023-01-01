package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
type Grafana struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GrafanaSpec   `json:"spec,omitempty"`
	Status GrafanaStatus `json:"status,omitempty"`
}

type GrafanaSpec struct {
	UserSelector        metav1.LabelSelector         `json:"userSelector"`
	AdminUser           string                       `json:"adminUser,omitempty"`
	AdminPasswordSecret *corev1.SecretKeySelector    `json:"adminPasswordSecret,omitempty"`
	Service             *corev1.LocalObjectReference `json:"service,omitempty"`
}

type GrafanaStatus struct {
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type GrafanaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Grafana `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
type GrafanaUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GrafanaUserSpec   `json:"spec,omitempty"`
	Status GrafanaUserStatus `json:"status,omitempty"`
}

type GrafanaUserSpec struct {
	Email string `json:"email"`
	Admin bool   `json:"admin,omitempty"`
}

type GrafanaUserStatus struct {
	Ready bool `json:"ready,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GrafanaUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []GrafanaUser `json:"items"`
}
