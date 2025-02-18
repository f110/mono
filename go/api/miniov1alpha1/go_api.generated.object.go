package miniov1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const GroupName = "minio.f110.dev"

var (
	GroupVersion       = metav1.GroupVersion{Group: GroupName, Version: "v1alpha1"}
	SchemeBuilder      = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme        = SchemeBuilder.AddToScheme
	SchemaGroupVersion = schema.GroupVersion{Group: "minio.f110.dev", Version: "v1alpha1"}
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemaGroupVersion,
		&MinIOBucket{},
		&MinIOBucketList{},
		&MinIOCluster{},
		&MinIOClusterList{},
		&MinIOUser{},
		&MinIOUserList{},
	)
	metav1.AddToGroupVersion(scheme, SchemaGroupVersion)
	return nil
}

type BucketFinalizePolicy string

const (
	BucketFinalizePolicyDelete BucketFinalizePolicy = "Delete"
	BucketFinalizePolicyKeep   BucketFinalizePolicy = "Keep"
)

type BucketPolicy string

const (
	BucketPolicyPublic   BucketPolicy = "Public"
	BucketPolicyReadOnly BucketPolicy = "ReadOnly"
	BucketPolicyPrivate  BucketPolicy = "Private"
)

type ClusterPhase string

const (
	ClusterPhaseCreating ClusterPhase = "Creating"
	ClusterPhaseRunning  ClusterPhase = "Running"
)

type MinIOBucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              MinIOBucketSpec   `json:"spec"`
	Status            MinIOBucketStatus `json:"status"`
}

func (in *MinIOBucket) DeepCopyInto(out *MinIOBucket) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *MinIOBucket) DeepCopy() *MinIOBucket {
	if in == nil {
		return nil
	}
	out := new(MinIOBucket)
	in.DeepCopyInto(out)
	return out
}

func (in *MinIOBucket) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type MinIOBucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []MinIOBucket `json:"items"`
}

func (in *MinIOBucketList) DeepCopyInto(out *MinIOBucketList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]MinIOBucket, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *MinIOBucketList) DeepCopy() *MinIOBucketList {
	if in == nil {
		return nil
	}
	out := new(MinIOBucketList)
	in.DeepCopyInto(out)
	return out
}

func (in *MinIOBucketList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type MinIOCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              MinIOClusterSpec   `json:"spec"`
	Status            MinIOClusterStatus `json:"status"`
}

func (in *MinIOCluster) DeepCopyInto(out *MinIOCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *MinIOCluster) DeepCopy() *MinIOCluster {
	if in == nil {
		return nil
	}
	out := new(MinIOCluster)
	in.DeepCopyInto(out)
	return out
}

func (in *MinIOCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type MinIOClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []MinIOCluster `json:"items"`
}

func (in *MinIOClusterList) DeepCopyInto(out *MinIOClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]MinIOCluster, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *MinIOClusterList) DeepCopy() *MinIOClusterList {
	if in == nil {
		return nil
	}
	out := new(MinIOClusterList)
	in.DeepCopyInto(out)
	return out
}

func (in *MinIOClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type MinIOUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              MinIOUserSpec   `json:"spec"`
	Status            MinIOUserStatus `json:"status"`
}

func (in *MinIOUser) DeepCopyInto(out *MinIOUser) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *MinIOUser) DeepCopy() *MinIOUser {
	if in == nil {
		return nil
	}
	out := new(MinIOUser)
	in.DeepCopyInto(out)
	return out
}

func (in *MinIOUser) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type MinIOUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []MinIOUser `json:"items"`
}

func (in *MinIOUserList) DeepCopyInto(out *MinIOUserList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]MinIOUser, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *MinIOUserList) DeepCopy() *MinIOUserList {
	if in == nil {
		return nil
	}
	out := new(MinIOUserList)
	in.DeepCopyInto(out)
	return out
}

func (in *MinIOUserList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type MinIOBucketSpec struct {
	// selector is a selector of MinIOInstance.
	Selector metav1.LabelSelector `json:"selector"`
	// bucket_finalize_policy is a policy when deleted CR Object.
	//
	//	If bucket_finalize_policy is an empty string, then it is the same as "keep".
	BucketFinalizePolicy BucketFinalizePolicy `json:"bucketFinalizePolicy"`
	// policy is the policy of the bucket. One of public, readOnly, private.
	//
	//	If you don't want to give public access, set private or an empty value.
	//	If it is an empty value, The bucket will not have any policy.
	//	Currently, MinIOBucket can't use prefix based policy.
	Policy BucketPolicy `json:"policy"`
	// create_index_file is a flag that creates index.html on top of bucket.
	CreateIndexFile bool `json:"createIndexFile"`
}

func (in *MinIOBucketSpec) DeepCopyInto(out *MinIOBucketSpec) {
	*out = *in
	in.Selector.DeepCopyInto(&out.Selector)
}

func (in *MinIOBucketSpec) DeepCopy() *MinIOBucketSpec {
	if in == nil {
		return nil
	}
	out := new(MinIOBucketSpec)
	in.DeepCopyInto(out)
	return out
}

type MinIOBucketStatus struct {
	Ready bool `json:"ready"`
}

func (in *MinIOBucketStatus) DeepCopyInto(out *MinIOBucketStatus) {
	*out = *in
}

func (in *MinIOBucketStatus) DeepCopy() *MinIOBucketStatus {
	if in == nil {
		return nil
	}
	out := new(MinIOBucketStatus)
	in.DeepCopyInto(out)
	return out
}

type MinIOClusterSpec struct {
	StorageClassName string `json:"storageClassName,omitempty"`
	Image            string `json:"image,omitempty"`
	// total_size is the size of the cluster in Gigabytes.
	TotalSize        int                  `json:"totalSize"`
	Nodes            int                  `json:"nodes"`
	Buckets          []MinIOClusterBucket `json:"buckets"`
	IdentityProvider *IdentityProvider    `json:"identityProvider,omitempty"`
	ExternalUrl      string               `json:"externalUrl,omitempty"`
}

func (in *MinIOClusterSpec) DeepCopyInto(out *MinIOClusterSpec) {
	*out = *in
	if in.Buckets != nil {
		l := make([]MinIOClusterBucket, len(in.Buckets))
		for i := range in.Buckets {
			in.Buckets[i].DeepCopyInto(&l[i])
		}
		out.Buckets = l
	}
	if in.IdentityProvider != nil {
		in, out := &in.IdentityProvider, &out.IdentityProvider
		*out = new(IdentityProvider)
		(*in).DeepCopyInto(*out)
	}
}

func (in *MinIOClusterSpec) DeepCopy() *MinIOClusterSpec {
	if in == nil {
		return nil
	}
	out := new(MinIOClusterSpec)
	in.DeepCopyInto(out)
	return out
}

type MinIOClusterStatus struct {
	Phase ClusterPhase `json:"phase"`
	Ready bool         `json:"ready"`
}

func (in *MinIOClusterStatus) DeepCopyInto(out *MinIOClusterStatus) {
	*out = *in
}

func (in *MinIOClusterStatus) DeepCopy() *MinIOClusterStatus {
	if in == nil {
		return nil
	}
	out := new(MinIOClusterStatus)
	in.DeepCopyInto(out)
	return out
}

type MinIOUserSpec struct {
	// selector is a selector of MinIOInstance
	Selector metav1.LabelSelector `json:"selector"`
	// path is a path in vault
	Path string `json:"path"`
	// mount_path is a mount path of KV secrets engine.
	MountPath string `json:"mountPath"`
	Policy    string `json:"policy"`
}

func (in *MinIOUserSpec) DeepCopyInto(out *MinIOUserSpec) {
	*out = *in
	in.Selector.DeepCopyInto(&out.Selector)
}

func (in *MinIOUserSpec) DeepCopy() *MinIOUserSpec {
	if in == nil {
		return nil
	}
	out := new(MinIOUserSpec)
	in.DeepCopyInto(out)
	return out
}

type MinIOUserStatus struct {
	Ready     bool   `json:"ready"`
	AccessKey string `json:"accessKey,omitempty"`
	Vault     bool   `json:"vault,omitempty"`
}

func (in *MinIOUserStatus) DeepCopyInto(out *MinIOUserStatus) {
	*out = *in
}

func (in *MinIOUserStatus) DeepCopy() *MinIOUserStatus {
	if in == nil {
		return nil
	}
	out := new(MinIOUserStatus)
	in.DeepCopyInto(out)
	return out
}

type MinIOClusterBucket struct {
	Name string `json:"name"`
	// policy is the policy of the bucket. One of public, readOnly, private.
	//
	//	If you don't want to give public access, set private or an empty value.
	//	If it is an empty value, The bucket will not have any policy.
	//	Currently, MinIOBucket can't use prefix based policy.
	Policy BucketPolicy `json:"policy,omitempty"`
	// create_index_file is a flag that creates index.html on top of bucket.
	CreateIndexFile bool `json:"createIndexFile,omitempty"`
}

func (in *MinIOClusterBucket) DeepCopyInto(out *MinIOClusterBucket) {
	*out = *in
}

func (in *MinIOClusterBucket) DeepCopy() *MinIOClusterBucket {
	if in == nil {
		return nil
	}
	out := new(MinIOClusterBucket)
	in.DeepCopyInto(out)
	return out
}

type IdentityProvider struct {
	DiscoveryUrl string         `json:"discoveryUrl"`
	ClientId     string         `json:"clientId"`
	ClientSecret SecretSelector `json:"clientSecret"`
	Scopes       []string       `json:"scopes"`
	Comment      string         `json:"comment,omitempty"`
}

func (in *IdentityProvider) DeepCopyInto(out *IdentityProvider) {
	*out = *in
	in.ClientSecret.DeepCopyInto(&out.ClientSecret)
	if in.Scopes != nil {
		t := make([]string, len(in.Scopes))
		copy(t, in.Scopes)
		out.Scopes = t
	}
}

func (in *IdentityProvider) DeepCopy() *IdentityProvider {
	if in == nil {
		return nil
	}
	out := new(IdentityProvider)
	in.DeepCopyInto(out)
	return out
}

type SecretSelector struct {
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`
	Vault  *VaultSecretSelector      `json:"vault,omitempty"`
}

func (in *SecretSelector) DeepCopyInto(out *SecretSelector) {
	*out = *in
	if in.Secret != nil {
		in, out := &in.Secret, &out.Secret
		*out = new(corev1.SecretKeySelector)
		(*in).DeepCopyInto(*out)
	}
	if in.Vault != nil {
		in, out := &in.Vault, &out.Vault
		*out = new(VaultSecretSelector)
		(*in).DeepCopyInto(*out)
	}
}

func (in *SecretSelector) DeepCopy() *SecretSelector {
	if in == nil {
		return nil
	}
	out := new(SecretSelector)
	in.DeepCopyInto(out)
	return out
}

type VaultSecretSelector struct {
	MountPath string `json:"mountPath"`
	Path      string `json:"path"`
	Key       string `json:"key"`
}

func (in *VaultSecretSelector) DeepCopyInto(out *VaultSecretSelector) {
	*out = *in
}

func (in *VaultSecretSelector) DeepCopy() *VaultSecretSelector {
	if in == nil {
		return nil
	}
	out := new(VaultSecretSelector)
	in.DeepCopyInto(out)
	return out
}
