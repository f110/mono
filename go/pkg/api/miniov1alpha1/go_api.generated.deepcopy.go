package miniov1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

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
	//  If bucket_finalize_policy is an empty string, then it is the same as "keep".
	BucketFinalizePolicy BucketFinalizePolicy `json:"bucketFinalizePolicy"`
	// policy is the policy of the bucket. One of public, readOnly, private.
	//  If you don't want to give public access, set private or an empty value.
	//  If it is an empty value, The bucket will not have any policy.
	//  Currently, MinIOBucket can't use prefix based policy.
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

type MinIOUserSpec struct {
	// selector is a selector of MinIOInstance
	Selector metav1.LabelSelector `json:"selector"`
	// path is a path in vault
	Path string `json:"path"`
	// mount_path is a mount path of KV secrets engine.
	MountPath string `json:"mountPath"`
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
