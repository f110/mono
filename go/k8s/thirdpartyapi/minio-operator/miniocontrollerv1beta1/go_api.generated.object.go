package miniocontrollerv1beta1

import (
	"go.f110.dev/kubeproto/go/apis/appsv1"
	"go.f110.dev/kubeproto/go/apis/corev1"
	"go.f110.dev/kubeproto/go/apis/metav1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const GroupName = "miniocontroller.min.io"

var (
	GroupVersion       = metav1.GroupVersion{Group: GroupName, Version: "v1beta1"}
	SchemeBuilder      = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme        = SchemeBuilder.AddToScheme
	SchemaGroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1beta1"}
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemaGroupVersion,
		&MinIOInstance{},
		&MinIOInstanceList{},
		&Mirror{},
		&MirrorList{},
	)
	metav1.AddToGroupVersion(scheme, SchemaGroupVersion)
	return nil
}

type MinIOInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Scheduler         *MinIOInstanceScheduler `json:"scheduler,omitempty"`
	Spec              MinIOInstanceSpec       `json:"spec"`
	Status            MinIOInstanceStatus     `json:"status"`
}

func (in *MinIOInstance) DeepCopyInto(out *MinIOInstance) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Scheduler != nil {
		in, out := &in.Scheduler, &out.Scheduler
		*out = new(MinIOInstanceScheduler)
		(*in).DeepCopyInto(*out)
	}
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *MinIOInstance) DeepCopy() *MinIOInstance {
	if in == nil {
		return nil
	}
	out := new(MinIOInstance)
	in.DeepCopyInto(out)
	return out
}

func (in *MinIOInstance) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type MinIOInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []MinIOInstance `json:"items"`
}

func (in *MinIOInstanceList) DeepCopyInto(out *MinIOInstanceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]MinIOInstance, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *MinIOInstanceList) DeepCopy() *MinIOInstanceList {
	if in == nil {
		return nil
	}
	out := new(MinIOInstanceList)
	in.DeepCopyInto(out)
	return out
}

func (in *MinIOInstanceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type Mirror struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              MirrorSpec   `json:"spec"`
	Status            MirrorStatus `json:"status"`
}

func (in *Mirror) DeepCopyInto(out *Mirror) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *Mirror) DeepCopy() *Mirror {
	if in == nil {
		return nil
	}
	out := new(Mirror)
	in.DeepCopyInto(out)
	return out
}

func (in *Mirror) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type MirrorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Mirror `json:"items"`
}

func (in *MirrorList) DeepCopyInto(out *MirrorList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]Mirror, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *MirrorList) DeepCopy() *MirrorList {
	if in == nil {
		return nil
	}
	out := new(MirrorList)
	in.DeepCopyInto(out)
	return out
}

func (in *MirrorList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type MinIOInstanceScheduler struct {
	// SchedulerName defines the name of scheduler to be used to schedule MinIOInstance pods
	Name string `json:"name"`
}

func (in *MinIOInstanceScheduler) DeepCopyInto(out *MinIOInstanceScheduler) {
	*out = *in
}

func (in *MinIOInstanceScheduler) DeepCopy() *MinIOInstanceScheduler {
	if in == nil {
		return nil
	}
	out := new(MinIOInstanceScheduler)
	in.DeepCopyInto(out)
	return out
}

type MinIOInstanceSpec struct {
	// Image defines the MinIOInstance Docker image.
	Image string `json:"image,omitempty"`
	// Replicas defines the number of MinIO instances in a MinIOInstance resource
	Replicas int `json:"replicas,omitempty"`
	// Pod Management Policy for pod created by StatefulSet
	PodManagementPolicy appsv1.PodManagementPolicyType `json:"podManagementPolicy,omitempty"`
	// Metadata defines the object metadata passed to each pod that is a part of this MinIOInstance
	Metadata *metav1.ObjectMeta `json:"metadata,omitempty"`
	// If provided, use this secret as the credentials for MinIOInstance resource
	// Otherwise MinIO server creates dynamic credentials printed on MinIO server startup banner
	CredsSecret *corev1.LocalObjectReference `json:"credsSecret,omitempty"`
	// If provided, use these environment variables for MinIOInstance resource
	Env []corev1.EnvVar `json:"env"`
	// If provided, use these requests and limit for cpu/memory resource allocation
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// VolumeClaimTemplate allows a user to specify how volumes inside a MinIOInstance
	VolumeClaimTemplate *corev1.PersistentVolumeClaim `json:"volumeClaimTemplate,omitempty"`
	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// If specified, affinity will define the pod's scheduling constraints
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// ExternalCertSecret allows a user to specify custom CA certificate, and private key for group replication SSL.
	ExternalCertSecret *LocalCertificateReference `json:"externalCertSecret,omitempty"`
	// Mount path for MinIO volume (PV). Defaults to /export
	Mountpath string `json:"mountPath,omitempty"`
	// Subpath inside mount path. This is the directory where MinIO stores data. Default to "" (empty)
	Subpath string `json:"subPath,omitempty"`
	// Liveness Probe for container liveness. Container will be restarted if the probe fails.
	Liveness *corev1.Probe `json:"liveness,omitempty"`
	// Readiness Probe for container readiness. Container will be removed from service endpoints if the probe fails.
	Readiness *corev1.Probe `json:"readiness,omitempty"`
	// RequestAutoCert allows user to enable Kubernetes based TLS cert generation and signing as explained here:
	// https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/
	RequestAutoCert bool `json:"requestAutoCert,omitempty"`
	// CertConfig allows users to set entries like CommonName, Organization, etc for the certificate
	CertConfig *CertificateConfig `json:"certConfig,omitempty"`
	// Tolerations allows users to set entries like effect, key, operator, value.
	Tolerations []corev1.Toleration `json:"tolerations"`
}

func (in *MinIOInstanceSpec) DeepCopyInto(out *MinIOInstanceSpec) {
	*out = *in
	if in.Metadata != nil {
		in, out := &in.Metadata, &out.Metadata
		*out = new(metav1.ObjectMeta)
		(*in).DeepCopyInto(*out)
	}
	if in.CredsSecret != nil {
		in, out := &in.CredsSecret, &out.CredsSecret
		*out = new(corev1.LocalObjectReference)
		(*in).DeepCopyInto(*out)
	}
	if in.Env != nil {
		l := make([]corev1.EnvVar, len(in.Env))
		for i := range in.Env {
			in.Env[i].DeepCopyInto(&l[i])
		}
		out.Env = l
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(corev1.ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
	if in.VolumeClaimTemplate != nil {
		in, out := &in.VolumeClaimTemplate, &out.VolumeClaimTemplate
		*out = new(corev1.PersistentVolumeClaim)
		(*in).DeepCopyInto(*out)
	}
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(corev1.Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.ExternalCertSecret != nil {
		in, out := &in.ExternalCertSecret, &out.ExternalCertSecret
		*out = new(LocalCertificateReference)
		(*in).DeepCopyInto(*out)
	}
	if in.Liveness != nil {
		in, out := &in.Liveness, &out.Liveness
		*out = new(corev1.Probe)
		(*in).DeepCopyInto(*out)
	}
	if in.Readiness != nil {
		in, out := &in.Readiness, &out.Readiness
		*out = new(corev1.Probe)
		(*in).DeepCopyInto(*out)
	}
	if in.CertConfig != nil {
		in, out := &in.CertConfig, &out.CertConfig
		*out = new(CertificateConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Tolerations != nil {
		l := make([]corev1.Toleration, len(in.Tolerations))
		for i := range in.Tolerations {
			in.Tolerations[i].DeepCopyInto(&l[i])
		}
		out.Tolerations = l
	}
}

func (in *MinIOInstanceSpec) DeepCopy() *MinIOInstanceSpec {
	if in == nil {
		return nil
	}
	out := new(MinIOInstanceSpec)
	in.DeepCopyInto(out)
	return out
}

type MinIOInstanceStatus struct {
	AvailableReplicas int `json:"availableReplicas"`
}

func (in *MinIOInstanceStatus) DeepCopyInto(out *MinIOInstanceStatus) {
	*out = *in
}

func (in *MinIOInstanceStatus) DeepCopy() *MinIOInstanceStatus {
	if in == nil {
		return nil
	}
	out := new(MinIOInstanceStatus)
	in.DeepCopyInto(out)
	return out
}

type MirrorSpec struct {
	// Version defines the MinIO Client (mc) Docker image version.
	Version string `json:"version"`
	// SourceEndpoint is the endpoint of MinIO instance to backup.
	SourceEndpoint string `json:"srcEndpoint"`
	// SourceCredsSecret as the credentials for source MinIO instance.
	SourceCredsSecret *corev1.LocalObjectReference `json:"srcCredsSecret,omitempty"`
	// SourceBucket defines the bucket on source MinIO instance
	SourceBucket string `json:"srcBucket,omitempty"`
	// Region in which the source S3 compatible bucket is located.
	// uses "us-east-1" by default
	SourceRegion string `json:"srcRegion"`
	// Endpoint (hostname only or fully qualified URI) of S3 compatible
	// storage service.
	TargetEndpoint string `json:"targetEndpoint"`
	// CredentialsSecret is a reference to the Secret containing the
	// credentials authenticating with the S3 compatible storage service.
	TargetCredsSecret *corev1.LocalObjectReference `json:"targetCredsSecret,omitempty"`
	// Bucket in which to store the Backup.
	TargetBucket string `json:"targetBucket"`
	// Region in which the Target S3 compatible bucket is located.
	// uses "us-east-1" by default
	TargetRegion string `json:"targetRegion"`
}

func (in *MirrorSpec) DeepCopyInto(out *MirrorSpec) {
	*out = *in
	if in.SourceCredsSecret != nil {
		in, out := &in.SourceCredsSecret, &out.SourceCredsSecret
		*out = new(corev1.LocalObjectReference)
		(*in).DeepCopyInto(*out)
	}
	if in.TargetCredsSecret != nil {
		in, out := &in.TargetCredsSecret, &out.TargetCredsSecret
		*out = new(corev1.LocalObjectReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *MirrorSpec) DeepCopy() *MirrorSpec {
	if in == nil {
		return nil
	}
	out := new(MirrorSpec)
	in.DeepCopyInto(out)
	return out
}

type MirrorStatus struct {
	// Outcome holds the results of a Mirror operation.
	Outcome string `json:"outcome"`
	// TimeStarted is the time at which the backup was started.
	TimeStarted metav1.Time `json:"timeStarted"`
	// TimeCompleted is the time at which the backup completed.
	TimeCompleted metav1.Time `json:"timeCompleted"`
}

func (in *MirrorStatus) DeepCopyInto(out *MirrorStatus) {
	*out = *in
	in.TimeStarted.DeepCopyInto(&out.TimeStarted)
	in.TimeCompleted.DeepCopyInto(&out.TimeCompleted)
}

func (in *MirrorStatus) DeepCopy() *MirrorStatus {
	if in == nil {
		return nil
	}
	out := new(MirrorStatus)
	in.DeepCopyInto(out)
	return out
}

type LocalCertificateReference struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

func (in *LocalCertificateReference) DeepCopyInto(out *LocalCertificateReference) {
	*out = *in
}

func (in *LocalCertificateReference) DeepCopy() *LocalCertificateReference {
	if in == nil {
		return nil
	}
	out := new(LocalCertificateReference)
	in.DeepCopyInto(out)
	return out
}

type CertificateConfig struct {
	CommonName       string   `json:"commonName,omitempty"`
	OrganizationName []string `json:"organizationName"`
	DNSNames         []string `json:"dnsNames"`
}

func (in *CertificateConfig) DeepCopyInto(out *CertificateConfig) {
	*out = *in
	if in.OrganizationName != nil {
		t := make([]string, len(in.OrganizationName))
		copy(t, in.OrganizationName)
		out.OrganizationName = t
	}
	if in.DNSNames != nil {
		t := make([]string, len(in.DNSNames))
		copy(t, in.DNSNames)
		out.DNSNames = t
	}
}

func (in *CertificateConfig) DeepCopy() *CertificateConfig {
	if in == nil {
		return nil
	}
	out := new(CertificateConfig)
	in.DeepCopyInto(out)
	return out
}
