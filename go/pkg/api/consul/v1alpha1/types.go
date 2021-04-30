package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
type ConsulBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConsulBackupSpec   `json:"spec,omitempty"`
	Status ConsulBackupStatus `json:"status,omitempty"`
}

type ConsulBackupSpec struct {
	IntervalInSecond int                     `json:"intervalInSeconds"`
	MaxBackups       int                     `json:"maxBackups"`
	Storage          ConsulBackupStorageSpec `json:"storage,omitempty"`
}

type ConsulBackupStorageSpec struct {
	MinIO *BackupStorageMinIOSpec `json:"minio,omitempty"`
	GCS   *BackupStorageGCSSpec   `json:"gcs,omitempty"`
}

type BackupStorageMinIOSpec struct {
	Service    *ObjectReference `json:"service"`
	Credential AWSCredential    `json:"credential"`
	Bucket     string           `json:"bucket"`
	Path       string           `json:"path"`
	Secure     bool             `json:"secure,omitempty"`
}

type ObjectReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

type AWSCredential struct {
	AccessKeyID     *corev1.SecretKeySelector `json:"accessKeyID"`
	SecretAccessKey *corev1.SecretKeySelector `json:"secretAccessKey"`
}

type BackupStorageGCSSpec struct {
	Bucket     string        `json:"bucket,omitempty"`
	Path       string        `json:"path,omitempty"`
	Credential GCPCredential `json:"credential,omitempty"`
}

type GCPCredential struct {
	ServiceAccountJSONKey *corev1.SecretKeySelector `json:"serviceAccountJSONKey,omitempty"`
}

type ConsulBackupStatus struct {
	Succeeded         bool                        `json:"succeeded"`
	LastSucceededTime *metav1.Time                `json:"lastSucceededTime,omitempty"`
	History           []ConsulBackupStatusHistory `json:"backupStatusHistory,omitempty"`
}

type ConsulBackupStatusHistory struct {
	Succeeded   bool         `json:"succeeded,omitempty"`
	ExecuteTime *metav1.Time `json:"executeTime,omitempty"`
	Path        string       `json:"path,omitempty"`
	Message     string       `json:"message,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ConsulBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ConsulBackup `json:"items"`
}
