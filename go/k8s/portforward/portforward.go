package portforward

import (
	"context"

	"go.f110.dev/kubeproto/go/apis/corev1"
	"go.f110.dev/kubeproto/go/apis/metav1"
	"go.f110.dev/kubeproto/go/k8sclient"
	"go.f110.dev/xerrors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/portforward"
)

func PortForward(
	ctx context.Context,
	service *corev1.Service,
	port int,
	client *k8sclient.Set,
	podLister *k8sclient.CoreV1PodLister,
) (*portforward.PortForwarder, uint16, error) {
	selector := labels.SelectorFromSet(service.Spec.Selector)
	var pods []*corev1.Pod
	if podLister != nil {
		if p, err := podLister.List(service.Namespace, selector); err != nil {
			return nil, 0, xerrors.WithStack(err)
		} else {
			pods = p
		}
	} else {
		selector := labels.SelectorFromSet(service.Spec.Selector)
		p, err := client.CoreV1.ListPod(ctx, service.Namespace, metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			return nil, 0, xerrors.WithStack(err)
		}
		pp := make([]*corev1.Pod, len(p.Items))
		for i := range p.Items {
			pp[i] = &p.Items[i]
		}
		pods = pp
	}

	var targetPod *corev1.Pod
	for _, v := range pods {
		if v.Status.Phase == corev1.PodPhaseRunning {
			targetPod = v
			break
		}
	}
	if targetPod == nil {
		return nil, 0, xerrors.New("all pods are not running")
	}

	pf, localPort, err := client.CoreV1.PortForward(ctx, targetPod, port)
	if err != nil {
		return nil, 0, xerrors.WithStack(err)
	}
	return pf, localPort, nil
}
