package k8sfactory

import (
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
)

func PodDisruptionBudgetFactory(base *policyv1.PodDisruptionBudget, traits ...Trait) *policyv1.PodDisruptionBudget {
	var p *policyv1.PodDisruptionBudget
	if base == nil {
		p = &policyv1.PodDisruptionBudget{}
	} else {
		p = base.DeepCopy()
	}

	setGVK(p, scheme.Scheme)

	for _, v := range traits {
		v(p)
	}

	return p
}

func MinAvailable(v int) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *policyv1.PodDisruptionBudget:
			m := intstr.FromInt(v)
			obj.Spec.MinAvailable = &m
		}
	}
}
