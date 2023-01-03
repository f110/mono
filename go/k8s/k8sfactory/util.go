package k8sfactory

import (
	"math/rand"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
)

type Trait func(object interface{})

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

var charset = []byte("abcdefghijklmnopqrstuvwxyz0123456789")

func randomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}

	return string(b)
}
