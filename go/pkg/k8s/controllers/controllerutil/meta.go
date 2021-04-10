package controllerutil

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func SetOwner(obj runtime.Object, owner runtime.Object, scheme *runtime.Scheme) {
	objMeta, ok := obj.(metav1.Object)
	if !ok {
		return
	}
	ownerMeta, ok := owner.(metav1.Object)
	if !ok {
		return
	}
	if metav1.IsControlledBy(objMeta, ownerMeta) {
		return
	}

	gvks, _, err := scheme.ObjectKinds(owner)
	if err != nil {
		return
	}
	if len(gvks) != 1 {
		return
	}
	ref := metav1.NewControllerRef(ownerMeta, gvks[0])
	objMeta.SetOwnerReferences(append(objMeta.GetOwnerReferences(), *ref))
}
