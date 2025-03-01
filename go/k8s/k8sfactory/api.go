package k8sfactory

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	"go.f110.dev/mono/go/api/consulv1alpha1"
	"go.f110.dev/mono/go/api/grafanav1alpha1"
	"go.f110.dev/mono/go/api/harborv1alpha1"
	"go.f110.dev/mono/go/api/miniov1alpha1"
	"go.f110.dev/mono/go/k8s/client"
)

func ServiceReference(v corev1.LocalObjectReference) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *consulv1alpha1.ConsulBackup:
			obj.Spec.Service = v
		case *grafanav1alpha1.Grafana:
			obj.Spec.Service = &v
		}
	}
}

func ConsulBackupFactory(base *consulv1alpha1.ConsulBackup, traits ...Trait) *consulv1alpha1.ConsulBackup {
	var s *consulv1alpha1.ConsulBackup
	if base == nil {
		s = &consulv1alpha1.ConsulBackup{}
	} else {
		s = base.DeepCopy()
	}

	setGVK(s, client.Scheme)

	for _, v := range traits {
		v(s)
	}

	return s
}

func BackupSucceeded(time time.Time) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *consulv1alpha1.ConsulBackup:
			obj.Status.Succeeded = true
			obj.Status.LastSucceededTime = &metav1.Time{Time: time}
		}
	}
}

func MaxBackup(v int) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *consulv1alpha1.ConsulBackup:
			obj.Spec.MaxBackups = v
		}
	}
}

func BackupInterval(seconds int) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *consulv1alpha1.ConsulBackup:
			obj.Spec.IntervalInSeconds = seconds
		}
	}
}

func BackupStorage(v any) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *consulv1alpha1.ConsulBackup:
			switch s := v.(type) {
			case *consulv1alpha1.BackupStorageMinIOSpec:
				obj.Spec.Storage.MinIO = s
			case *consulv1alpha1.BackupStorageGCSSpec:
				obj.Spec.Storage.GCS = s
			}
		}
	}
}

func BackupMinIOStorageFactory(base *consulv1alpha1.BackupStorageMinIOSpec, traits ...Trait) *consulv1alpha1.BackupStorageMinIOSpec {
	var s *consulv1alpha1.BackupStorageMinIOSpec
	if base == nil {
		s = &consulv1alpha1.BackupStorageMinIOSpec{}
	} else {
		s = base.DeepCopy()
	}

	for _, v := range traits {
		v(s)
	}

	return s
}

func Bucket(bucket string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *consulv1alpha1.BackupStorageMinIOSpec:
			obj.Bucket = bucket
		}
	}
}

func StoragePath(path string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *consulv1alpha1.BackupStorageMinIOSpec:
			obj.Path = path
		}
	}
}

func AWSCredential(creds *consulv1alpha1.AWSCredential) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *consulv1alpha1.BackupStorageMinIOSpec:
			obj.Credential = *creds
		}
	}
}

func BackupService(objRef *consulv1alpha1.ObjectReference) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *consulv1alpha1.BackupStorageMinIOSpec:
			obj.Service = objRef
		}
	}
}

func AWSCredentialFactory(base *consulv1alpha1.AWSCredential, traits ...Trait) *consulv1alpha1.AWSCredential {
	var s *consulv1alpha1.AWSCredential
	if base == nil {
		s = &consulv1alpha1.AWSCredential{}
	} else {
		s = base.DeepCopy()
	}

	for _, v := range traits {
		v(s)
	}

	return s
}

func AccessKey(ref *corev1.SecretKeySelector) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *consulv1alpha1.AWSCredential:
			obj.AccessKeyID = ref
		}
	}
}

func SecretAccessKey(ref *corev1.SecretKeySelector) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *consulv1alpha1.AWSCredential:
			obj.SecretAccessKey = ref
		}
	}
}

func ObjectReference(obj metav1.Object) *consulv1alpha1.ObjectReference {
	return &consulv1alpha1.ObjectReference{Name: obj.GetName(), Namespace: obj.GetNamespace()}
}

func MinIOBucketFactory(base *miniov1alpha1.MinIOBucket, traits ...Trait) *miniov1alpha1.MinIOBucket {
	var s *miniov1alpha1.MinIOBucket
	if base == nil {
		s = &miniov1alpha1.MinIOBucket{}
	} else {
		s = base.DeepCopy()
	}

	setGVK(s, client.Scheme)

	for _, v := range traits {
		v(s)
	}

	return s
}

func MinIOSelector(sel metav1.LabelSelector) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *miniov1alpha1.MinIOBucket:
			obj.Spec.Selector = sel
		case *miniov1alpha1.MinIOUser:
			obj.Spec.Selector = &sel
		}
	}
}

func EnableCreatingIndexFile(object any) {
	switch obj := object.(type) {
	case *miniov1alpha1.MinIOBucket:
		obj.Spec.CreateIndexFile = true
	}
}

func DisableCreatingIndexFile(object any) {
	switch obj := object.(type) {
	case *miniov1alpha1.MinIOBucket:
		obj.Spec.CreateIndexFile = false
	}
}

func MinIOUserFactory(base *miniov1alpha1.MinIOUser, traits ...Trait) *miniov1alpha1.MinIOUser {
	var s *miniov1alpha1.MinIOUser
	if base == nil {
		s = &miniov1alpha1.MinIOUser{}
	} else {
		s = base.DeepCopy()
	}

	setGVK(s, client.Scheme)

	for _, v := range traits {
		v(s)
	}

	return s
}

func VaultPath(mountPath, path string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *miniov1alpha1.MinIOUser:
			obj.Spec.MountPath = mountPath
			obj.Spec.Path = path
		}
	}
}

func GrafanaFactory(base *grafanav1alpha1.Grafana, traits ...Trait) *grafanav1alpha1.Grafana {
	var s *grafanav1alpha1.Grafana
	if base == nil {
		s = &grafanav1alpha1.Grafana{}
	} else {
		s = base.DeepCopy()
	}

	setGVK(s, client.Scheme)

	for _, v := range traits {
		v(s)
	}

	return s
}

func UserSelector(v metav1.LabelSelector) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *grafanav1alpha1.Grafana:
			obj.Spec.UserSelector = v
		}
	}
}

func AdminPasswordSecret(v *corev1.SecretKeySelector) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *grafanav1alpha1.Grafana:
			obj.Spec.AdminPasswordSecret = v
		}
	}
}

func GrafanaUserFactory(base *grafanav1alpha1.GrafanaUser, traits ...Trait) *grafanav1alpha1.GrafanaUser {
	var s *grafanav1alpha1.GrafanaUser
	if base == nil {
		s = &grafanav1alpha1.GrafanaUser{}
	} else {
		s = base.DeepCopy()
	}

	setGVK(s, client.Scheme)

	for _, v := range traits {
		v(s)
	}

	return s
}

func UserEmail(v string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *grafanav1alpha1.GrafanaUser:
			obj.Spec.Email = v
		}
	}
}

func HarborProjectFactory(base *harborv1alpha1.HarborProject, traits ...Trait) *harborv1alpha1.HarborProject {
	var s *harborv1alpha1.HarborProject
	if base == nil {
		s = &harborv1alpha1.HarborProject{}
	} else {
		s = base.DeepCopy()
	}

	setGVK(s, client.Scheme)

	for _, v := range traits {
		v(s)
	}

	return s
}

func ReadyProject(id int) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *harborv1alpha1.HarborProject:
			obj.Status.Ready = true
			obj.Status.ProjectId = id
		}
	}
}

func HarborRobotAccountFactory(base *harborv1alpha1.HarborRobotAccount, traits ...Trait) *harborv1alpha1.HarborRobotAccount {
	var s *harborv1alpha1.HarborRobotAccount
	if base == nil {
		s = &harborv1alpha1.HarborRobotAccount{}
	} else {
		s = base.DeepCopy()
	}

	setGVK(s, client.Scheme)

	for _, v := range traits {
		v(s)
	}

	return s
}

func ProjectReference(p *harborv1alpha1.HarborProject) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *harborv1alpha1.HarborRobotAccount:
			obj.Spec.ProjectName = p.Name
			obj.Spec.ProjectNamespace = p.Namespace
		}
	}
}

func DockerSecret(s *corev1.Secret) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *harborv1alpha1.HarborRobotAccount:
			obj.Spec.SecretName = s.Name
		}
	}
}

func NewSecretProviderClassFactory(base *secretsstorev1.SecretProviderClass, traits ...Trait) *secretsstorev1.SecretProviderClass {
	var s *secretsstorev1.SecretProviderClass
	if base == nil {
		s = &secretsstorev1.SecretProviderClass{}
	} else {
		s = base.DeepCopy()
	}

	setGVK(s, client.Scheme)

	for _, v := range traits {
		v(s)
	}

	return s
}

func Parameters(params map[string]string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *secretsstorev1.SecretProviderClass:
			obj.Spec.Parameters = params
		}
	}
}

func Provider(name string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *secretsstorev1.SecretProviderClass:
			obj.Spec.Provider = secretsstorev1.Provider(name)
		}
	}
}

func MinIOClusterFactory(base *miniov1alpha1.MinIOCluster, traits ...Trait) *miniov1alpha1.MinIOCluster {
	var s *miniov1alpha1.MinIOCluster
	if base == nil {
		s = &miniov1alpha1.MinIOCluster{}
	} else {
		s = base.DeepCopy()
	}

	setGVK(s, client.Scheme)

	for _, v := range traits {
		v(s)
	}

	return s
}

func Nodes(n int) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *miniov1alpha1.MinIOCluster:
			obj.Spec.Nodes = n
		}
	}
}

func TotalSize(n int) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *miniov1alpha1.MinIOCluster:
			obj.Spec.TotalSize = n
		}
	}
}
