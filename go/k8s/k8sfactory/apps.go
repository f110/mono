package k8sfactory

import (
	"go.f110.dev/kubeproto/go/apis/appsv1"
	"k8s.io/client-go/kubernetes/scheme"
)

func DeploymentFactory(base *appsv1.Deployment, traits ...Trait) *appsv1.Deployment {
	var d *appsv1.Deployment
	if base == nil {
		d = &appsv1.Deployment{}
	} else {
		d = base.DeepCopy()
	}

	setGVK(d, scheme.Scheme)

	for _, v := range traits {
		v(d)
	}

	return d
}

func Replicas(v int) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *appsv1.Deployment:
			obj.Spec.Replicas = v
		}
	}
}
