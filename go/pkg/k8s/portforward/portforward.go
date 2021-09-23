package portforward

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

func PortForward(
	ctx context.Context,
	service *corev1.Service,
	port int,
	config *rest.Config,
	client kubernetes.Interface,
	podLister corev1listers.PodLister,
) (*portforward.PortForwarder, uint16, error) {
	selector := labels.SelectorFromSet(service.Spec.Selector)
	var pods []*corev1.Pod
	if podLister != nil {
		if p, err := podLister.List(selector); err != nil {
			return nil, 0, xerrors.Errorf(": %w", err)
		} else {
			pods = p
		}
	} else {
		selector := labels.SelectorFromSet(service.Spec.Selector)
		p, err := client.CoreV1().Pods(service.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			return nil, 0, xerrors.Errorf(": %w", err)
		}
		pp := make([]*corev1.Pod, len(p.Items))
		for i := range p.Items {
			pp[i] = &p.Items[i]
		}
		pods = pp
	}

	var targetPod *corev1.Pod
	for _, v := range pods {
		if v.Status.Phase == corev1.PodRunning {
			targetPod = v
			break
		}
	}
	if targetPod == nil {
		return nil, 0, xerrors.New("all pods are not running")
	}

	req := client.CoreV1().RESTClient().Post().Resource("pods").Namespace(targetPod.Namespace).Name(targetPod.Name).SubResource("portforward")
	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return nil, 0, xerrors.Errorf(": %w", err)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req.URL())

	readyCh := make(chan struct{})
	pf, err := portforward.New(dialer, []string{fmt.Sprintf(":%d", port)}, context.Background().Done(), readyCh, nil, nil)
	if err != nil {
		return nil, 0, xerrors.Errorf(": %w", err)
	}
	errCh := make(chan error)
	go func() {
		err := pf.ForwardPorts()
		if err != nil {
			errCh <- err
		}
	}()

	select {
	case <-readyCh:
	case err := <-errCh:
		return nil, 0, xerrors.Errorf(": %w", err)
	case <-time.After(5 * time.Second):
		return nil, 0, errors.New("timed out")
	}

	ports, err := pf.GetPorts()
	if err != nil {
		return nil, 0, xerrors.Errorf(": %w", err)
	}

	return pf, ports[0].Local, nil
}
