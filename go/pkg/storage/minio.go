package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.f110.dev/mono/go/pkg/k8s/portforward"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	pf "k8s.io/client-go/tools/portforward"
)

type MinIOOptions struct {
	Name            string
	Namespace       string
	Port            int
	Bucket          string
	AccessKey       string
	SecretAccessKey string

	// PodLister is an optional value.
	PodLister corev1listers.PodLister
	// ServiceLister is an optional value.
	ServiceLister corev1listers.ServiceLister
	// Transport is an optional value.
	Transport http.RoundTripper
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

	opt MinIOOptions
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
		opt:             opt,
	}
}

func (m *MinIO) Put(ctx context.Context, name string, data *bytes.Buffer) error {
	return m.PutReader(ctx, name, data, int64(data.Len()))
}

func (m *MinIO) PutReader(ctx context.Context, name string, r io.Reader, size int64) error {
	mc, forwarder, err := m.newMinIOClient(ctx)
	if forwarder != nil {
		defer forwarder.Close()
	}
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	_, err = mc.PutObject(ctx, m.bucket, name, r, size, minio.PutObjectOptions{})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (m *MinIO) List(ctx context.Context, prefix string) ([]string, error) {
	mc, forwarder, err := m.newMinIOClient(ctx)
	if forwarder != nil {
		defer forwarder.Close()
	}
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	if prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	listCh := mc.ListObjects(ctx, m.bucket, minio.ListObjectsOptions{Prefix: prefix})
	files := make([]string, 0)
	for obj := range listCh {
		if obj.Err != nil {
			return nil, xerrors.Errorf(": %w", obj.Err)
		}
		files = append(files, obj.Key)
	}

	return files, nil
}

func (m *MinIO) Get(ctx context.Context, name string) ([]byte, error) {
	mc, forwarder, err := m.newMinIOClient(ctx)
	if forwarder != nil {
		defer forwarder.Close()
	}
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	obj, err := mc.GetObject(ctx, m.bucket, name, minio.GetObjectOptions{})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	buf, err := ioutil.ReadAll(obj)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	return buf, nil
}

func (m *MinIO) Delete(ctx context.Context, name string) error {
	mc, forwarder, err := m.newMinIOClient(ctx)
	if forwarder != nil {
		defer forwarder.Close()
	}
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	err = mc.RemoveObject(ctx, m.bucket, name, minio.RemoveObjectOptions{})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (m *MinIO) newMinIOClient(ctx context.Context) (*minio.Client, *pf.PortForwarder, error) {
	endpoint, forwarder, err := m.getMinIOEndpoint(ctx, m.name, m.namespace, m.port)
	if err != nil {
		return nil, nil, xerrors.Errorf(": %w", err)
	}
	creds := credentials.NewStaticV4(m.accessKey, m.secretAccessKey, "")
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:     creds,
		Secure:    false,
		Transport: m.opt.Transport,
	})
	if err != nil {
		return nil, nil, xerrors.Errorf(": %w", err)
	}

	return mc, forwarder, nil
}

func (m *MinIO) getMinIOEndpoint(ctx context.Context, name, namespace string, port int) (string, *pf.PortForwarder, error) {
	var forwarder *pf.PortForwarder
	endpoint := fmt.Sprintf("%s-hl-svc.%s.svc:%d", name, namespace, port)
	if m.dev {
		var svc *corev1.Service
		if m.opt.ServiceLister != nil {
			if s, err := m.opt.ServiceLister.Services(namespace).Get(name); err != nil {
				return "", nil, xerrors.Errorf(": %w", err)
			} else {
				svc = s
			}
		} else {
			s, err := m.client.CoreV1().Services(namespace).Get(ctx, fmt.Sprintf("%s-hl-svc", name), metav1.GetOptions{})
			if err != nil {
				return "", nil, xerrors.Errorf(": %w", err)
			}
			svc = s
		}
		f, port, err := portforward.PortForward(ctx, svc, int(svc.Spec.Ports[0].Port), m.config, m.client, m.opt.PodLister)
		if err != nil {
			return "", nil, xerrors.Errorf(": %w", err)
		}
		forwarder = f
		endpoint = fmt.Sprintf("127.0.0.1:%d", port)
	}

	return endpoint, forwarder, nil
}
