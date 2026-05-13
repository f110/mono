package k8sfactory

import (
	"sort"

	"go.f110.dev/kubeproto/go/apis/appsv1"
	"go.f110.dev/kubeproto/go/apis/batchv1"
	"go.f110.dev/kubeproto/go/apis/corev1"
	"go.f110.dev/kubeproto/go/apis/metav1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
)

func PodFactory(base *corev1.Pod, traits ...Trait) *corev1.Pod {
	var p *corev1.Pod
	if base == nil {
		p = &corev1.Pod{}
	} else {
		p = base.DeepCopy()
	}

	setGVK(p, scheme.Scheme)

	for _, v := range traits {
		v(p)
	}

	return p
}

func Ready(v any) {
	p, ok := v.(*corev1.Pod)
	if !ok {
		return
	}
	p.Status.Phase = corev1.PodPhaseRunning
	containerStatus := make([]corev1.ContainerStatus, 0)
	for _, v := range p.Spec.Containers {
		containerStatus = append(containerStatus, corev1.ContainerStatus{
			Name:    v.Name,
			Ready:   true,
			Image:   v.Image,
			Started: true,
		})
	}
	p.Status.ContainerStatuses = containerStatus
	p.Status.Conditions = append(p.Status.Conditions, corev1.PodCondition{
		Type:               corev1.PodConditionTypeReady,
		Status:             corev1.ConditionStatusTrue,
		LastTransitionTime: new(metav1.Now()),
	})
}

// NotReady is the trait function for k8sfactory.
// The object is created but not ready.
func NotReady(v any) {
	p, ok := v.(*corev1.Pod)
	if !ok {
		return
	}

	p.Status.Phase = corev1.PodPhaseRunning
	p.Status.Conditions = append(p.Status.Conditions, corev1.PodCondition{
		Type:               corev1.PodConditionTypeReady,
		Status:             corev1.ConditionStatusFalse,
		LastTransitionTime: new(metav1.Now()),
	})
	containerStatus := make([]corev1.ContainerStatus, 0)
	for _, v := range p.Spec.Containers {
		containerStatus = append(containerStatus, corev1.ContainerStatus{
			Name:    v.Name,
			Image:   v.Image,
			Ready:   false,
			Started: true,
		})
	}
	p.Status.ContainerStatuses = containerStatus
}

func PodSucceeded(v any) {
	p, ok := v.(*corev1.Pod)
	if !ok {
		return
	}
	p.Status.Phase = corev1.PodPhaseSucceeded
}

func PodFailed(v any) {
	p, ok := v.(*corev1.Pod)
	if !ok {
		return
	}
	p.Status.Phase = corev1.PodPhaseFailed
}

func RestartPolicy(policy corev1.RestartPolicy) Trait {
	return func(object any) {
		p, ok := object.(*corev1.Pod)
		if !ok {
			return
		}
		if p.Spec == nil {
			p.Spec = &corev1.PodSpec{}
		}
		p.Spec.RestartPolicy = policy
	}
}

func Container(c *corev1.Container) Trait {
	return func(object any) {
		if c == nil {
			return
		}

		switch obj := object.(type) {
		case *corev1.Pod:
			if obj.Spec == nil {
				obj.Spec = &corev1.PodSpec{}
			}
			obj.Spec.Containers = append(obj.Spec.Containers, *c)
		}
	}
}

func InitContainer(c *corev1.Container) Trait {
	return func(object any) {
		if c == nil {
			return
		}

		switch obj := object.(type) {
		case *corev1.Pod:
			obj.Spec.InitContainers = append(obj.Spec.InitContainers, *c)
		case *batchv1.Job:
			obj.Spec.Template.Spec.InitContainers = append(obj.Spec.Template.Spec.InitContainers, *c)
		}
	}
}

func PreferredInterPodAntiAffinity(weight int, selector *metav1.LabelSelector, key string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Pod:
			if obj.Spec.Affinity == nil {
				obj.Spec.Affinity = &corev1.Affinity{}
			}
			if obj.Spec.Affinity.PodAntiAffinity == nil {
				obj.Spec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
			}

			obj.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
				obj.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
				corev1.WeightedPodAffinityTerm{
					Weight: weight,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: selector,
						TopologyKey:   key,
					},
				},
			)
		}
	}
}

func AntiNodeAffinity(key string, nodes []string) Trait {
	return func(object any) {
		if len(nodes) == 0 {
			return
		}

		switch obj := object.(type) {
		case *corev1.Pod:
			if obj.Spec.Affinity == nil {
				obj.Spec.Affinity = &corev1.Affinity{}
			}
			if obj.Spec.Affinity.NodeAffinity == nil {
				obj.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{}}
			}
			if obj.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
				obj.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
			}

			obj.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(
				obj.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
				corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{
					{Key: key, Operator: corev1.NodeSelectorOperatorNotIn, Values: nodes},
				}},
			)
		}
	}
}

func ServiceAccount(v string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Pod:
			obj.Spec.ServiceAccountName = v
		}
	}
}

func Hostname(v string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Pod:
			obj.Spec.Hostname = v
		}
	}
}

func Subdomain(v string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Pod:
			obj.Spec.Subdomain = v
		}
	}
}

func ContainerFactory(base *corev1.Container, traits ...Trait) *corev1.Container {
	var c *corev1.Container
	if base == nil {
		c = &corev1.Container{}
	} else {
		c = base.DeepCopy()
	}

	for _, v := range traits {
		v(c)
	}

	return c
}

func Image(image string, cmd []string) Trait {
	return func(object any) {
		c, ok := object.(*corev1.Container)
		if !ok {
			return
		}
		c.Image = image
		c.Command = cmd
	}
}

func Args(args ...string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Container:
			obj.Args = args
		}
	}
}

func WorkDir(dir string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Container:
			obj.WorkingDir = dir
		}
	}
}

func PullPolicy(p corev1.PullPolicy) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Container:
			obj.ImagePullPolicy = p
		}
	}
}

func EnvVar(k, v string) Trait {
	return func(object any) {
		if v == "" {
			return
		}
		c, ok := object.(*corev1.Container)
		if !ok {
			return
		}
		c.Env = append(c.Env, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}
}

func EnvFrom(name string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Container:
			obj.EnvFrom = append(obj.EnvFrom, corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: name,
					},
				},
			})
		}
	}
}

func EnvFromField(k, v string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Container:
			obj.Env = append(obj.Env, corev1.EnvVar{
				Name: k,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: v,
					},
				},
			})
		}
	}
}

func EnvFromSecret(k, name, secretKey string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Container:
			obj.Env = append(obj.Env, corev1.EnvVar{
				Name: k,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: name,
						},
						Key: secretKey,
					},
				},
			})
		}
	}
}

func LivenessProbe(p *corev1.Probe) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Container:
			obj.LivenessProbe = p
		}
	}
}

func ReadinessProbe(p *corev1.Probe) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Container:
			obj.ReadinessProbe = p
		}
	}
}

func ProbeFactory(base *corev1.Probe, traits ...Trait) *corev1.Probe {
	var p *corev1.Probe
	if base == nil {
		p = &corev1.Probe{}
	} else {
		p = base.DeepCopy()
	}

	for _, v := range traits {
		v(p)
	}

	return p
}

func InitialDelay(s int) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Probe:
			obj.InitialDelaySeconds = s
		}
	}
}

func Timeout(s int) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Probe:
			obj.TimeoutSeconds = s
		}
	}
}

func ProbeHandler(h corev1.ProbeHandler) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Probe:
			obj.ProbeHandler = h
		}
	}
}

func TCPProbe(port int) corev1.ProbeHandler {
	return corev1.ProbeHandler{
		TCPSocket: &corev1.TCPSocketAction{
			Port: intstr.FromInt32(int32(port)),
		},
	}
}

func HTTPProbe(port int, path string) corev1.ProbeHandler {
	return corev1.ProbeHandler{
		HTTPGet: &corev1.HTTPGetAction{
			Port: intstr.FromInt32(int32(port)),
			Path: path,
		},
	}
}

func ExecProbe(command ...string) corev1.ProbeHandler {
	return corev1.ProbeHandler{
		Exec: &corev1.ExecAction{Command: command},
	}
}

func Volume(vol *VolumeSource) Trait {
	return func(object any) {
		if vol == nil {
			return
		}

		switch obj := object.(type) {
		case *corev1.Container:
			obj.VolumeMounts = append(obj.VolumeMounts, vol.Mount)
		case *corev1.Pod:
			if obj.Spec == nil {
				obj.Spec = &corev1.PodSpec{}
			}
			obj.Spec.Volumes = append(obj.Spec.Volumes, vol.Source)
		}
	}
}

func SortVolume() Trait {
	var t Trait
	t = func(object any) {
		switch obj := object.(type) {
		case *corev1.PodSpec:
			sort.Slice(obj.Volumes, func(i, j int) bool {
				return obj.Volumes[i].Name < obj.Volumes[j].Name
			})
		case *corev1.Pod:
			t(obj.Spec)
		case *batchv1.Job:
			t(obj.Spec.Template.Spec)
		}
	}
	return t
}

func ResourceLimit(cpu, mem resource.Quantity) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Container:
			if obj.Resources == nil {
				obj.Resources = &corev1.ResourceRequirements{}
			}
			if obj.Resources.Limits == nil {
				obj.Resources.Limits = make(map[string]resource.Quantity)
			}
			obj.Resources.Limits[string(corev1.ResourceNameCpu)] = cpu
			obj.Resources.Limits[string(corev1.ResourceNameMemory)] = mem
		}
	}
}

func ResourceRequest(cpu, mem resource.Quantity) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Container:
			if obj.Resources == nil {
				obj.Resources = &corev1.ResourceRequirements{}
			}
			if obj.Resources.Requests == nil {
				obj.Resources.Requests = make(map[string]resource.Quantity)
			}
			obj.Resources.Requests[string(corev1.ResourceNameCpu)] = cpu
			obj.Resources.Requests[string(corev1.ResourceNameMemory)] = mem
		}
	}
}

func ServiceAccountFactory(base *corev1.ServiceAccount, traits ...Trait) *corev1.ServiceAccount {
	var sa *corev1.ServiceAccount
	if base == nil {
		sa = &corev1.ServiceAccount{}
	} else {
		sa = base.DeepCopy()
	}

	setGVK(sa, scheme.Scheme)

	for _, v := range traits {
		v(sa)
	}

	return sa
}

func ServiceFactory(base *corev1.Service, traits ...Trait) *corev1.Service {
	var s *corev1.Service
	if base == nil {
		s = &corev1.Service{}
	} else {
		s = base.DeepCopy()
	}

	setGVK(s, scheme.Scheme)

	for _, v := range traits {
		v(s)
	}

	return s
}

func ClusterIP(object any) {
	switch obj := object.(type) {
	case *corev1.Service:
		obj.Spec.Type = corev1.ServiceTypeClusterIP
	}
}

func LoadBalancer(object any) {
	switch obj := object.(type) {
	case *corev1.Service:
		obj.Spec.Type = corev1.ServiceTypeLoadBalancer
	}
}

func LoadBalancerIP(ip string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Service:
			obj.Spec.LoadBalancerIP = ip
		}
	}
}

func TrafficPolicyLocal(object any) {
	switch obj := object.(type) {
	case *corev1.Service:
		obj.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
	}
}

func IPNone(object any) {
	switch obj := object.(type) {
	case *corev1.Service:
		obj.Spec.ClusterIP = "None"
	}
}

func PublishNotReadyAddresses(object any) {
	switch obj := object.(type) {
	case *corev1.Service:
		obj.Spec.PublishNotReadyAddresses = true
	}
}

func Selector(v ...string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Service:
			sel := make(map[string]string)
			for i := 0; i < len(v); i += 2 {
				sel[v[i]] = v[i+1]
			}
			obj.Spec.Selector = sel
		}
	}
}

func Port(name string, protocol corev1.Protocol, port int) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Service:
			if obj.Spec == nil {
				obj.Spec = &corev1.ServiceSpec{}
			}
			obj.Spec.Ports = append(obj.Spec.Ports, corev1.ServicePort{
				Name:     name,
				Protocol: protocol,
				Port:     port,
			})
		case *corev1.Container:
			obj.Ports = append(obj.Ports, corev1.ContainerPort{
				Name:          name,
				Protocol:      protocol,
				ContainerPort: port,
			})
		}
	}
}

func TargetPort(name string, protocol corev1.Protocol, port int, targetPort intstr.IntOrString) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Service:
			obj.Spec.Ports = append(obj.Spec.Ports, corev1.ServicePort{
				Name:       name,
				Protocol:   protocol,
				Port:       port,
				TargetPort: &targetPort,
			})
		}
	}
}

func SecretFactory(base *corev1.Secret, traits ...Trait) *corev1.Secret {
	var s *corev1.Secret
	if base == nil {
		s = &corev1.Secret{}
	} else {
		s = base.DeepCopy()
	}

	setGVK(s, scheme.Scheme)

	for _, v := range traits {
		v(s)
	}

	return s
}

func Data(key string, value []byte) Trait {
	return func(v any) {
		switch obj := v.(type) {
		case *corev1.Secret:
			if obj.Data == nil {
				obj.Data = make(map[string][]byte)
			}
			obj.Data[key] = value
		case *corev1.ConfigMap:
			if obj.Data == nil {
				obj.Data = make(map[string]string)
			}
			obj.Data[key] = string(value)
		}
	}
}

func ConfigMapFactory(base *corev1.ConfigMap, traits ...Trait) *corev1.ConfigMap {
	var cm *corev1.ConfigMap
	if base == nil {
		cm = &corev1.ConfigMap{}
	} else {
		cm = base.DeepCopy()
	}

	setGVK(cm, scheme.Scheme)

	for _, v := range traits {
		v(cm)
	}

	return cm
}

func Requests(req map[string]resource.Quantity) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Container:
			if obj.Resources == nil {
				obj.Resources = &corev1.ResourceRequirements{}
			}
			obj.Resources.Requests = req
		case *corev1.PersistentVolumeClaim:
			if obj.Spec == nil {
				obj.Spec = &corev1.PersistentVolumeClaimSpec{}
			}
			if obj.Spec.Resources == nil {
				obj.Spec.Resources = &corev1.VolumeResourceRequirements{}
			}
			obj.Spec.Resources.Requests = req
		}
	}
}

func Limits(lim map[string]resource.Quantity) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Container:
			obj.Resources.Limits = lim
		}
	}
}

func EventFactory(base *corev1.Event, traits ...Trait) *corev1.Event {
	var e *corev1.Event
	if base == nil {
		e = &corev1.Event{}
	} else {
		e = base.DeepCopy()
	}

	setGVK(e, scheme.Scheme)

	for _, v := range traits {
		v(e)
	}

	return e
}

func Reason(v string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.Event:
			obj.Reason = v
		}
	}
}

func SecretKeySelector(secret *corev1.Secret, key string) *corev1.SecretKeySelector {
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: secret.Name,
		},
		Key: key,
	}
}

func LocalObjectReference(obj metav1.Object) corev1.LocalObjectReference {
	return corev1.LocalObjectReference{Name: obj.GetObjectMeta().GetName()}
}

func Pod(p *corev1.Pod) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *batchv1.Job:
			obj.Spec.Template = corev1.PodTemplateSpec{
				ObjectMeta: &p.ObjectMeta,
				Spec:       p.Spec,
			}
		case *appsv1.Deployment:
			obj.Spec.Template = corev1.PodTemplateSpec{
				ObjectMeta: &p.ObjectMeta,
				Spec:       p.Spec,
			}
		}
	}
}

func PersistentVolumeClaimFactory(base *corev1.PersistentVolumeClaim, traits ...Trait) *corev1.PersistentVolumeClaim {
	var e *corev1.PersistentVolumeClaim
	if base == nil {
		e = &corev1.PersistentVolumeClaim{}
	} else {
		e = base.DeepCopy()
	}

	setGVK(e, scheme.Scheme)

	for _, v := range traits {
		v(e)
	}

	return e
}

func StorageClassName(name string) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.PersistentVolumeClaim:
			obj.Spec.StorageClassName = name
		}
	}
}

func AccessModes(modes ...corev1.PersistentVolumeAccessMode) Trait {
	return func(object any) {
		switch obj := object.(type) {
		case *corev1.PersistentVolumeClaim:
			obj.Spec.AccessModes = modes
		}
	}
}
