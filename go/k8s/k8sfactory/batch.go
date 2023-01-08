package k8sfactory

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
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
		case *batchv1beta1.CronJob:
			obj.Spec.Schedule = v
		case *batchv1.CronJob:
			obj.Spec.Schedule = v
		}
	}
}

func Job(j *batchv1.Job) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *batchv1beta1.CronJob:
			obj.Spec.JobTemplate = batchv1beta1.JobTemplateSpec{
				ObjectMeta: j.ObjectMeta,
				Spec:       j.Spec,
			}
		case *batchv1.CronJob:
			obj.Spec.JobTemplate = batchv1.JobTemplateSpec{
				ObjectMeta: j.ObjectMeta,
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

func Pod(p *corev1.Pod) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *batchv1.Job:
			obj.Spec.Template = corev1.PodTemplateSpec{
				ObjectMeta: p.ObjectMeta,
				Spec:       p.Spec,
			}
		case *appsv1.Deployment:
			obj.Spec.Template = corev1.PodTemplateSpec{
				ObjectMeta: p.ObjectMeta,
				Spec:       p.Spec,
			}
		}
	}
}

func BackoffLimit(limit int32) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *batchv1.Job:
			obj.Spec.BackoffLimit = &limit
		}
	}
}
