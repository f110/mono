package client

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"time"

	"github.com/minio/minio-operator/pkg/apis/miniocontroller/v1beta1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"go.f110.dev/mono/go/pkg/api/consulv1alpha1"
	"go.f110.dev/mono/go/pkg/api/grafanav1alpha1"
	"go.f110.dev/mono/go/pkg/api/harborv1alpha1"
	"go.f110.dev/mono/go/pkg/api/miniov1alpha1"
)

var (
	Scheme         = runtime.NewScheme()
	ParameterCodec = runtime.NewParameterCodec(Scheme)
	Codecs         = serializer.NewCodecFactory(Scheme)
	AddToScheme    = localSchemeBuilder.AddToScheme
)

var localSchemeBuilder = runtime.SchemeBuilder{
	consulv1alpha1.AddToScheme,
	grafanav1alpha1.AddToScheme,
	harborv1alpha1.AddToScheme,
	miniov1alpha1.AddToScheme,
	v1beta1.AddToScheme,
}

func init() {
	for _, v := range []func(*runtime.Scheme) error{
		consulv1alpha1.AddToScheme,
		grafanav1alpha1.AddToScheme,
		harborv1alpha1.AddToScheme,
		miniov1alpha1.AddToScheme,
		v1beta1.AddToScheme,
	} {
		if err := v(Scheme); err != nil {
			panic(err)
		}
	}
}

type Backend interface {
	Get(ctx context.Context, resourceName, kindName, namespace, name string, opts metav1.GetOptions, result runtime.Object) (runtime.Object, error)
	List(ctx context.Context, resourceName, kindName, namespace string, opts metav1.ListOptions, result runtime.Object) (runtime.Object, error)
	Create(ctx context.Context, resourceName, kindName string, obj runtime.Object, opts metav1.CreateOptions, result runtime.Object) (runtime.Object, error)
	Update(ctx context.Context, resourceName, kindName string, obj runtime.Object, opts metav1.UpdateOptions, result runtime.Object) (runtime.Object, error)
	UpdateStatus(ctx context.Context, resourceName, kindName string, obj runtime.Object, opts metav1.UpdateOptions, result runtime.Object) (runtime.Object, error)
	Delete(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string, opts metav1.DeleteOptions) error
	Watch(ctx context.Context, gvr schema.GroupVersionResource, namespace string, opts metav1.ListOptions) (watch.Interface, error)
}
type Set struct {
	ConsulV1alpha1         *ConsulV1alpha1
	GrafanaV1alpha1        *GrafanaV1alpha1
	HarborV1alpha1         *HarborV1alpha1
	MinioV1alpha1          *MinioV1alpha1
	MiniocontrollerV1beta1 *MiniocontrollerV1beta1
}

func NewSet(cfg *rest.Config) (*Set, error) {
	s := &Set{}
	{
		conf := *cfg
		conf.GroupVersion = &consulv1alpha1.SchemaGroupVersion
		conf.APIPath = "/apis"
		conf.NegotiatedSerializer = Codecs.WithoutConversion()
		c, err := rest.RESTClientFor(&conf)
		if err != nil {
			return nil, err
		}
		s.ConsulV1alpha1 = NewConsulV1alpha1Client(&restBackend{client: c})
	}
	{
		conf := *cfg
		conf.GroupVersion = &grafanav1alpha1.SchemaGroupVersion
		conf.APIPath = "/apis"
		conf.NegotiatedSerializer = Codecs.WithoutConversion()
		c, err := rest.RESTClientFor(&conf)
		if err != nil {
			return nil, err
		}
		s.GrafanaV1alpha1 = NewGrafanaV1alpha1Client(&restBackend{client: c})
	}
	{
		conf := *cfg
		conf.GroupVersion = &harborv1alpha1.SchemaGroupVersion
		conf.APIPath = "/apis"
		conf.NegotiatedSerializer = Codecs.WithoutConversion()
		c, err := rest.RESTClientFor(&conf)
		if err != nil {
			return nil, err
		}
		s.HarborV1alpha1 = NewHarborV1alpha1Client(&restBackend{client: c})
	}
	{
		conf := *cfg
		conf.GroupVersion = &miniov1alpha1.SchemaGroupVersion
		conf.APIPath = "/apis"
		conf.NegotiatedSerializer = Codecs.WithoutConversion()
		c, err := rest.RESTClientFor(&conf)
		if err != nil {
			return nil, err
		}
		s.MinioV1alpha1 = NewMinioV1alpha1Client(&restBackend{client: c})
	}
	{
		conf := *cfg
		conf.GroupVersion = &v1beta1.SchemaGroupVersion
		conf.APIPath = "/apis"
		conf.NegotiatedSerializer = Codecs.WithoutConversion()
		c, err := rest.RESTClientFor(&conf)
		if err != nil {
			return nil, err
		}
		s.MiniocontrollerV1beta1 = NewMiniocontrollerV1beta1Client(&restBackend{client: c})
	}

	return s, nil
}

type restBackend struct {
	client *rest.RESTClient
}

func (r *restBackend) Get(ctx context.Context, resourceName, kindName, namespace, name string, opts metav1.GetOptions, result runtime.Object) (runtime.Object, error) {
	return result, r.client.Get().
		Namespace(namespace).
		Resource(resourceName).
		Name(name).
		VersionedParams(&opts, ParameterCodec).
		Do(ctx).
		Into(result)
}

func (r *restBackend) List(ctx context.Context, resourceName, kindName, namespace string, opts metav1.ListOptions, result runtime.Object) (runtime.Object, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	return result, r.client.Get().
		Namespace(namespace).
		Resource(resourceName).
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
}

func (r *restBackend) Create(ctx context.Context, resourceName, kindName string, obj runtime.Object, opts metav1.CreateOptions, result runtime.Object) (runtime.Object, error) {
	m := obj.(metav1.Object)
	if m == nil {
		return nil, errors.New("obj is not implement metav1.Object")
	}
	return result, r.client.Post().
		Namespace(m.GetNamespace()).
		Resource(resourceName).
		VersionedParams(&opts, ParameterCodec).
		Body(obj).
		Do(ctx).
		Into(result)
}

func (r *restBackend) Update(ctx context.Context, resourceName, kindName string, obj runtime.Object, opts metav1.UpdateOptions, result runtime.Object) (runtime.Object, error) {
	m := obj.(metav1.Object)
	if m == nil {
		return nil, errors.New("obj is not implement metav1.Object")
	}
	return result, r.client.Put().
		Namespace(m.GetNamespace()).
		Resource(resourceName).
		Name(m.GetName()).
		VersionedParams(&opts, ParameterCodec).
		Body(obj).
		Do(ctx).
		Into(result)
}

func (r *restBackend) UpdateStatus(ctx context.Context, resourceName, kindName string, obj runtime.Object, opts metav1.UpdateOptions, result runtime.Object) (runtime.Object, error) {
	m := obj.(metav1.Object)
	if m == nil {
		return nil, errors.New("obj is not implement metav1.Object")
	}
	return result, r.client.Put().
		Namespace(m.GetNamespace()).
		Resource(resourceName).
		Name(m.GetName()).
		SubResource("status").
		VersionedParams(&opts, ParameterCodec).
		Body(obj).
		Do(ctx).
		Into(result)
}

func (r *restBackend) Delete(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string, opts metav1.DeleteOptions) error {
	return r.client.Delete().
		Namespace(namespace).
		Resource(gvr.Resource).
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (r *restBackend) Watch(ctx context.Context, gvr schema.GroupVersionResource, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return r.client.Get().
		Namespace(namespace).
		Resource(gvr.Resource).
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

type ConsulV1alpha1 struct {
	backend Backend
}

func NewConsulV1alpha1Client(b Backend) *ConsulV1alpha1 {
	return &ConsulV1alpha1{backend: b}
}

func (c *ConsulV1alpha1) GetConsulBackup(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*consulv1alpha1.ConsulBackup, error) {
	result, err := c.backend.Get(ctx, "consulbackups", "ConsulBackup", namespace, name, opts, &consulv1alpha1.ConsulBackup{})
	if err != nil {
		return nil, err
	}
	return result.(*consulv1alpha1.ConsulBackup), nil
}

func (c *ConsulV1alpha1) CreateConsulBackup(ctx context.Context, v *consulv1alpha1.ConsulBackup, opts metav1.CreateOptions) (*consulv1alpha1.ConsulBackup, error) {
	result, err := c.backend.Create(ctx, "consulbackups", "ConsulBackup", v, opts, &consulv1alpha1.ConsulBackup{})
	if err != nil {
		return nil, err
	}
	return result.(*consulv1alpha1.ConsulBackup), nil
}

func (c *ConsulV1alpha1) UpdateConsulBackup(ctx context.Context, v *consulv1alpha1.ConsulBackup, opts metav1.UpdateOptions) (*consulv1alpha1.ConsulBackup, error) {
	result, err := c.backend.Update(ctx, "consulbackups", "ConsulBackup", v, opts, &consulv1alpha1.ConsulBackup{})
	if err != nil {
		return nil, err
	}
	return result.(*consulv1alpha1.ConsulBackup), nil
}

func (c *ConsulV1alpha1) UpdateStatusConsulBackup(ctx context.Context, v *consulv1alpha1.ConsulBackup, opts metav1.UpdateOptions) (*consulv1alpha1.ConsulBackup, error) {
	result, err := c.backend.UpdateStatus(ctx, "consulbackups", "ConsulBackup", v, opts, &consulv1alpha1.ConsulBackup{})
	if err != nil {
		return nil, err
	}
	return result.(*consulv1alpha1.ConsulBackup), nil
}

func (c *ConsulV1alpha1) DeleteConsulBackup(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.backend.Delete(ctx, schema.GroupVersionResource{Group: "consul.f110.dev", Version: "v1alpha1", Resource: "consulbackups"}, namespace, name, opts)
}

func (c *ConsulV1alpha1) ListConsulBackup(ctx context.Context, namespace string, opts metav1.ListOptions) (*consulv1alpha1.ConsulBackupList, error) {
	result, err := c.backend.List(ctx, "consulbackups", "ConsulBackup", namespace, opts, &consulv1alpha1.ConsulBackupList{})
	if err != nil {
		return nil, err
	}
	return result.(*consulv1alpha1.ConsulBackupList), nil
}

func (c *ConsulV1alpha1) WatchConsulBackup(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.backend.Watch(ctx, schema.GroupVersionResource{Group: "consul.f110.dev", Version: "v1alpha1", Resource: "consulbackups"}, namespace, opts)
}

type GrafanaV1alpha1 struct {
	backend Backend
}

func NewGrafanaV1alpha1Client(b Backend) *GrafanaV1alpha1 {
	return &GrafanaV1alpha1{backend: b}
}

func (c *GrafanaV1alpha1) GetGrafana(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*grafanav1alpha1.Grafana, error) {
	result, err := c.backend.Get(ctx, "grafanas", "Grafana", namespace, name, opts, &grafanav1alpha1.Grafana{})
	if err != nil {
		return nil, err
	}
	return result.(*grafanav1alpha1.Grafana), nil
}

func (c *GrafanaV1alpha1) CreateGrafana(ctx context.Context, v *grafanav1alpha1.Grafana, opts metav1.CreateOptions) (*grafanav1alpha1.Grafana, error) {
	result, err := c.backend.Create(ctx, "grafanas", "Grafana", v, opts, &grafanav1alpha1.Grafana{})
	if err != nil {
		return nil, err
	}
	return result.(*grafanav1alpha1.Grafana), nil
}

func (c *GrafanaV1alpha1) UpdateGrafana(ctx context.Context, v *grafanav1alpha1.Grafana, opts metav1.UpdateOptions) (*grafanav1alpha1.Grafana, error) {
	result, err := c.backend.Update(ctx, "grafanas", "Grafana", v, opts, &grafanav1alpha1.Grafana{})
	if err != nil {
		return nil, err
	}
	return result.(*grafanav1alpha1.Grafana), nil
}

func (c *GrafanaV1alpha1) UpdateStatusGrafana(ctx context.Context, v *grafanav1alpha1.Grafana, opts metav1.UpdateOptions) (*grafanav1alpha1.Grafana, error) {
	result, err := c.backend.UpdateStatus(ctx, "grafanas", "Grafana", v, opts, &grafanav1alpha1.Grafana{})
	if err != nil {
		return nil, err
	}
	return result.(*grafanav1alpha1.Grafana), nil
}

func (c *GrafanaV1alpha1) DeleteGrafana(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.backend.Delete(ctx, schema.GroupVersionResource{Group: "grafana.f110.dev", Version: "v1alpha1", Resource: "grafanas"}, namespace, name, opts)
}

func (c *GrafanaV1alpha1) ListGrafana(ctx context.Context, namespace string, opts metav1.ListOptions) (*grafanav1alpha1.GrafanaList, error) {
	result, err := c.backend.List(ctx, "grafanas", "Grafana", namespace, opts, &grafanav1alpha1.GrafanaList{})
	if err != nil {
		return nil, err
	}
	return result.(*grafanav1alpha1.GrafanaList), nil
}

func (c *GrafanaV1alpha1) WatchGrafana(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.backend.Watch(ctx, schema.GroupVersionResource{Group: "grafana.f110.dev", Version: "v1alpha1", Resource: "grafanas"}, namespace, opts)
}

func (c *GrafanaV1alpha1) GetGrafanaUser(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*grafanav1alpha1.GrafanaUser, error) {
	result, err := c.backend.Get(ctx, "grafanausers", "GrafanaUser", namespace, name, opts, &grafanav1alpha1.GrafanaUser{})
	if err != nil {
		return nil, err
	}
	return result.(*grafanav1alpha1.GrafanaUser), nil
}

func (c *GrafanaV1alpha1) CreateGrafanaUser(ctx context.Context, v *grafanav1alpha1.GrafanaUser, opts metav1.CreateOptions) (*grafanav1alpha1.GrafanaUser, error) {
	result, err := c.backend.Create(ctx, "grafanausers", "GrafanaUser", v, opts, &grafanav1alpha1.GrafanaUser{})
	if err != nil {
		return nil, err
	}
	return result.(*grafanav1alpha1.GrafanaUser), nil
}

func (c *GrafanaV1alpha1) UpdateGrafanaUser(ctx context.Context, v *grafanav1alpha1.GrafanaUser, opts metav1.UpdateOptions) (*grafanav1alpha1.GrafanaUser, error) {
	result, err := c.backend.Update(ctx, "grafanausers", "GrafanaUser", v, opts, &grafanav1alpha1.GrafanaUser{})
	if err != nil {
		return nil, err
	}
	return result.(*grafanav1alpha1.GrafanaUser), nil
}

func (c *GrafanaV1alpha1) UpdateStatusGrafanaUser(ctx context.Context, v *grafanav1alpha1.GrafanaUser, opts metav1.UpdateOptions) (*grafanav1alpha1.GrafanaUser, error) {
	result, err := c.backend.UpdateStatus(ctx, "grafanausers", "GrafanaUser", v, opts, &grafanav1alpha1.GrafanaUser{})
	if err != nil {
		return nil, err
	}
	return result.(*grafanav1alpha1.GrafanaUser), nil
}

func (c *GrafanaV1alpha1) DeleteGrafanaUser(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.backend.Delete(ctx, schema.GroupVersionResource{Group: "grafana.f110.dev", Version: "v1alpha1", Resource: "grafanausers"}, namespace, name, opts)
}

func (c *GrafanaV1alpha1) ListGrafanaUser(ctx context.Context, namespace string, opts metav1.ListOptions) (*grafanav1alpha1.GrafanaUserList, error) {
	result, err := c.backend.List(ctx, "grafanausers", "GrafanaUser", namespace, opts, &grafanav1alpha1.GrafanaUserList{})
	if err != nil {
		return nil, err
	}
	return result.(*grafanav1alpha1.GrafanaUserList), nil
}

func (c *GrafanaV1alpha1) WatchGrafanaUser(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.backend.Watch(ctx, schema.GroupVersionResource{Group: "grafana.f110.dev", Version: "v1alpha1", Resource: "grafanausers"}, namespace, opts)
}

type HarborV1alpha1 struct {
	backend Backend
}

func NewHarborV1alpha1Client(b Backend) *HarborV1alpha1 {
	return &HarborV1alpha1{backend: b}
}

func (c *HarborV1alpha1) GetHarborProject(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*harborv1alpha1.HarborProject, error) {
	result, err := c.backend.Get(ctx, "harborprojects", "HarborProject", namespace, name, opts, &harborv1alpha1.HarborProject{})
	if err != nil {
		return nil, err
	}
	return result.(*harborv1alpha1.HarborProject), nil
}

func (c *HarborV1alpha1) CreateHarborProject(ctx context.Context, v *harborv1alpha1.HarborProject, opts metav1.CreateOptions) (*harborv1alpha1.HarborProject, error) {
	result, err := c.backend.Create(ctx, "harborprojects", "HarborProject", v, opts, &harborv1alpha1.HarborProject{})
	if err != nil {
		return nil, err
	}
	return result.(*harborv1alpha1.HarborProject), nil
}

func (c *HarborV1alpha1) UpdateHarborProject(ctx context.Context, v *harborv1alpha1.HarborProject, opts metav1.UpdateOptions) (*harborv1alpha1.HarborProject, error) {
	result, err := c.backend.Update(ctx, "harborprojects", "HarborProject", v, opts, &harborv1alpha1.HarborProject{})
	if err != nil {
		return nil, err
	}
	return result.(*harborv1alpha1.HarborProject), nil
}

func (c *HarborV1alpha1) UpdateStatusHarborProject(ctx context.Context, v *harborv1alpha1.HarborProject, opts metav1.UpdateOptions) (*harborv1alpha1.HarborProject, error) {
	result, err := c.backend.UpdateStatus(ctx, "harborprojects", "HarborProject", v, opts, &harborv1alpha1.HarborProject{})
	if err != nil {
		return nil, err
	}
	return result.(*harborv1alpha1.HarborProject), nil
}

func (c *HarborV1alpha1) DeleteHarborProject(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.backend.Delete(ctx, schema.GroupVersionResource{Group: "harbor.f110.dev", Version: "v1alpha1", Resource: "harborprojects"}, namespace, name, opts)
}

func (c *HarborV1alpha1) ListHarborProject(ctx context.Context, namespace string, opts metav1.ListOptions) (*harborv1alpha1.HarborProjectList, error) {
	result, err := c.backend.List(ctx, "harborprojects", "HarborProject", namespace, opts, &harborv1alpha1.HarborProjectList{})
	if err != nil {
		return nil, err
	}
	return result.(*harborv1alpha1.HarborProjectList), nil
}

func (c *HarborV1alpha1) WatchHarborProject(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.backend.Watch(ctx, schema.GroupVersionResource{Group: "harbor.f110.dev", Version: "v1alpha1", Resource: "harborprojects"}, namespace, opts)
}

func (c *HarborV1alpha1) GetHarborRobotAccount(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*harborv1alpha1.HarborRobotAccount, error) {
	result, err := c.backend.Get(ctx, "harborrobotaccounts", "HarborRobotAccount", namespace, name, opts, &harborv1alpha1.HarborRobotAccount{})
	if err != nil {
		return nil, err
	}
	return result.(*harborv1alpha1.HarborRobotAccount), nil
}

func (c *HarborV1alpha1) CreateHarborRobotAccount(ctx context.Context, v *harborv1alpha1.HarborRobotAccount, opts metav1.CreateOptions) (*harborv1alpha1.HarborRobotAccount, error) {
	result, err := c.backend.Create(ctx, "harborrobotaccounts", "HarborRobotAccount", v, opts, &harborv1alpha1.HarborRobotAccount{})
	if err != nil {
		return nil, err
	}
	return result.(*harborv1alpha1.HarborRobotAccount), nil
}

func (c *HarborV1alpha1) UpdateHarborRobotAccount(ctx context.Context, v *harborv1alpha1.HarborRobotAccount, opts metav1.UpdateOptions) (*harborv1alpha1.HarborRobotAccount, error) {
	result, err := c.backend.Update(ctx, "harborrobotaccounts", "HarborRobotAccount", v, opts, &harborv1alpha1.HarborRobotAccount{})
	if err != nil {
		return nil, err
	}
	return result.(*harborv1alpha1.HarborRobotAccount), nil
}

func (c *HarborV1alpha1) UpdateStatusHarborRobotAccount(ctx context.Context, v *harborv1alpha1.HarborRobotAccount, opts metav1.UpdateOptions) (*harborv1alpha1.HarborRobotAccount, error) {
	result, err := c.backend.UpdateStatus(ctx, "harborrobotaccounts", "HarborRobotAccount", v, opts, &harborv1alpha1.HarborRobotAccount{})
	if err != nil {
		return nil, err
	}
	return result.(*harborv1alpha1.HarborRobotAccount), nil
}

func (c *HarborV1alpha1) DeleteHarborRobotAccount(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.backend.Delete(ctx, schema.GroupVersionResource{Group: "harbor.f110.dev", Version: "v1alpha1", Resource: "harborrobotaccounts"}, namespace, name, opts)
}

func (c *HarborV1alpha1) ListHarborRobotAccount(ctx context.Context, namespace string, opts metav1.ListOptions) (*harborv1alpha1.HarborRobotAccountList, error) {
	result, err := c.backend.List(ctx, "harborrobotaccounts", "HarborRobotAccount", namespace, opts, &harborv1alpha1.HarborRobotAccountList{})
	if err != nil {
		return nil, err
	}
	return result.(*harborv1alpha1.HarborRobotAccountList), nil
}

func (c *HarborV1alpha1) WatchHarborRobotAccount(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.backend.Watch(ctx, schema.GroupVersionResource{Group: "harbor.f110.dev", Version: "v1alpha1", Resource: "harborrobotaccounts"}, namespace, opts)
}

type MinioV1alpha1 struct {
	backend Backend
}

func NewMinioV1alpha1Client(b Backend) *MinioV1alpha1 {
	return &MinioV1alpha1{backend: b}
}

func (c *MinioV1alpha1) GetMinIOBucket(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*miniov1alpha1.MinIOBucket, error) {
	result, err := c.backend.Get(ctx, "miniobuckets", "MinIOBucket", namespace, name, opts, &miniov1alpha1.MinIOBucket{})
	if err != nil {
		return nil, err
	}
	return result.(*miniov1alpha1.MinIOBucket), nil
}

func (c *MinioV1alpha1) CreateMinIOBucket(ctx context.Context, v *miniov1alpha1.MinIOBucket, opts metav1.CreateOptions) (*miniov1alpha1.MinIOBucket, error) {
	result, err := c.backend.Create(ctx, "miniobuckets", "MinIOBucket", v, opts, &miniov1alpha1.MinIOBucket{})
	if err != nil {
		return nil, err
	}
	return result.(*miniov1alpha1.MinIOBucket), nil
}

func (c *MinioV1alpha1) UpdateMinIOBucket(ctx context.Context, v *miniov1alpha1.MinIOBucket, opts metav1.UpdateOptions) (*miniov1alpha1.MinIOBucket, error) {
	result, err := c.backend.Update(ctx, "miniobuckets", "MinIOBucket", v, opts, &miniov1alpha1.MinIOBucket{})
	if err != nil {
		return nil, err
	}
	return result.(*miniov1alpha1.MinIOBucket), nil
}

func (c *MinioV1alpha1) UpdateStatusMinIOBucket(ctx context.Context, v *miniov1alpha1.MinIOBucket, opts metav1.UpdateOptions) (*miniov1alpha1.MinIOBucket, error) {
	result, err := c.backend.UpdateStatus(ctx, "miniobuckets", "MinIOBucket", v, opts, &miniov1alpha1.MinIOBucket{})
	if err != nil {
		return nil, err
	}
	return result.(*miniov1alpha1.MinIOBucket), nil
}

func (c *MinioV1alpha1) DeleteMinIOBucket(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.backend.Delete(ctx, schema.GroupVersionResource{Group: "minio.f110.dev", Version: "v1alpha1", Resource: "miniobuckets"}, namespace, name, opts)
}

func (c *MinioV1alpha1) ListMinIOBucket(ctx context.Context, namespace string, opts metav1.ListOptions) (*miniov1alpha1.MinIOBucketList, error) {
	result, err := c.backend.List(ctx, "miniobuckets", "MinIOBucket", namespace, opts, &miniov1alpha1.MinIOBucketList{})
	if err != nil {
		return nil, err
	}
	return result.(*miniov1alpha1.MinIOBucketList), nil
}

func (c *MinioV1alpha1) WatchMinIOBucket(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.backend.Watch(ctx, schema.GroupVersionResource{Group: "minio.f110.dev", Version: "v1alpha1", Resource: "miniobuckets"}, namespace, opts)
}

func (c *MinioV1alpha1) GetMinIOUser(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*miniov1alpha1.MinIOUser, error) {
	result, err := c.backend.Get(ctx, "miniousers", "MinIOUser", namespace, name, opts, &miniov1alpha1.MinIOUser{})
	if err != nil {
		return nil, err
	}
	return result.(*miniov1alpha1.MinIOUser), nil
}

func (c *MinioV1alpha1) CreateMinIOUser(ctx context.Context, v *miniov1alpha1.MinIOUser, opts metav1.CreateOptions) (*miniov1alpha1.MinIOUser, error) {
	result, err := c.backend.Create(ctx, "miniousers", "MinIOUser", v, opts, &miniov1alpha1.MinIOUser{})
	if err != nil {
		return nil, err
	}
	return result.(*miniov1alpha1.MinIOUser), nil
}

func (c *MinioV1alpha1) UpdateMinIOUser(ctx context.Context, v *miniov1alpha1.MinIOUser, opts metav1.UpdateOptions) (*miniov1alpha1.MinIOUser, error) {
	result, err := c.backend.Update(ctx, "miniousers", "MinIOUser", v, opts, &miniov1alpha1.MinIOUser{})
	if err != nil {
		return nil, err
	}
	return result.(*miniov1alpha1.MinIOUser), nil
}

func (c *MinioV1alpha1) UpdateStatusMinIOUser(ctx context.Context, v *miniov1alpha1.MinIOUser, opts metav1.UpdateOptions) (*miniov1alpha1.MinIOUser, error) {
	result, err := c.backend.UpdateStatus(ctx, "miniousers", "MinIOUser", v, opts, &miniov1alpha1.MinIOUser{})
	if err != nil {
		return nil, err
	}
	return result.(*miniov1alpha1.MinIOUser), nil
}

func (c *MinioV1alpha1) DeleteMinIOUser(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.backend.Delete(ctx, schema.GroupVersionResource{Group: "minio.f110.dev", Version: "v1alpha1", Resource: "miniousers"}, namespace, name, opts)
}

func (c *MinioV1alpha1) ListMinIOUser(ctx context.Context, namespace string, opts metav1.ListOptions) (*miniov1alpha1.MinIOUserList, error) {
	result, err := c.backend.List(ctx, "miniousers", "MinIOUser", namespace, opts, &miniov1alpha1.MinIOUserList{})
	if err != nil {
		return nil, err
	}
	return result.(*miniov1alpha1.MinIOUserList), nil
}

func (c *MinioV1alpha1) WatchMinIOUser(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.backend.Watch(ctx, schema.GroupVersionResource{Group: "minio.f110.dev", Version: "v1alpha1", Resource: "miniousers"}, namespace, opts)
}

type MiniocontrollerV1beta1 struct {
	backend Backend
}

func NewMiniocontrollerV1beta1Client(b Backend) *MiniocontrollerV1beta1 {
	return &MiniocontrollerV1beta1{backend: b}
}

func (c *MiniocontrollerV1beta1) GetMinIOInstance(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*v1beta1.MinIOInstance, error) {
	result, err := c.backend.Get(ctx, "minioinstances", "MinIOInstance", namespace, name, opts, &v1beta1.MinIOInstance{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.MinIOInstance), nil
}

func (c *MiniocontrollerV1beta1) CreateMinIOInstance(ctx context.Context, v *v1beta1.MinIOInstance, opts metav1.CreateOptions) (*v1beta1.MinIOInstance, error) {
	result, err := c.backend.Create(ctx, "minioinstances", "MinIOInstance", v, opts, &v1beta1.MinIOInstance{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.MinIOInstance), nil
}

func (c *MiniocontrollerV1beta1) UpdateMinIOInstance(ctx context.Context, v *v1beta1.MinIOInstance, opts metav1.UpdateOptions) (*v1beta1.MinIOInstance, error) {
	result, err := c.backend.Update(ctx, "minioinstances", "MinIOInstance", v, opts, &v1beta1.MinIOInstance{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.MinIOInstance), nil
}

func (c *MiniocontrollerV1beta1) DeleteMinIOInstance(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.backend.Delete(ctx, schema.GroupVersionResource{Group: "miniocontroller.min.io", Version: "v1beta1", Resource: "minioinstances"}, namespace, name, opts)
}

func (c *MiniocontrollerV1beta1) ListMinIOInstance(ctx context.Context, namespace string, opts metav1.ListOptions) (*v1beta1.MinIOInstanceList, error) {
	result, err := c.backend.List(ctx, "minioinstances", "MinIOInstance", namespace, opts, &v1beta1.MinIOInstanceList{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.MinIOInstanceList), nil
}

func (c *MiniocontrollerV1beta1) WatchMinIOInstance(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.backend.Watch(ctx, schema.GroupVersionResource{Group: "miniocontroller.min.io", Version: "v1beta1", Resource: "minioinstances"}, namespace, opts)
}

func (c *MiniocontrollerV1beta1) GetMirror(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*v1beta1.Mirror, error) {
	result, err := c.backend.Get(ctx, "mirrors", "Mirror", namespace, name, opts, &v1beta1.Mirror{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.Mirror), nil
}

func (c *MiniocontrollerV1beta1) CreateMirror(ctx context.Context, v *v1beta1.Mirror, opts metav1.CreateOptions) (*v1beta1.Mirror, error) {
	result, err := c.backend.Create(ctx, "mirrors", "Mirror", v, opts, &v1beta1.Mirror{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.Mirror), nil
}

func (c *MiniocontrollerV1beta1) UpdateMirror(ctx context.Context, v *v1beta1.Mirror, opts metav1.UpdateOptions) (*v1beta1.Mirror, error) {
	result, err := c.backend.Update(ctx, "mirrors", "Mirror", v, opts, &v1beta1.Mirror{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.Mirror), nil
}

func (c *MiniocontrollerV1beta1) DeleteMirror(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.backend.Delete(ctx, schema.GroupVersionResource{Group: "miniocontroller.min.io", Version: "v1beta1", Resource: "mirrors"}, namespace, name, opts)
}

func (c *MiniocontrollerV1beta1) ListMirror(ctx context.Context, namespace string, opts metav1.ListOptions) (*v1beta1.MirrorList, error) {
	result, err := c.backend.List(ctx, "mirrors", "Mirror", namespace, opts, &v1beta1.MirrorList{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.MirrorList), nil
}

func (c *MiniocontrollerV1beta1) WatchMirror(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.backend.Watch(ctx, schema.GroupVersionResource{Group: "miniocontroller.min.io", Version: "v1beta1", Resource: "mirrors"}, namespace, opts)
}

type InformerCache struct {
	mu        sync.Mutex
	informers map[reflect.Type]cache.SharedIndexInformer
}

func NewInformerCache() *InformerCache {
	return &InformerCache{informers: make(map[reflect.Type]cache.SharedIndexInformer)}
}

func (c *InformerCache) Write(obj runtime.Object, newFunc func() cache.SharedIndexInformer) cache.SharedIndexInformer {
	c.mu.Lock()
	defer c.mu.Unlock()

	typ := reflect.TypeOf(obj)
	if v, ok := c.informers[typ]; ok {
		return v
	}
	informer := newFunc()
	c.informers[typ] = informer

	return informer
}

func (c *InformerCache) Informers() []cache.SharedIndexInformer {
	c.mu.Lock()
	defer c.mu.Unlock()

	a := make([]cache.SharedIndexInformer, 0, len(c.informers))
	for _, v := range c.informers {
		a = append(a, v)
	}

	return a
}

type InformerFactory struct {
	set   *Set
	cache *InformerCache

	namespace    string
	resyncPeriod time.Duration
}

func NewInformerFactory(s *Set, c *InformerCache, namespace string, resyncPeriod time.Duration) *InformerFactory {
	return &InformerFactory{set: s, cache: c, namespace: namespace, resyncPeriod: resyncPeriod}
}

func (f *InformerFactory) Cache() *InformerCache {
	return f.cache
}

func (f *InformerFactory) InformerFor(obj runtime.Object) cache.SharedIndexInformer {
	switch obj.(type) {
	case *consulv1alpha1.ConsulBackup:
		return NewConsulV1alpha1Informer(f.cache, f.set.ConsulV1alpha1, f.namespace, f.resyncPeriod).ConsulBackupInformer()
	case *grafanav1alpha1.Grafana:
		return NewGrafanaV1alpha1Informer(f.cache, f.set.GrafanaV1alpha1, f.namespace, f.resyncPeriod).GrafanaInformer()
	case *grafanav1alpha1.GrafanaUser:
		return NewGrafanaV1alpha1Informer(f.cache, f.set.GrafanaV1alpha1, f.namespace, f.resyncPeriod).GrafanaUserInformer()
	case *harborv1alpha1.HarborProject:
		return NewHarborV1alpha1Informer(f.cache, f.set.HarborV1alpha1, f.namespace, f.resyncPeriod).HarborProjectInformer()
	case *harborv1alpha1.HarborRobotAccount:
		return NewHarborV1alpha1Informer(f.cache, f.set.HarborV1alpha1, f.namespace, f.resyncPeriod).HarborRobotAccountInformer()
	case *miniov1alpha1.MinIOBucket:
		return NewMinioV1alpha1Informer(f.cache, f.set.MinioV1alpha1, f.namespace, f.resyncPeriod).MinIOBucketInformer()
	case *miniov1alpha1.MinIOUser:
		return NewMinioV1alpha1Informer(f.cache, f.set.MinioV1alpha1, f.namespace, f.resyncPeriod).MinIOUserInformer()
	case *v1beta1.MinIOInstance:
		return NewMiniocontrollerV1beta1Informer(f.cache, f.set.MiniocontrollerV1beta1, f.namespace, f.resyncPeriod).MinIOInstanceInformer()
	case *v1beta1.Mirror:
		return NewMiniocontrollerV1beta1Informer(f.cache, f.set.MiniocontrollerV1beta1, f.namespace, f.resyncPeriod).MirrorInformer()
	default:
		return nil
	}
}

func (f *InformerFactory) InformerForResource(gvr schema.GroupVersionResource) cache.SharedIndexInformer {
	switch gvr {
	case consulv1alpha1.SchemaGroupVersion.WithResource("consulbackups"):
		return NewConsulV1alpha1Informer(f.cache, f.set.ConsulV1alpha1, f.namespace, f.resyncPeriod).ConsulBackupInformer()
	case grafanav1alpha1.SchemaGroupVersion.WithResource("grafanas"):
		return NewGrafanaV1alpha1Informer(f.cache, f.set.GrafanaV1alpha1, f.namespace, f.resyncPeriod).GrafanaInformer()
	case grafanav1alpha1.SchemaGroupVersion.WithResource("grafanausers"):
		return NewGrafanaV1alpha1Informer(f.cache, f.set.GrafanaV1alpha1, f.namespace, f.resyncPeriod).GrafanaUserInformer()
	case harborv1alpha1.SchemaGroupVersion.WithResource("harborprojects"):
		return NewHarborV1alpha1Informer(f.cache, f.set.HarborV1alpha1, f.namespace, f.resyncPeriod).HarborProjectInformer()
	case harborv1alpha1.SchemaGroupVersion.WithResource("harborrobotaccounts"):
		return NewHarborV1alpha1Informer(f.cache, f.set.HarborV1alpha1, f.namespace, f.resyncPeriod).HarborRobotAccountInformer()
	case miniov1alpha1.SchemaGroupVersion.WithResource("miniobuckets"):
		return NewMinioV1alpha1Informer(f.cache, f.set.MinioV1alpha1, f.namespace, f.resyncPeriod).MinIOBucketInformer()
	case miniov1alpha1.SchemaGroupVersion.WithResource("miniousers"):
		return NewMinioV1alpha1Informer(f.cache, f.set.MinioV1alpha1, f.namespace, f.resyncPeriod).MinIOUserInformer()
	case v1beta1.SchemaGroupVersion.WithResource("minioinstances"):
		return NewMiniocontrollerV1beta1Informer(f.cache, f.set.MiniocontrollerV1beta1, f.namespace, f.resyncPeriod).MinIOInstanceInformer()
	case v1beta1.SchemaGroupVersion.WithResource("mirrors"):
		return NewMiniocontrollerV1beta1Informer(f.cache, f.set.MiniocontrollerV1beta1, f.namespace, f.resyncPeriod).MirrorInformer()
	default:
		return nil
	}
}

func (f *InformerFactory) Run(ctx context.Context) {
	for _, v := range f.cache.Informers() {
		go v.Run(ctx.Done())
	}
}

type ConsulV1alpha1Informer struct {
	cache        *InformerCache
	client       *ConsulV1alpha1
	namespace    string
	resyncPeriod time.Duration
	indexers     cache.Indexers
}

func NewConsulV1alpha1Informer(c *InformerCache, client *ConsulV1alpha1, namespace string, resyncPeriod time.Duration) *ConsulV1alpha1Informer {
	return &ConsulV1alpha1Informer{
		cache:        c,
		client:       client,
		namespace:    namespace,
		resyncPeriod: resyncPeriod,
		indexers:     cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	}
}

func (f *ConsulV1alpha1Informer) ConsulBackupInformer() cache.SharedIndexInformer {
	return f.cache.Write(&consulv1alpha1.ConsulBackup{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return f.client.ListConsulBackup(context.TODO(), f.namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchConsulBackup(context.TODO(), f.namespace, metav1.ListOptions{})
				},
			},
			&consulv1alpha1.ConsulBackup{},
			f.resyncPeriod,
			f.indexers,
		)
	})
}

func (f *ConsulV1alpha1Informer) ConsulBackupLister() *ConsulV1alpha1ConsulBackupLister {
	return NewConsulV1alpha1ConsulBackupLister(f.ConsulBackupInformer().GetIndexer())
}

type GrafanaV1alpha1Informer struct {
	cache        *InformerCache
	client       *GrafanaV1alpha1
	namespace    string
	resyncPeriod time.Duration
	indexers     cache.Indexers
}

func NewGrafanaV1alpha1Informer(c *InformerCache, client *GrafanaV1alpha1, namespace string, resyncPeriod time.Duration) *GrafanaV1alpha1Informer {
	return &GrafanaV1alpha1Informer{
		cache:        c,
		client:       client,
		namespace:    namespace,
		resyncPeriod: resyncPeriod,
		indexers:     cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	}
}

func (f *GrafanaV1alpha1Informer) GrafanaInformer() cache.SharedIndexInformer {
	return f.cache.Write(&grafanav1alpha1.Grafana{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return f.client.ListGrafana(context.TODO(), f.namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchGrafana(context.TODO(), f.namespace, metav1.ListOptions{})
				},
			},
			&grafanav1alpha1.Grafana{},
			f.resyncPeriod,
			f.indexers,
		)
	})
}

func (f *GrafanaV1alpha1Informer) GrafanaLister() *GrafanaV1alpha1GrafanaLister {
	return NewGrafanaV1alpha1GrafanaLister(f.GrafanaInformer().GetIndexer())
}

func (f *GrafanaV1alpha1Informer) GrafanaUserInformer() cache.SharedIndexInformer {
	return f.cache.Write(&grafanav1alpha1.GrafanaUser{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return f.client.ListGrafanaUser(context.TODO(), f.namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchGrafanaUser(context.TODO(), f.namespace, metav1.ListOptions{})
				},
			},
			&grafanav1alpha1.GrafanaUser{},
			f.resyncPeriod,
			f.indexers,
		)
	})
}

func (f *GrafanaV1alpha1Informer) GrafanaUserLister() *GrafanaV1alpha1GrafanaUserLister {
	return NewGrafanaV1alpha1GrafanaUserLister(f.GrafanaUserInformer().GetIndexer())
}

type HarborV1alpha1Informer struct {
	cache        *InformerCache
	client       *HarborV1alpha1
	namespace    string
	resyncPeriod time.Duration
	indexers     cache.Indexers
}

func NewHarborV1alpha1Informer(c *InformerCache, client *HarborV1alpha1, namespace string, resyncPeriod time.Duration) *HarborV1alpha1Informer {
	return &HarborV1alpha1Informer{
		cache:        c,
		client:       client,
		namespace:    namespace,
		resyncPeriod: resyncPeriod,
		indexers:     cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	}
}

func (f *HarborV1alpha1Informer) HarborProjectInformer() cache.SharedIndexInformer {
	return f.cache.Write(&harborv1alpha1.HarborProject{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return f.client.ListHarborProject(context.TODO(), f.namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchHarborProject(context.TODO(), f.namespace, metav1.ListOptions{})
				},
			},
			&harborv1alpha1.HarborProject{},
			f.resyncPeriod,
			f.indexers,
		)
	})
}

func (f *HarborV1alpha1Informer) HarborProjectLister() *HarborV1alpha1HarborProjectLister {
	return NewHarborV1alpha1HarborProjectLister(f.HarborProjectInformer().GetIndexer())
}

func (f *HarborV1alpha1Informer) HarborRobotAccountInformer() cache.SharedIndexInformer {
	return f.cache.Write(&harborv1alpha1.HarborRobotAccount{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return f.client.ListHarborRobotAccount(context.TODO(), f.namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchHarborRobotAccount(context.TODO(), f.namespace, metav1.ListOptions{})
				},
			},
			&harborv1alpha1.HarborRobotAccount{},
			f.resyncPeriod,
			f.indexers,
		)
	})
}

func (f *HarborV1alpha1Informer) HarborRobotAccountLister() *HarborV1alpha1HarborRobotAccountLister {
	return NewHarborV1alpha1HarborRobotAccountLister(f.HarborRobotAccountInformer().GetIndexer())
}

type MinioV1alpha1Informer struct {
	cache        *InformerCache
	client       *MinioV1alpha1
	namespace    string
	resyncPeriod time.Duration
	indexers     cache.Indexers
}

func NewMinioV1alpha1Informer(c *InformerCache, client *MinioV1alpha1, namespace string, resyncPeriod time.Duration) *MinioV1alpha1Informer {
	return &MinioV1alpha1Informer{
		cache:        c,
		client:       client,
		namespace:    namespace,
		resyncPeriod: resyncPeriod,
		indexers:     cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	}
}

func (f *MinioV1alpha1Informer) MinIOBucketInformer() cache.SharedIndexInformer {
	return f.cache.Write(&miniov1alpha1.MinIOBucket{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return f.client.ListMinIOBucket(context.TODO(), f.namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchMinIOBucket(context.TODO(), f.namespace, metav1.ListOptions{})
				},
			},
			&miniov1alpha1.MinIOBucket{},
			f.resyncPeriod,
			f.indexers,
		)
	})
}

func (f *MinioV1alpha1Informer) MinIOBucketLister() *MinioV1alpha1MinIOBucketLister {
	return NewMinioV1alpha1MinIOBucketLister(f.MinIOBucketInformer().GetIndexer())
}

func (f *MinioV1alpha1Informer) MinIOUserInformer() cache.SharedIndexInformer {
	return f.cache.Write(&miniov1alpha1.MinIOUser{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return f.client.ListMinIOUser(context.TODO(), f.namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchMinIOUser(context.TODO(), f.namespace, metav1.ListOptions{})
				},
			},
			&miniov1alpha1.MinIOUser{},
			f.resyncPeriod,
			f.indexers,
		)
	})
}

func (f *MinioV1alpha1Informer) MinIOUserLister() *MinioV1alpha1MinIOUserLister {
	return NewMinioV1alpha1MinIOUserLister(f.MinIOUserInformer().GetIndexer())
}

type MiniocontrollerV1beta1Informer struct {
	cache        *InformerCache
	client       *MiniocontrollerV1beta1
	namespace    string
	resyncPeriod time.Duration
	indexers     cache.Indexers
}

func NewMiniocontrollerV1beta1Informer(c *InformerCache, client *MiniocontrollerV1beta1, namespace string, resyncPeriod time.Duration) *MiniocontrollerV1beta1Informer {
	return &MiniocontrollerV1beta1Informer{
		cache:        c,
		client:       client,
		namespace:    namespace,
		resyncPeriod: resyncPeriod,
		indexers:     cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	}
}

func (f *MiniocontrollerV1beta1Informer) MinIOInstanceInformer() cache.SharedIndexInformer {
	return f.cache.Write(&v1beta1.MinIOInstance{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return f.client.ListMinIOInstance(context.TODO(), f.namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchMinIOInstance(context.TODO(), f.namespace, metav1.ListOptions{})
				},
			},
			&v1beta1.MinIOInstance{},
			f.resyncPeriod,
			f.indexers,
		)
	})
}

func (f *MiniocontrollerV1beta1Informer) MinIOInstanceLister() *MiniocontrollerV1beta1MinIOInstanceLister {
	return NewMiniocontrollerV1beta1MinIOInstanceLister(f.MinIOInstanceInformer().GetIndexer())
}

func (f *MiniocontrollerV1beta1Informer) MirrorInformer() cache.SharedIndexInformer {
	return f.cache.Write(&v1beta1.Mirror{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return f.client.ListMirror(context.TODO(), f.namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchMirror(context.TODO(), f.namespace, metav1.ListOptions{})
				},
			},
			&v1beta1.Mirror{},
			f.resyncPeriod,
			f.indexers,
		)
	})
}

func (f *MiniocontrollerV1beta1Informer) MirrorLister() *MiniocontrollerV1beta1MirrorLister {
	return NewMiniocontrollerV1beta1MirrorLister(f.MirrorInformer().GetIndexer())
}

type ConsulV1alpha1ConsulBackupLister struct {
	indexer cache.Indexer
}

func NewConsulV1alpha1ConsulBackupLister(indexer cache.Indexer) *ConsulV1alpha1ConsulBackupLister {
	return &ConsulV1alpha1ConsulBackupLister{indexer: indexer}
}

func (x *ConsulV1alpha1ConsulBackupLister) List(namespace string, selector labels.Selector) ([]*consulv1alpha1.ConsulBackup, error) {
	var ret []*consulv1alpha1.ConsulBackup
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*consulv1alpha1.ConsulBackup).DeepCopy())
	})
	return ret, err
}

func (x *ConsulV1alpha1ConsulBackupLister) Get(namespace, name string) (*consulv1alpha1.ConsulBackup, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(consulv1alpha1.SchemaGroupVersion.WithResource("consulbackup").GroupResource(), name)
	}
	return obj.(*consulv1alpha1.ConsulBackup).DeepCopy(), nil
}

type GrafanaV1alpha1GrafanaLister struct {
	indexer cache.Indexer
}

func NewGrafanaV1alpha1GrafanaLister(indexer cache.Indexer) *GrafanaV1alpha1GrafanaLister {
	return &GrafanaV1alpha1GrafanaLister{indexer: indexer}
}

func (x *GrafanaV1alpha1GrafanaLister) List(namespace string, selector labels.Selector) ([]*grafanav1alpha1.Grafana, error) {
	var ret []*grafanav1alpha1.Grafana
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*grafanav1alpha1.Grafana).DeepCopy())
	})
	return ret, err
}

func (x *GrafanaV1alpha1GrafanaLister) Get(namespace, name string) (*grafanav1alpha1.Grafana, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(grafanav1alpha1.SchemaGroupVersion.WithResource("grafana").GroupResource(), name)
	}
	return obj.(*grafanav1alpha1.Grafana).DeepCopy(), nil
}

type GrafanaV1alpha1GrafanaUserLister struct {
	indexer cache.Indexer
}

func NewGrafanaV1alpha1GrafanaUserLister(indexer cache.Indexer) *GrafanaV1alpha1GrafanaUserLister {
	return &GrafanaV1alpha1GrafanaUserLister{indexer: indexer}
}

func (x *GrafanaV1alpha1GrafanaUserLister) List(namespace string, selector labels.Selector) ([]*grafanav1alpha1.GrafanaUser, error) {
	var ret []*grafanav1alpha1.GrafanaUser
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*grafanav1alpha1.GrafanaUser).DeepCopy())
	})
	return ret, err
}

func (x *GrafanaV1alpha1GrafanaUserLister) Get(namespace, name string) (*grafanav1alpha1.GrafanaUser, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(grafanav1alpha1.SchemaGroupVersion.WithResource("grafanauser").GroupResource(), name)
	}
	return obj.(*grafanav1alpha1.GrafanaUser).DeepCopy(), nil
}

type HarborV1alpha1HarborProjectLister struct {
	indexer cache.Indexer
}

func NewHarborV1alpha1HarborProjectLister(indexer cache.Indexer) *HarborV1alpha1HarborProjectLister {
	return &HarborV1alpha1HarborProjectLister{indexer: indexer}
}

func (x *HarborV1alpha1HarborProjectLister) List(namespace string, selector labels.Selector) ([]*harborv1alpha1.HarborProject, error) {
	var ret []*harborv1alpha1.HarborProject
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*harborv1alpha1.HarborProject).DeepCopy())
	})
	return ret, err
}

func (x *HarborV1alpha1HarborProjectLister) Get(namespace, name string) (*harborv1alpha1.HarborProject, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(harborv1alpha1.SchemaGroupVersion.WithResource("harborproject").GroupResource(), name)
	}
	return obj.(*harborv1alpha1.HarborProject).DeepCopy(), nil
}

type HarborV1alpha1HarborRobotAccountLister struct {
	indexer cache.Indexer
}

func NewHarborV1alpha1HarborRobotAccountLister(indexer cache.Indexer) *HarborV1alpha1HarborRobotAccountLister {
	return &HarborV1alpha1HarborRobotAccountLister{indexer: indexer}
}

func (x *HarborV1alpha1HarborRobotAccountLister) List(namespace string, selector labels.Selector) ([]*harborv1alpha1.HarborRobotAccount, error) {
	var ret []*harborv1alpha1.HarborRobotAccount
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*harborv1alpha1.HarborRobotAccount).DeepCopy())
	})
	return ret, err
}

func (x *HarborV1alpha1HarborRobotAccountLister) Get(namespace, name string) (*harborv1alpha1.HarborRobotAccount, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(harborv1alpha1.SchemaGroupVersion.WithResource("harborrobotaccount").GroupResource(), name)
	}
	return obj.(*harborv1alpha1.HarborRobotAccount).DeepCopy(), nil
}

type MinioV1alpha1MinIOBucketLister struct {
	indexer cache.Indexer
}

func NewMinioV1alpha1MinIOBucketLister(indexer cache.Indexer) *MinioV1alpha1MinIOBucketLister {
	return &MinioV1alpha1MinIOBucketLister{indexer: indexer}
}

func (x *MinioV1alpha1MinIOBucketLister) List(namespace string, selector labels.Selector) ([]*miniov1alpha1.MinIOBucket, error) {
	var ret []*miniov1alpha1.MinIOBucket
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*miniov1alpha1.MinIOBucket).DeepCopy())
	})
	return ret, err
}

func (x *MinioV1alpha1MinIOBucketLister) Get(namespace, name string) (*miniov1alpha1.MinIOBucket, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(miniov1alpha1.SchemaGroupVersion.WithResource("miniobucket").GroupResource(), name)
	}
	return obj.(*miniov1alpha1.MinIOBucket).DeepCopy(), nil
}

type MinioV1alpha1MinIOUserLister struct {
	indexer cache.Indexer
}

func NewMinioV1alpha1MinIOUserLister(indexer cache.Indexer) *MinioV1alpha1MinIOUserLister {
	return &MinioV1alpha1MinIOUserLister{indexer: indexer}
}

func (x *MinioV1alpha1MinIOUserLister) List(namespace string, selector labels.Selector) ([]*miniov1alpha1.MinIOUser, error) {
	var ret []*miniov1alpha1.MinIOUser
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*miniov1alpha1.MinIOUser).DeepCopy())
	})
	return ret, err
}

func (x *MinioV1alpha1MinIOUserLister) Get(namespace, name string) (*miniov1alpha1.MinIOUser, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(miniov1alpha1.SchemaGroupVersion.WithResource("miniouser").GroupResource(), name)
	}
	return obj.(*miniov1alpha1.MinIOUser).DeepCopy(), nil
}

type MiniocontrollerV1beta1MinIOInstanceLister struct {
	indexer cache.Indexer
}

func NewMiniocontrollerV1beta1MinIOInstanceLister(indexer cache.Indexer) *MiniocontrollerV1beta1MinIOInstanceLister {
	return &MiniocontrollerV1beta1MinIOInstanceLister{indexer: indexer}
}

func (x *MiniocontrollerV1beta1MinIOInstanceLister) List(namespace string, selector labels.Selector) ([]*v1beta1.MinIOInstance, error) {
	var ret []*v1beta1.MinIOInstance
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.MinIOInstance).DeepCopy())
	})
	return ret, err
}

func (x *MiniocontrollerV1beta1MinIOInstanceLister) Get(namespace, name string) (*v1beta1.MinIOInstance, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(v1beta1.SchemaGroupVersion.WithResource("minioinstance").GroupResource(), name)
	}
	return obj.(*v1beta1.MinIOInstance).DeepCopy(), nil
}

type MiniocontrollerV1beta1MirrorLister struct {
	indexer cache.Indexer
}

func NewMiniocontrollerV1beta1MirrorLister(indexer cache.Indexer) *MiniocontrollerV1beta1MirrorLister {
	return &MiniocontrollerV1beta1MirrorLister{indexer: indexer}
}

func (x *MiniocontrollerV1beta1MirrorLister) List(namespace string, selector labels.Selector) ([]*v1beta1.Mirror, error) {
	var ret []*v1beta1.Mirror
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.Mirror).DeepCopy())
	})
	return ret, err
}

func (x *MiniocontrollerV1beta1MirrorLister) Get(namespace, name string) (*v1beta1.Mirror, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(v1beta1.SchemaGroupVersion.WithResource("mirror").GroupResource(), name)
	}
	return obj.(*v1beta1.Mirror).DeepCopy(), nil
}
