package k8sfactory

import (
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/varptr"
)

type Trait func(object any)

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
