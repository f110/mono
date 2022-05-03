package consulv1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ConsulBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              ConsulBackupSpec   `json:"spec"`
	Status            ConsulBackupStatus `json:"status"`
}

func (in *ConsulBackup) DeepCopyInto(out *ConsulBackup) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *ConsulBackup) DeepCopy() *ConsulBackup {
	if in == nil {
		return nil
	}
	out := new(ConsulBackup)
	in.DeepCopyInto(out)
	return out
}

func (in *ConsulBackup) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type ConsulBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ConsulBackup `json:"items"`
}

func (in *ConsulBackupList) DeepCopyInto(out *ConsulBackupList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]ConsulBackup, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *ConsulBackupList) DeepCopy() *ConsulBackupList {
	if in == nil {
		return nil
	}
	out := new(ConsulBackupList)
	in.DeepCopyInto(out)
	return out
}

func (in *ConsulBackupList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type ConsulBackupSpec struct {
	IntervalInSeconds int                         `json:"intervalInSeconds"`
	MaxBackups        int                         `json:"maxBackups"`
	Service           corev1.LocalObjectReference `json:"service"`
	Storage           ConsulBackupStorageSpec     `json:"storage"`
}

func (in *ConsulBackupSpec) DeepCopyInto(out *ConsulBackupSpec) {
	*out = *in
	in.Service.DeepCopyInto(&out.Service)
	in.Storage.DeepCopyInto(&out.Storage)
}

func (in *ConsulBackupSpec) DeepCopy() *ConsulBackupSpec {
	if in == nil {
		return nil
	}
	out := new(ConsulBackupSpec)
	in.DeepCopyInto(out)
	return out
}

type ConsulBackupStatus struct {
	Succeeded         bool                        `json:"succeeded"`
	LastSucceededTime *metav1.Time                `json:"lastSucceededTime,omitempty"`
	History           []ConsulBackupStatusHistory `json:"history"`
}

func (in *ConsulBackupStatus) DeepCopyInto(out *ConsulBackupStatus) {
	*out = *in
	if in.LastSucceededTime != nil {
		in, out := &in.LastSucceededTime, &out.LastSucceededTime
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
	if in.History != nil {
		l := make([]ConsulBackupStatusHistory, len(in.History))
		for i := range in.History {
			in.History[i].DeepCopyInto(&l[i])
		}
		out.History = l
	}
}

func (in *ConsulBackupStatus) DeepCopy() *ConsulBackupStatus {
	if in == nil {
		return nil
	}
	out := new(ConsulBackupStatus)
	in.DeepCopyInto(out)
	return out
}

type ConsulBackupStorageSpec struct {
	Minio *BackupStorageMinIOSpec `json:"minio,omitempty"`
	Gcs   *BackupStorageGCSSpec   `json:"gcs,omitempty"`
}

func (in *ConsulBackupStorageSpec) DeepCopyInto(out *ConsulBackupStorageSpec) {
	*out = *in
	if in.Minio != nil {
		in, out := &in.Minio, &out.Minio
		*out = new(BackupStorageMinIOSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Gcs != nil {
		in, out := &in.Gcs, &out.Gcs
		*out = new(BackupStorageGCSSpec)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ConsulBackupStorageSpec) DeepCopy() *ConsulBackupStorageSpec {
	if in == nil {
		return nil
	}
	out := new(ConsulBackupStorageSpec)
	in.DeepCopyInto(out)
	return out
}

type ConsulBackupStatusHistory struct {
	Succeeded   bool         `json:"succeeded"`
	ExecuteTime *metav1.Time `json:"executeTime,omitempty"`
	Path        string       `json:"path"`
	Message     string       `json:"message"`
}

func (in *ConsulBackupStatusHistory) DeepCopyInto(out *ConsulBackupStatusHistory) {
	*out = *in
	if in.ExecuteTime != nil {
		in, out := &in.ExecuteTime, &out.ExecuteTime
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ConsulBackupStatusHistory) DeepCopy() *ConsulBackupStatusHistory {
	if in == nil {
		return nil
	}
	out := new(ConsulBackupStatusHistory)
	in.DeepCopyInto(out)
	return out
}

type BackupStorageMinIOSpec struct {
	Service    *ObjectReference `json:"service,omitempty"`
	Credential AWSCredential    `json:"credential"`
	Bucket     string           `json:"bucket"`
	Path       string           `json:"path"`
	Secure     bool             `json:"secure"`
}

func (in *BackupStorageMinIOSpec) DeepCopyInto(out *BackupStorageMinIOSpec) {
	*out = *in
	if in.Service != nil {
		in, out := &in.Service, &out.Service
		*out = new(ObjectReference)
		(*in).DeepCopyInto(*out)
	}
	in.Credential.DeepCopyInto(&out.Credential)
}

func (in *BackupStorageMinIOSpec) DeepCopy() *BackupStorageMinIOSpec {
	if in == nil {
		return nil
	}
	out := new(BackupStorageMinIOSpec)
	in.DeepCopyInto(out)
	return out
}

type BackupStorageGCSSpec struct {
	Bucket     string        `json:"bucket"`
	Path       string        `json:"path"`
	Credential GCPCredential `json:"credential"`
}

func (in *BackupStorageGCSSpec) DeepCopyInto(out *BackupStorageGCSSpec) {
	*out = *in
	in.Credential.DeepCopyInto(&out.Credential)
}

func (in *BackupStorageGCSSpec) DeepCopy() *BackupStorageGCSSpec {
	if in == nil {
		return nil
	}
	out := new(BackupStorageGCSSpec)
	in.DeepCopyInto(out)
	return out
}

type ObjectReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

func (in *ObjectReference) DeepCopyInto(out *ObjectReference) {
	*out = *in
}

func (in *ObjectReference) DeepCopy() *ObjectReference {
	if in == nil {
		return nil
	}
	out := new(ObjectReference)
	in.DeepCopyInto(out)
	return out
}

type AWSCredential struct {
	Accesskeyid     *corev1.SecretKeySelector `json:"accessKeyId,omitempty"`
	SecretAccessKey *corev1.SecretKeySelector `json:"secretAccessKey,omitempty"`
}

func (in *AWSCredential) DeepCopyInto(out *AWSCredential) {
	*out = *in
	if in.Accesskeyid != nil {
		in, out := &in.Accesskeyid, &out.Accesskeyid
		*out = new(corev1.SecretKeySelector)
		(*in).DeepCopyInto(*out)
	}
	if in.SecretAccessKey != nil {
		in, out := &in.SecretAccessKey, &out.SecretAccessKey
		*out = new(corev1.SecretKeySelector)
		(*in).DeepCopyInto(*out)
	}
}

func (in *AWSCredential) DeepCopy() *AWSCredential {
	if in == nil {
		return nil
	}
	out := new(AWSCredential)
	in.DeepCopyInto(out)
	return out
}

type GCPCredential struct {
	Serviceaccountjson *corev1.SecretKeySelector `json:"serviceAccountJson,omitempty"`
}

func (in *GCPCredential) DeepCopyInto(out *GCPCredential) {
	*out = *in
	if in.Serviceaccountjson != nil {
		in, out := &in.Serviceaccountjson, &out.Serviceaccountjson
		*out = new(corev1.SecretKeySelector)
		(*in).DeepCopyInto(*out)
	}
}

func (in *GCPCredential) DeepCopy() *GCPCredential {
	if in == nil {
		return nil
	}
	out := new(GCPCredential)
	in.DeepCopyInto(out)
	return out
}
