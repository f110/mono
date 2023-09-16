package metav1

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

var Unversioned = schema.GroupVersion{Group: "", Version: "v1"}

const (
	NamespaceDefault = "default"
	NamespaceAll     = ""
	NamespaceNone    = ""
	NamespaceSystem  = "kube-system"
	NamespacePublic  = "kube-public"
)

func (in *TypeMeta) GetObjectKind() schema.ObjectKind { return in }

func (in *TypeMeta) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	in.APIVersion, in.Kind = gvk.ToAPIVersionAndKind()
}

func (in *TypeMeta) GroupVersionKind() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(in.APIVersion, in.Kind)
}

type InternalEvent watch.Event

func (e *InternalEvent) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }

func (e *InternalEvent) DeepCopyObject() runtime.Object {
	if c := e.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

func (e *InternalEvent) DeepCopy() *InternalEvent {
	if e == nil {
		return nil
	}
	out := new(InternalEvent)
	e.DeepCopyInto(out)
	return out
}

func (e *InternalEvent) DeepCopyInto(out *InternalEvent) {
	*out = *e
	if e.Object != nil {
		out.Object = e.Object.DeepCopyObject()
	}
	return
}

func (in *WatchEvent) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }

func (in *WatchEvent) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func AddToGroupVersion(scheme *runtime.Scheme, groupVersion schema.GroupVersion) {
	scheme.AddKnownTypeWithName(groupVersion.WithKind("WatchEventKind"), &WatchEvent{})
	scheme.AddKnownTypeWithName(
		schema.GroupVersion{Group: groupVersion.Group, Version: runtime.APIVersionInternal}.WithKind("WatchEventKind"),
		&InternalEvent{},
	)
	// Supports legacy code paths, most callers should use metav1.ParameterCodec for now
	scheme.AddKnownTypes(groupVersion, &ListOptions{},
		&GetOptions{},
		&DeleteOptions{},
		&CreateOptions{},
		&UpdateOptions{},
		&PatchOptions{},
	)
	// Register Unversioned types under their own special group
	scheme.AddUnversionedTypes(Unversioned,
		&Status{},
		&APIVersions{},
		&APIGroupList{},
		&APIGroup{},
		&APIResourceList{},
	)
}

func Now() Time {
	return NewTime(time.Now())
}

func NewTime(t time.Time) Time {
	return Time{
		Seconds: t.Unix(),
		Nanos:   t.Nanosecond(),
	}
}

func (in *Time) IsZero() bool {
	if in == nil {
		return true
	}
	t := time.Unix(in.Seconds, int64(in.Nanos))
	return t.IsZero()
}

func (in *Time) Before(u *Time) bool {
	if in != nil && u != nil {
		return in.Time().Before(u.Time())
	}
	return false
}

func (in *Time) After(u *Time) bool {
	if in != nil && u != nil {
		return in.Time().After(u.Time())
	}
	return false
}

func (in *Time) Equal(u *Time) bool {
	if in == nil && u == nil {
		return true
	}
	if in != nil && u != nil {
		return in.Time().Equal(u.Time())
	}
	return false
}

func (in *Time) Time() time.Time {
	return time.Unix(in.Seconds, int64(in.Nanos))
}

func (in *Time) Unix() int64 {
	return in.Time().Unix()
}

type Object interface {
	GetNamespace() string
	SetNamespace(namespace string)
	GetName() string
	SetName(name string)
	GetGenerateName() string
	SetGenerateName(name string)
	GetUID() string
	SetUID(uid string)
	GetResourceVersion() string
	SetResourceVersion(version string)
	GetGeneration() int64
	SetGeneration(generation int64)
	GetSelfLink() string
	SetSelfLink(selfLink string)
	GetCreationTimestamp() Time
	SetCreationTimestamp(timestamp Time)
	GetDeletionTimestamp() *Time
	SetDeletionTimestamp(timestamp *Time)
	GetDeletionGracePeriodSeconds() *int64
	SetDeletionGracePeriodSeconds(*int64)
	GetLabels() map[string]string
	SetLabels(labels map[string]string)
	GetAnnotations() map[string]string
	SetAnnotations(annotations map[string]string)
	GetFinalizers() []string
	SetFinalizers(finalizers []string)
	GetOwnerReferences() []OwnerReference
	SetOwnerReferences([]OwnerReference)
	GetManagedFields() []ManagedFieldsEntry
	SetManagedFields(managedFields []ManagedFieldsEntry)
}

func NewControllerRef(owner Object, gvk schema.GroupVersionKind) *OwnerReference {
	return &OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               owner.GetName(),
		UID:                owner.GetUID(),
		BlockOwnerDeletion: true,
		Controller:         true,
	}
}

func IsControlledBy(obj Object, owner Object) bool {
	for _, v := range obj.GetOwnerReferences() {
		if v.Controller && v.UID == owner.GetUID() {
			return true
		}
	}
	return false
}

func HasAnnotation(obj ObjectMeta, key string) bool {
	_, ok := obj.Annotations[key]
	return ok
}

func SetMetadataAnnotation(obj *ObjectMeta, key string, value string) {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
}

// Functions for Object
var _ Object = (*ObjectMeta)(nil)

func (in *ObjectMeta) GetNamespace() string                { return in.Namespace }
func (in *ObjectMeta) SetNamespace(namespace string)       { in.Namespace = namespace }
func (in *ObjectMeta) GetName() string                     { return in.Name }
func (in *ObjectMeta) SetName(name string)                 { in.Name = name }
func (in *ObjectMeta) GetGenerateName() string             { return in.GenerateName }
func (in *ObjectMeta) SetGenerateName(generateName string) { in.GenerateName = generateName }
func (in *ObjectMeta) GetUID() string                      { return in.UID }
func (in *ObjectMeta) SetUID(uid string)                   { in.UID = uid }
func (in *ObjectMeta) GetResourceVersion() string          { return in.ResourceVersion }
func (in *ObjectMeta) SetResourceVersion(version string)   { in.ResourceVersion = version }
func (in *ObjectMeta) GetGeneration() int64                { return in.Generation }
func (in *ObjectMeta) SetGeneration(generation int64)      { in.Generation = generation }
func (in *ObjectMeta) GetSelfLink() string                 { return in.SelfLink }
func (in *ObjectMeta) SetSelfLink(selfLink string)         { in.SelfLink = selfLink }
func (in *ObjectMeta) GetCreationTimestamp() Time          { return *in.CreationTimestamp }
func (in *ObjectMeta) SetCreationTimestamp(creationTimestamp Time) {
	in.CreationTimestamp = &creationTimestamp
}
func (in *ObjectMeta) GetDeletionTimestamp() *Time { return in.DeletionTimestamp }
func (in *ObjectMeta) SetDeletionTimestamp(deletionTimestamp *Time) {
	in.DeletionTimestamp = deletionTimestamp
}
func (in *ObjectMeta) GetDeletionGracePeriodSeconds() *int64 {
	return &in.DeletionGracePeriodSeconds
}
func (in *ObjectMeta) SetDeletionGracePeriodSeconds(deletionGracePeriodSeconds *int64) {
	in.DeletionGracePeriodSeconds = *deletionGracePeriodSeconds
}
func (in *ObjectMeta) GetLabels() map[string]string                 { return in.Labels }
func (in *ObjectMeta) SetLabels(labels map[string]string)           { in.Labels = labels }
func (in *ObjectMeta) GetAnnotations() map[string]string            { return in.Annotations }
func (in *ObjectMeta) SetAnnotations(annotations map[string]string) { in.Annotations = annotations }
func (in *ObjectMeta) GetFinalizers() []string                      { return in.Finalizers }
func (in *ObjectMeta) SetFinalizers(finalizers []string)            { in.Finalizers = finalizers }
func (in *ObjectMeta) GetOwnerReferences() []OwnerReference         { return in.OwnerReferences }
func (in *ObjectMeta) SetOwnerReferences(references []OwnerReference) {
	in.OwnerReferences = references
}
func (in *ObjectMeta) GetManagedFields() []ManagedFieldsEntry { return in.ManagedFields }
func (in *ObjectMeta) SetManagedFields(managedFields []ManagedFieldsEntry) {
	in.ManagedFields = managedFields
}
