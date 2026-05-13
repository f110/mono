package k8sfactory

import (
	"fmt"
	"maps"
	"slices"
	"time"

	"go.f110.dev/kubeproto/go/apis/appsv1"
	"go.f110.dev/kubeproto/go/apis/batchv1"
	"go.f110.dev/kubeproto/go/apis/corev1"
	"go.f110.dev/kubeproto/go/apis/metav1"
	"go.f110.dev/kubeproto/go/apis/policyv1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"

	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/stringsutil"
)

func Name(v string) Trait {
	return func(object any) {
		m, ok := object.(interface {
			SetName(string)
		})
		if ok {
			m.SetName(v)
			return
		}

		switch obj := object.(type) {
		case *corev1.Container:
			obj.Name = v
		}
	}
}

func Namef(format string, a ...any) Trait {
	return Name(fmt.Sprintf(format, a...))
}

func DefaultNamespace(object any) {
	Namespace(metav1.NamespaceDefault)(object)
}

func Namespace(v string) Trait {
	return func(object any) {
		if m, ok := object.(metav1.Object); ok {
			m.GetObjectMeta().SetNamespace(v)
			return
		}
		if m, ok := object.(interface{ SetNamespace(string) }); ok {
			m.SetNamespace(v)
			return
		}
	}
}

func Generation(v int64) Trait {
	return func(object any) {
		m, ok := object.(metav1.Object)
		if ok {
			m.GetObjectMeta().SetGeneration(v)
		}
	}
}

func UID() Trait {
	return func(object any) {
		m, ok := object.(metav1.Object)
		if ok {
			m.GetObjectMeta().SetUID(uuid.NewUUID())
		}
	}
}

func Created(object any) {
	m, ok := object.(metav1.Object)
	if ok {
		m.GetObjectMeta().CreationTimestamp = new(metav1.Now())
		m.GetObjectMeta().SetUID(uuid.NewUUID())
		if m.GetObjectMeta().GetGenerateName() != "" && m.GetObjectMeta().GetName() == "" {
			m.GetObjectMeta().SetName(m.GetObjectMeta().GetGenerateName() + stringsutil.RandomString(5))
		}
	}
}

func CreatedAt(now time.Time) Trait {
	return func(object any) {
		Created(object)
		m, ok := object.(metav1.Object)
		if ok {
			m.GetObjectMeta().CreationTimestamp = new(metav1.NewTime(now))
		}
	}
}

func Delete(object any) {
	m, ok := object.(metav1.Object)
	if ok {
		m.GetObjectMeta().DeletionTimestamp = new(metav1.Now())
	}
}

func Annotation(k, v string) Trait {
	return func(object any) {
		m, ok := object.(metav1.Object)
		if ok {
			a := m.GetObjectMeta().GetAnnotations()
			if a == nil {
				a = make(map[string]string)
			}
			if v == "" {
				delete(a, k)
			} else {
				a[k] = v
			}
			m.GetObjectMeta().SetAnnotations(a)
			return
		}
	}
}

func Annotations(annotations map[string]string) Trait {
	return func(object any) {
		m, ok := object.(metav1.Object)
		if ok {
			a := m.GetObjectMeta().GetAnnotations()
			if a != nil {
				maps.Copy(a, annotations)
			} else {
				a = annotations
			}
			m.GetObjectMeta().SetAnnotations(a)
			return
		}
	}
}

func Label(v ...string) Trait {
	return func(object any) {
		m, ok := object.(metav1.Object)
		if ok {
			a := m.GetObjectMeta().GetLabels()
			if a == nil {
				a = make(map[string]string)
			}
			for i := 0; i < len(v); i += 2 {
				a[v[i]] = v[i+1]
			}
			m.GetObjectMeta().SetLabels(a)
			return
		}
	}
}

func Labels(label map[string]string) Trait {
	return func(object any) {
		if m, ok := object.(metav1.Object); ok {
			a := m.GetObjectMeta().GetLabels()
			if a != nil {
				maps.Copy(a, label)
			} else {
				a = label
			}
			m.GetObjectMeta().SetLabels(a)
			return
		}
		if m, ok := object.(interface {
			GetLabels() map[string]string
			SetLabels(map[string]string)
		}); ok {
			a := m.GetLabels()
			if a != nil {
				maps.Copy(a, label)
			} else {
				a = label
			}
			m.SetLabels(a)
			return
		}
	}
}

func ControlledBy(v runtime.Object, s *runtime.Scheme) Trait {
	return func(object any) {
		owner, ok := v.(metav1.Object)
		if !ok {
			return
		}

		m, ok := object.(metav1.Object)
		if ok {
			if metav1.IsControlledBy(m, owner) {
				return
			}

			gvks, _, err := s.ObjectKinds(v)
			if err != nil {
				return
			}
			if len(gvks) == 0 {
				return
			}
			objectMeta, ok := v.(metav1.Object)
			if !ok {
				return
			}

			ref := append(m.GetObjectMeta().OwnerReferences, metav1.NewControllerRef(*objectMeta.GetObjectMeta(), gvks[0]))
			m.GetObjectMeta().OwnerReferences = ref
		}
	}
}

func ClearOwnerReference(object any) {
	objMeta, ok := object.(metav1.Object)
	if !ok {
		return
	}
	objMeta.GetObjectMeta().OwnerReferences = make([]metav1.OwnerReference, 0)
}

func MatchLabel(v map[string]string) metav1.LabelSelector {
	return metav1.LabelSelector{
		MatchLabels: v,
	}
}

func MatchExpression(v ...metav1.LabelSelectorRequirement) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchExpressions: v,
	}
}

func MatchLabelSelector(label map[string]string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Service:
			if obj.Spec == nil {
				obj.Spec = &corev1.ServiceSpec{}
			}
			obj.Spec.Selector = label
		case *appsv1.Deployment:
			if obj.Spec == nil {
				obj.Spec = &appsv1.DeploymentSpec{}
			}
			obj.Spec.Selector = &metav1.LabelSelector{MatchLabels: label}
		case *policyv1.PodDisruptionBudget:
			if obj.Spec == nil {
				obj.Spec = &policyv1.PodDisruptionBudgetSpec{}
			}
			obj.Spec.Selector = &metav1.LabelSelector{MatchLabels: label}
		case *batchv1.Job:
			if obj.Spec == nil {
				obj.Spec = &batchv1.JobSpec{}
			}
			obj.Spec.Selector = &metav1.LabelSelector{MatchLabels: label}
		}
	}
}

func Finalizer(v string) Trait {
	return func(object any) {
		m, ok := object.(metav1.Object)
		if ok {
			found := slices.Contains(m.GetObjectMeta().GetFinalizers(), v)
			if found {
				return
			}

			m.GetObjectMeta().SetFinalizers(append(m.GetObjectMeta().GetFinalizers(), v))
		}
	}
}

func RemoveFinalizer(v string) Trait {
	return func(object any) {
		m, ok := object.(metav1.Object)
		if ok {
			m.GetObjectMeta().SetFinalizers(enumerable.Delete(m.GetObjectMeta().GetFinalizers(), v))
		}
	}
}
