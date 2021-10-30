package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	pf "k8s.io/client-go/tools/portforward"

	"go.f110.dev/mono/go/pkg/k8s/portforward"
)

type MinIOOptions struct {
	Name            string
	Namespace       string
	Endpoint        string
	Port            int
	AccessKey       string
	SecretAccessKey string

	Dev bool

	// PodLister is an optional value.
	PodLister corev1listers.PodLister
	// ServiceLister is an optional value.
	ServiceLister corev1listers.ServiceLister
	// Transport is an optional value.
	Transport http.RoundTripper

	client *minio.Client

	k8sClient     kubernetes.Interface
	restConfig    *rest.Config
	portForwarder *pf.PortForwarder
}

func (m *MinIOOptions) Client(ctx context.Context) (*minio.Client, error) {
	if m.client != nil {
		return m.client, nil
	}

	if m.k8sClient != nil {
		c, forwarder, err := m.newMinIOClientViaService(ctx)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		m.client = c
		m.portForwarder = forwarder
	} else if m.Endpoint != "" {
		creds := credentials.NewStaticV4(m.AccessKey, m.SecretAccessKey, "")
		c, err := minio.New(m.Endpoint, &minio.Options{
			Creds:     creds,
			Secure:    false,
			Transport: m.Transport,
		})
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		m.client = c
	}

	return m.client, nil
}

func (m *MinIOOptions) newMinIOClientViaService(ctx context.Context) (*minio.Client, *pf.PortForwarder, error) {
	endpoint, forwarder, err := m.getMinIOEndpoint(ctx, m.Name, m.Namespace, m.Port)
	if err != nil {
		return nil, nil, xerrors.Errorf(": %w", err)
	}
	creds := credentials.NewStaticV4(m.AccessKey, m.SecretAccessKey, "")
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:     creds,
		Secure:    false,
		Transport: m.Transport,
	})
	if err != nil {
		return nil, nil, xerrors.Errorf(": %w", err)
	}

	return mc, forwarder, nil
}

func (m *MinIOOptions) getMinIOEndpoint(ctx context.Context, name, namespace string, port int) (string, *pf.PortForwarder, error) {
	var forwarder *pf.PortForwarder
	endpoint := fmt.Sprintf("%s-hl-svc.%s.svc:%d", name, namespace, port)
	if m.Dev {
		var svc *corev1.Service
		if m.ServiceLister != nil {
			if s, err := m.ServiceLister.Services(namespace).Get(name); err != nil {
				return "", nil, xerrors.Errorf(": %w", err)
			} else {
				svc = s
			}
		} else {
			s, err := m.k8sClient.CoreV1().Services(namespace).Get(ctx, fmt.Sprintf("%s-hl-svc", name), metav1.GetOptions{})
			if err != nil {
				return "", nil, xerrors.Errorf(": %w", err)
			}
			svc = s
		}
		f, port, err := portforward.PortForward(ctx, svc, int(svc.Spec.Ports[0].Port), m.restConfig, m.k8sClient, m.PodLister)
		if err != nil {
			return "", nil, xerrors.Errorf(": %w", err)
		}
		forwarder = f
		endpoint = fmt.Sprintf("127.0.0.1:%d", port)
	}

	return endpoint, forwarder, nil
}

func (m *MinIOOptions) Close() {
	if m.portForwarder != nil {
		m.portForwarder.Close()
	}
}
func NewMinIOOptionsViaService(
	client kubernetes.Interface,
	config *rest.Config,
	name, namespace string,
	port int,
	accessKey, secretAccessKey string,
	dev bool,
) MinIOOptions {
	return MinIOOptions{
		Name:            name,
		Namespace:       namespace,
		Port:            port,
		AccessKey:       accessKey,
		SecretAccessKey: secretAccessKey,
		Dev:             dev,
		k8sClient:       client,
		restConfig:      config,
	}
}

func NewMinIOOptionsViaEndpoint(endpoint, accessKey, secretAccessKey string) MinIOOptions {
	return MinIOOptions{
		Endpoint:        endpoint,
		AccessKey:       accessKey,
		SecretAccessKey: secretAccessKey,
	}
}

type MinIO struct {
	bucket string
	opt    MinIOOptions
}

var _ storageInterface = &MinIO{}

func NewMinIOStorage(bucket string, opt MinIOOptions) *MinIO {
	return &MinIO{
		bucket: bucket,
		opt:    opt,
	}
}

func (m *MinIO) Put(ctx context.Context, name string, data []byte) error {
	return m.PutReader(ctx, name, bytes.NewReader(data))
}

func (m *MinIO) PutReader(ctx context.Context, name string, r io.Reader) error {
	mc, err := m.opt.Client(ctx)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	_, err = mc.PutObject(ctx, m.bucket, name, r, -1, minio.PutObjectOptions{})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (m *MinIO) List(ctx context.Context, prefix string) ([]*Object, error) {
	files, err := m.ListRecursive(ctx, prefix, true)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	var objs []*Object
	for _, v := range files {
		objs = append(objs, &Object{
			Name:         v.Key,
			LastModified: v.LastModified,
			Size:         v.Size,
		})
	}
	return objs, nil
}

func (m *MinIO) ListRecursive(ctx context.Context, prefix string, recursive bool) ([]minio.ObjectInfo, error) {
	mc, err := m.opt.Client(ctx)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	if prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	listCh := mc.ListObjects(ctx, m.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: recursive})
	files := make([]minio.ObjectInfo, 0)
	for obj := range listCh {
		if obj.Err != nil {
			return nil, xerrors.Errorf(": %w", obj.Err)
		}
		files = append(files, obj)
	}

	return files, nil
}

func (m *MinIO) Get(ctx context.Context, name string) (io.Reader, error) {
	mc, err := m.opt.Client(ctx)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	obj, err := mc.GetObject(ctx, m.bucket, name, minio.GetObjectOptions{})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return obj, nil
}

func (m *MinIO) Delete(ctx context.Context, name string) error {
	mc, err := m.opt.Client(ctx)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	err = mc.RemoveObject(ctx, m.bucket, name, minio.RemoveObjectOptions{})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (m *MinIO) Close() {
	m.opt.Close()
}
