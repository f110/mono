package k8sfactory

import (
	miniocontrollerv1beta1 "github.com/minio/minio-operator/pkg/apis/miniocontroller/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func MinIOInstanceFactory(base *miniocontrollerv1beta1.MinIOInstance, traits ...Trait) *miniocontrollerv1beta1.MinIOInstance {
	var s *miniocontrollerv1beta1.MinIOInstance
	if base == nil {
		s = &miniocontrollerv1beta1.MinIOInstance{}
	} else {
		s = base.DeepCopy()
	}

	sch := runtime.NewScheme()
	_ = miniocontrollerv1beta1.AddToScheme(sch)
	setGVK(s, sch)

	for _, v := range traits {
		v(s)
	}

	return s
}

func MinIOCredential(ref corev1.LocalObjectReference) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *miniocontrollerv1beta1.MinIOInstance:
			obj.Spec.CredsSecret = &ref
		}
	}
}
