package k8sfactory

import (
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/varptr"
)

type Trait func(object any)

func Factory(base any, traits ...Trait) any {
	if base == nil {
		return nil
	}

	switch v := base.(type) {
	case *corev1.Pod:
		p := PodFactory(v, traits...)
		return p
	case *corev1.Container:
		p := ContainerFactory(v, traits...)
		return p
	case *corev1.ServiceAccount:
		p := ServiceAccountFactory(v, traits...)
		return p
	case *corev1.Service:
		p := ServiceFactory(v, traits...)
		return p
	case *corev1.Secret:
		p := SecretFactory(v, traits...)
		return p
	case *corev1.ConfigMap:
		p := ConfigMapFactory(v, traits...)
		return p
	case *corev1.Event:
		p := EventFactory(v, traits...)
		return p
	case *appsv1.Deployment:
		p := DeploymentFactory(v, traits...)
		return p
	case *batchv1.Job:
		p := JobFactory(v, traits...)
		return p
	case *batchv1.CronJob:
		p := CronJobFactory(v, traits...)
		return p
	case *networkingv1.IngressClass:
		p := IngressClassFactory(v, traits...)
		return p
	case *networkingv1.Ingress:
		p := IngressFactory(v, traits...)
		return p
	case *networkingv1.IngressRule:
		p := IngressRuleFactory(v, traits...)
		return p
	case *networkingv1.HTTPIngressPath:
		p := IngressPathFactory(v, traits...)
		return p
	case *policyv1.PodDisruptionBudget:
		p := PodDisruptionBudgetFactory(v, traits...)
		return p
	case *rbacv1.Role:
		p := RoleFactory(v, traits...)
		return p
	case *rbacv1.RoleBinding:
		p := RoleBindingFactory(v, traits...)
		return p
	default:
		return nil
	}
}

func OnContainer(name string, t Trait) Trait {
	var fn Trait
	fn = func(object any) {
		switch v := object.(type) {
		case *corev1.PodSpec:
			for i := range v.InitContainers {
				con := &v.InitContainers[i]
				if con.Name == name {
					t(con)
					break
				}
			}
			for i := range v.Containers {
				con := &v.Containers[i]
				if con.Name == name {
					t(con)
					break
				}
			}
		case *batchv1.Job:
			fn(&v.Spec.Template.Spec)
		}
	}
	return fn
}

type VolumeSource struct {
	Mount  corev1.VolumeMount
	Source corev1.Volume
}

func (s *VolumeSource) PathJoin(elem ...string) string {
	return filepath.Join(append([]string{s.Mount.MountPath}, elem...)...)
}

func NewConfigMapVolumeSource(name, path, configMapName string) *VolumeSource {
	return &VolumeSource{
		Mount: corev1.VolumeMount{
			Name:      name,
			MountPath: path,
		},
		Source: corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
				},
			},
		},
	}
}

func NewSecretVolumeSource(name, path string, source *corev1.Secret, items ...corev1.KeyToPath) *VolumeSource {
	return &VolumeSource{
		Mount: corev1.VolumeMount{
			Name:      name,
			MountPath: path,
			ReadOnly:  true,
		},
		Source: corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: source.Name,
					Items:      items,
				},
			},
		},
	}
}

func NewEmptyDirVolumeSource(name, path string) *VolumeSource {
	return &VolumeSource{
		Mount: corev1.VolumeMount{
			Name:      name,
			MountPath: path,
		},
		Source: corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
}

func NewPersistentVolumeClaimVolumeSource(name, path, pvcName string) *VolumeSource {
	return &VolumeSource{
		Mount: corev1.VolumeMount{
			Name:      name,
			MountPath: path,
		},
		Source: corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
	}
}

func NewSecretStoreVolumeSource(name, path string) *VolumeSource {
	return &VolumeSource{
		Mount: corev1.VolumeMount{
			Name:      name,
			MountPath: path,
			ReadOnly:  true,
		},
		Source: corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver:   "secrets-store.csi.k8s.io",
					ReadOnly: varptr.Ptr(true),
					VolumeAttributes: map[string]string{
						"secretProviderClass": name,
					},
				},
			},
		},
	}
}

func setGVK(in runtime.Object, scheme *runtime.Scheme) {
	if in.GetObjectKind().GroupVersionKind().Kind == "" {
		gvks, unversioned, err := scheme.ObjectKinds(in)
		if err == nil && !unversioned && len(gvks) > 0 {
			in.GetObjectKind().SetGroupVersionKind(gvks[0])
		}
	}
}
