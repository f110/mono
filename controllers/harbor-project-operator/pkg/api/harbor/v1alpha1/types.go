package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="ready",type="boolean",JSONPath=".status.ready",description="Ready",format="byte",priority=0
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp",description="age",format="date",priority=0

type HarborProject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HarborProjectSpec   `json:"spec,omitempty"`
	Status HarborProjectStatus `json:"status,omitempty"`
}

type HarborProjectSpec struct {
	// Public is an access level of project.
	// If Public sets true, then anyone can read.
	Public bool `json:"public,omitempty"`
}

type HarborProjectStatus struct {
	Ready     bool   `json:"ready,omitempty"`
	ProjectId int    `json:"project_id,omitempty"`
	Registry  string `json:"registry,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type HarborProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []HarborProject `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type HarborRobotAccount struct {
	// If you want to create a robot account for this project, you set RobotAccount.
	// The secret of robot account for docker is created on same namespace.
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HarborRobotAccountSpec   `json:"spec,omitempty"`
	Status HarborRobotAccountStatus `json:"status,omitempty"`
}

type HarborRobotAccountSpec struct {
	ProjectNamespace string `json:"project_namespace"`
	ProjectName      string `json:"project_name"`
	// SecretName is a name of docker config secret.
	SecretName string `json:"secret_name,omitempty"`
}

type HarborRobotAccountStatus struct {
	Ready   bool `json:"ready,omitempty"`
	RobotId int  `json:"robot_id,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type HarborRobotAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []HarborRobotAccount `json:"items"`
}
