package k8sfactory

import (
	"go.f110.dev/kubeproto/go/apis/batchv1"
	"k8s.io/client-go/kubernetes/scheme"
)

func CronJobFactory(base *batchv1.CronJob, traits ...Trait) *batchv1.CronJob {
	var cj *batchv1.CronJob
	if base == nil {
		cj = &batchv1.CronJob{}
	} else {
		cj = base.DeepCopy()
	}

	setGVK(cj, scheme.Scheme)

	for _, v := range traits {
		v(cj)
	}

	return cj
}

func Schedule(v string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *batchv1.CronJob:
			obj.Spec.Schedule = v
		}
	}
}

func Job(j *batchv1.Job) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *batchv1.CronJob:
			obj.Spec.JobTemplate = batchv1.JobTemplateSpec{
				ObjectMeta: &j.ObjectMeta,
				Spec:       j.Spec,
			}
		}
	}
}

func JobFactory(base *batchv1.Job, traits ...Trait) *batchv1.Job {
	var j *batchv1.Job
	if base == nil {
		j = &batchv1.Job{}
	} else {
		j = base.DeepCopy()
	}

	setGVK(j, scheme.Scheme)

	for _, v := range traits {
		v(j)
	}

	return j
}

func PodFailurePolicy(v batchv1.PodFailurePolicyRule) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *batchv1.Job:
			if obj.Spec.PodFailurePolicy == nil {
				obj.Spec.PodFailurePolicy = &batchv1.PodFailurePolicy{}
			}
			obj.Spec.PodFailurePolicy.Rules = append(obj.Spec.PodFailurePolicy.Rules, v)
		}
	}
}

func BackoffLimit(limit int) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *batchv1.Job:
			if obj.Spec == nil {
				obj.Spec = &batchv1.JobSpec{}
			}
			obj.Spec.BackoffLimit = limit
		}
	}
}

func JobComplete(object any) {
	switch obj := object.(type) {
	case *batchv1.Job:
		if obj.Status == nil {
			obj.Status = &batchv1.JobStatus{}
		}
		obj.Status.Conditions = append(obj.Status.Conditions, batchv1.JobCondition{
			Type: batchv1.JobConditionTypeComplete,
		})
	}
}
