package storage

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/minio/minio-go/v6"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"go.f110.dev/mono/lib/logger"
)

type MinIOOptions struct {
	Name            string
	Namespace       string
	Port            int
	Bucket          string
	AccessKey       string
	SecretAccessKey string
}

func NewMinIOOptions(name, namespace string, port int, bucket, accessKey, secretAccessKey string) MinIOOptions {
	return MinIOOptions{
		Name:            name,
		Namespace:       namespace,
		Port:            port,
		Bucket:          bucket,
		AccessKey:       accessKey,
		SecretAccessKey: secretAccessKey,
	}
}

type MinIO struct {
	client kubernetes.Interface
	config *rest.Config
	dev    bool

	name            string
	namespace       string
	port            int
	bucket          string
	accessKey       string
	secretAccessKey string
}

func NewMinIOStorage(client kubernetes.Interface, config *rest.Config, opt MinIOOptions, dev bool) *MinIO {
	return &MinIO{
		client:          client,
		config:          config,
		name:            opt.Name,
		namespace:       opt.Namespace,
		port:            opt.Port,
		bucket:          opt.Bucket,
		accessKey:       opt.AccessKey,
		secretAccessKey: opt.SecretAccessKey,
		dev:             dev,
	}
}

func (m *MinIO) Put(ctx context.Context, name string, data []byte) error {
	mc, forwarder, err := m.newMinIOClient()
	if forwarder != nil {
		defer forwarder.Close()
	}
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	_, err = mc.PutObjectWithContext(ctx, m.bucket, name, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (m *MinIO) Get(ctx context.Context, name string) ([]byte, error) {
	mc, forwarder, err := m.newMinIOClient()
	if forwarder != nil {
		defer forwarder.Close()
	}
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	obj, err := mc.GetObjectWithContext(ctx, m.bucket, name, minio.GetObjectOptions{})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	buf, err := ioutil.ReadAll(obj)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	return buf, nil
}

func (m *MinIO) newMinIOClient() (*minio.Client, *portforward.PortForwarder, error) {
	endpoint, forwarder, err := m.getMinIOEndpoint(m.name, m.namespace, m.port)
	if err != nil {
		return nil, nil, xerrors.Errorf(": %w", err)
	}
	mc, err := minio.New(endpoint, m.accessKey, m.secretAccessKey, false)
	if err != nil {
		return nil, nil, xerrors.Errorf(": %w", err)
	}

	return mc, forwarder, nil
}

func (m *MinIO) getMinIOEndpoint(name, namespace string, port int) (string, *portforward.PortForwarder, error) {
	var forwarder *portforward.PortForwarder
	endpoint := fmt.Sprintf("%s-hl-svc.%s.svc:%d", name, namespace, port)
	if m.dev {
		svc, err := m.client.CoreV1().Services(namespace).Get(fmt.Sprintf("%s-hl-svc", name), metav1.GetOptions{})
		if err != nil {
			return "", nil, xerrors.Errorf(": %w", err)
		}
		forwarder, err = m.portForward(svc, int(svc.Spec.Ports[0].Port))
		if err != nil {
			return "", nil, err
		}

		ports, err := forwarder.GetPorts()
		if err != nil {
			return "", nil, err
		}
		endpoint = fmt.Sprintf("127.0.0.1:%d", ports[0].Local)
	}

	return endpoint, forwarder, nil
}

func (m *MinIO) portForward(svc *corev1.Service, port int) (*portforward.PortForwarder, error) {
	selector := labels.SelectorFromSet(svc.Spec.Selector)
	podList, err := m.client.CoreV1().Pods(svc.Namespace).List(metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	var pod *corev1.Pod
	for i, v := range podList.Items {
		if v.Status.Phase == corev1.PodRunning {
			pod = &podList.Items[i]
			break
		}
	}
	if pod == nil {
		return nil, xerrors.New("all pods are not running yet")
	}

	req := m.client.CoreV1().RESTClient().Post().Resource("pods").Namespace(svc.Namespace).Name(pod.Name).SubResource("portforward")
	transport, upgrader, err := spdy.RoundTripperFor(m.config)
	if err != nil {
		return nil, err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req.URL())

	readyCh := make(chan struct{})
	pf, err := portforward.New(dialer, []string{fmt.Sprintf(":%d", port)}, context.Background().Done(), readyCh, nil, nil)
	if err != nil {
		return nil, err
	}
	go func() {
		err := pf.ForwardPorts()
		if err != nil {
			logger.Log.Error("Failed get ports", zap.Error(err))
		}
	}()

	select {
	case <-readyCh:
	case <-time.After(5 * time.Second):
		return nil, xerrors.New("timed out")
	}

	return pf, nil
}
