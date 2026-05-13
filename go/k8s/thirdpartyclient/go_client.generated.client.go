package thirdpartyclient

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"time"

	"go.f110.dev/kubeproto/go/apis/metav1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	v1beta1 "go.f110.dev/mono/go/k8s/thirdpartyapi/minio-operator/miniocontrollerv1beta1"
)

var (
	Scheme         = runtime.NewScheme()
	ParameterCodec = runtime.NewParameterCodec(Scheme)
	Codecs         = serializer.NewCodecFactory(Scheme)
	AddToScheme    = localSchemeBuilder.AddToScheme
)

var localSchemeBuilder = runtime.SchemeBuilder{
	v1beta1.AddToScheme,
}

func init() {
	for _, v := range []func(*runtime.Scheme) error{
		v1beta1.AddToScheme,
	} {
		if err := v(Scheme); err != nil {
			panic(err)
		}
	}
}

type Backend interface {
	Get(ctx context.Context, resourceName, namespace, name string, opts metav1.GetOptions, result runtime.Object) (runtime.Object, error)
	List(ctx context.Context, resourceName, namespace string, opts metav1.ListOptions, result runtime.Object) (runtime.Object, error)
	Create(ctx context.Context, resourceName string, obj runtime.Object, opts metav1.CreateOptions, result runtime.Object) (runtime.Object, error)
	Update(ctx context.Context, resourceName string, obj runtime.Object, opts metav1.UpdateOptions, result runtime.Object) (runtime.Object, error)
	UpdateStatus(ctx context.Context, resourceName string, obj runtime.Object, opts metav1.UpdateOptions, result runtime.Object) (runtime.Object, error)
	Delete(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string, opts metav1.DeleteOptions) error
	Watch(ctx context.Context, gvr schema.GroupVersionResource, namespace string, opts metav1.ListOptions) (watch.Interface, error)
	GetClusterScoped(ctx context.Context, resourceName, name string, opts metav1.GetOptions, result runtime.Object) (runtime.Object, error)
	ListClusterScoped(ctx context.Context, resourceName string, opts metav1.ListOptions, result runtime.Object) (runtime.Object, error)
	CreateClusterScoped(ctx context.Context, resourceName string, obj runtime.Object, opts metav1.CreateOptions, result runtime.Object) (runtime.Object, error)
	UpdateClusterScoped(ctx context.Context, resourceName string, obj runtime.Object, opts metav1.UpdateOptions, result runtime.Object) (runtime.Object, error)
	UpdateStatusClusterScoped(ctx context.Context, resourceName string, obj runtime.Object, opts metav1.UpdateOptions, result runtime.Object) (runtime.Object, error)
	DeleteClusterScoped(ctx context.Context, gvr schema.GroupVersionResource, name string, opts metav1.DeleteOptions) error
	WatchClusterScoped(ctx context.Context, gvr schema.GroupVersionResource, opts metav1.ListOptions) (watch.Interface, error)

	RESTClient() *rest.RESTClient
}

type Set struct {
	MiniocontrollerMinV1beta1 *MiniocontrollerMinV1beta1
}

func NewSet(cfg *rest.Config) (*Set, error) {
	s := &Set{}
	{
		conf := *cfg
		conf.GroupVersion = &v1beta1.SchemaGroupVersion
		conf.APIPath = "/apis"
		conf.NegotiatedSerializer = Codecs.WithoutConversion()
		c, err := rest.RESTClientFor(&conf)
		if err != nil {
			return nil, err
		}
		s.MiniocontrollerMinV1beta1 = NewMiniocontrollerMinV1beta1Client(&restBackend{client: c}, &conf)
	}

	return s, nil
}

type restBackend struct {
	client *rest.RESTClient
}

func (r *restBackend) Get(ctx context.Context, resourceName, namespace, name string, opts metav1.GetOptions, result runtime.Object) (runtime.Object, error) {
	return result, r.client.Get().
		Namespace(namespace).
		Resource(resourceName).
		Name(name).
		VersionedParams(&opts, ParameterCodec).
		Do(ctx).
		Into(result)
}

func (r *restBackend) List(ctx context.Context, resourceName, namespace string, opts metav1.ListOptions, result runtime.Object) (runtime.Object, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds > 0 {
		timeout = time.Duration(opts.TimeoutSeconds) * time.Second
	}
	return result, r.client.Get().
		Namespace(namespace).
		Resource(resourceName).
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
}

func (r *restBackend) Create(ctx context.Context, resourceName string, obj runtime.Object, opts metav1.CreateOptions, result runtime.Object) (runtime.Object, error) {
	m := obj.(metav1.Object)
	if m == nil {
		return nil, errors.New("obj is not implement metav1.Object")
	}
	meta := m.GetObjectMeta()
	return result, r.client.Post().
		Namespace(meta.Namespace).
		Resource(resourceName).
		VersionedParams(&opts, ParameterCodec).
		Body(obj).
		Do(ctx).
		Into(result)
}

func (r *restBackend) Update(ctx context.Context, resourceName string, obj runtime.Object, opts metav1.UpdateOptions, result runtime.Object) (runtime.Object, error) {
	m := obj.(metav1.Object)
	if m == nil {
		return nil, errors.New("obj is not implement metav1.Object")
	}
	meta := m.GetObjectMeta()
	return result, r.client.Put().
		Namespace(meta.Namespace).
		Resource(resourceName).
		Name(meta.Name).
		VersionedParams(&opts, ParameterCodec).
		Body(obj).
		Do(ctx).
		Into(result)
}

func (r *restBackend) UpdateStatus(ctx context.Context, resourceName string, obj runtime.Object, opts metav1.UpdateOptions, result runtime.Object) (runtime.Object, error) {
	m := obj.(metav1.Object)
	if m == nil {
		return nil, errors.New("obj is not implement metav1.Object")
	}
	meta := m.GetObjectMeta()
	return result, r.client.Put().
		Namespace(meta.Namespace).
		Resource(resourceName).
		Name(meta.Name).
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
	if opts.TimeoutSeconds > 0 {
		timeout = time.Duration(opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return r.client.Get().
		Namespace(namespace).
		Resource(gvr.Resource).
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

func (r *restBackend) GetClusterScoped(ctx context.Context, resourceName, name string, opts metav1.GetOptions, result runtime.Object) (runtime.Object, error) {
	return result, r.client.Get().
		Resource(resourceName).
		Name(name).
		VersionedParams(&opts, ParameterCodec).
		Do(ctx).
		Into(result)
}

func (r *restBackend) ListClusterScoped(ctx context.Context, resourceName string, opts metav1.ListOptions, result runtime.Object) (runtime.Object, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds > 0 {
		timeout = time.Duration(opts.TimeoutSeconds) * time.Second
	}
	return result, r.client.Get().
		Resource(resourceName).
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
}

func (r *restBackend) CreateClusterScoped(ctx context.Context, resourceName string, obj runtime.Object, opts metav1.CreateOptions, result runtime.Object) (runtime.Object, error) {
	return result, r.client.Post().
		Resource(resourceName).
		VersionedParams(&opts, ParameterCodec).
		Body(obj).
		Do(ctx).
		Into(result)
}

func (r *restBackend) UpdateClusterScoped(ctx context.Context, resourceName string, obj runtime.Object, opts metav1.UpdateOptions, result runtime.Object) (runtime.Object, error) {
	m := obj.(metav1.Object)
	if m == nil {
		return nil, errors.New("obj is not implement metav1.Object")
	}
	meta := m.GetObjectMeta()
	return result, r.client.Put().
		Resource(resourceName).
		Name(meta.Name).
		VersionedParams(&opts, ParameterCodec).
		Body(obj).
		Do(ctx).
		Into(result)
}

func (r *restBackend) UpdateStatusClusterScoped(ctx context.Context, resourceName string, obj runtime.Object, opts metav1.UpdateOptions, result runtime.Object) (runtime.Object, error) {
	m := obj.(metav1.Object)
	if m == nil {
		return nil, errors.New("obj is not implement metav1.Object")
	}
	meta := m.GetObjectMeta()
	return result, r.client.Put().
		Resource(resourceName).
		Name(meta.Name).
		SubResource("status").
		VersionedParams(&opts, ParameterCodec).
		Body(obj).
		Do(ctx).
		Into(result)
}

func (r *restBackend) DeleteClusterScoped(ctx context.Context, gvr schema.GroupVersionResource, name string, opts metav1.DeleteOptions) error {
	return r.client.Delete().
		Resource(gvr.Resource).
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (r *restBackend) WatchClusterScoped(ctx context.Context, gvr schema.GroupVersionResource, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds > 0 {
		timeout = time.Duration(opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return r.client.Get().
		Resource(gvr.Resource).
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

func (r *restBackend) RESTClient() *rest.RESTClient {
	return r.client
}

type MiniocontrollerMinV1beta1 struct {
	backend Backend
	config  *rest.Config
}

func NewMiniocontrollerMinV1beta1Client(b Backend, config *rest.Config) *MiniocontrollerMinV1beta1 {
	return &MiniocontrollerMinV1beta1{backend: b, config: config}
}

func (c *MiniocontrollerMinV1beta1) GetMinIOInstance(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*v1beta1.MinIOInstance, error) {
	result, err := c.backend.Get(ctx, "minioinstances", namespace, name, opts, &v1beta1.MinIOInstance{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.MinIOInstance), nil
}

func (c *MiniocontrollerMinV1beta1) CreateMinIOInstance(ctx context.Context, v *v1beta1.MinIOInstance, opts metav1.CreateOptions) (*v1beta1.MinIOInstance, error) {
	result, err := c.backend.Create(ctx, "minioinstances", v, opts, &v1beta1.MinIOInstance{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.MinIOInstance), nil
}

func (c *MiniocontrollerMinV1beta1) UpdateMinIOInstance(ctx context.Context, v *v1beta1.MinIOInstance, opts metav1.UpdateOptions) (*v1beta1.MinIOInstance, error) {
	result, err := c.backend.Update(ctx, "minioinstances", v, opts, &v1beta1.MinIOInstance{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.MinIOInstance), nil
}

func (c *MiniocontrollerMinV1beta1) UpdateStatusMinIOInstance(ctx context.Context, v *v1beta1.MinIOInstance, opts metav1.UpdateOptions) (*v1beta1.MinIOInstance, error) {
	result, err := c.backend.UpdateStatus(ctx, "minioinstances", v, opts, &v1beta1.MinIOInstance{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.MinIOInstance), nil
}

func (c *MiniocontrollerMinV1beta1) DeleteMinIOInstance(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.backend.Delete(ctx, schema.GroupVersionResource{Group: "miniocontroller.min.io", Version: "v1beta1", Resource: "minioinstances"}, namespace, name, opts)
}

func (c *MiniocontrollerMinV1beta1) ListMinIOInstance(ctx context.Context, namespace string, opts metav1.ListOptions) (*v1beta1.MinIOInstanceList, error) {
	result, err := c.backend.List(ctx, "minioinstances", namespace, opts, &v1beta1.MinIOInstanceList{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.MinIOInstanceList), nil
}

func (c *MiniocontrollerMinV1beta1) WatchMinIOInstance(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.backend.Watch(ctx, schema.GroupVersionResource{Group: "miniocontroller.min.io", Version: "v1beta1", Resource: "minioinstances"}, namespace, opts)
}

func (c *MiniocontrollerMinV1beta1) GetMirror(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*v1beta1.Mirror, error) {
	result, err := c.backend.Get(ctx, "mirrors", namespace, name, opts, &v1beta1.Mirror{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.Mirror), nil
}

func (c *MiniocontrollerMinV1beta1) CreateMirror(ctx context.Context, v *v1beta1.Mirror, opts metav1.CreateOptions) (*v1beta1.Mirror, error) {
	result, err := c.backend.Create(ctx, "mirrors", v, opts, &v1beta1.Mirror{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.Mirror), nil
}

func (c *MiniocontrollerMinV1beta1) UpdateMirror(ctx context.Context, v *v1beta1.Mirror, opts metav1.UpdateOptions) (*v1beta1.Mirror, error) {
	result, err := c.backend.Update(ctx, "mirrors", v, opts, &v1beta1.Mirror{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.Mirror), nil
}

func (c *MiniocontrollerMinV1beta1) UpdateStatusMirror(ctx context.Context, v *v1beta1.Mirror, opts metav1.UpdateOptions) (*v1beta1.Mirror, error) {
	result, err := c.backend.UpdateStatus(ctx, "mirrors", v, opts, &v1beta1.Mirror{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.Mirror), nil
}

func (c *MiniocontrollerMinV1beta1) DeleteMirror(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.backend.Delete(ctx, schema.GroupVersionResource{Group: "miniocontroller.min.io", Version: "v1beta1", Resource: "mirrors"}, namespace, name, opts)
}

func (c *MiniocontrollerMinV1beta1) ListMirror(ctx context.Context, namespace string, opts metav1.ListOptions) (*v1beta1.MirrorList, error) {
	result, err := c.backend.List(ctx, "mirrors", namespace, opts, &v1beta1.MirrorList{})
	if err != nil {
		return nil, err
	}
	return result.(*v1beta1.MirrorList), nil
}

func (c *MiniocontrollerMinV1beta1) WatchMirror(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
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
	case *v1beta1.MinIOInstance:
		return NewMiniocontrollerMinV1beta1Informer(f.cache, f.set.MiniocontrollerMinV1beta1, f.namespace, f.resyncPeriod).MinIOInstanceInformer()
	case *v1beta1.Mirror:
		return NewMiniocontrollerMinV1beta1Informer(f.cache, f.set.MiniocontrollerMinV1beta1, f.namespace, f.resyncPeriod).MirrorInformer()
	default:
		return nil
	}
}

func (f *InformerFactory) InformerForResource(gvr schema.GroupVersionResource) cache.SharedIndexInformer {
	switch gvr {
	case v1beta1.SchemaGroupVersion.WithResource("minioinstances"):
		return NewMiniocontrollerMinV1beta1Informer(f.cache, f.set.MiniocontrollerMinV1beta1, f.namespace, f.resyncPeriod).MinIOInstanceInformer()
	case v1beta1.SchemaGroupVersion.WithResource("mirrors"):
		return NewMiniocontrollerMinV1beta1Informer(f.cache, f.set.MiniocontrollerMinV1beta1, f.namespace, f.resyncPeriod).MirrorInformer()
	default:
		return nil
	}
}

func (f *InformerFactory) Run(ctx context.Context) {
	for _, v := range f.cache.Informers() {
		go v.Run(ctx.Done())
	}
}

type MiniocontrollerMinV1beta1Informer struct {
	cache        *InformerCache
	client       *MiniocontrollerMinV1beta1
	namespace    string
	resyncPeriod time.Duration
	indexers     cache.Indexers
}

func NewMiniocontrollerMinV1beta1Informer(c *InformerCache, client *MiniocontrollerMinV1beta1, namespace string, resyncPeriod time.Duration) *MiniocontrollerMinV1beta1Informer {
	return &MiniocontrollerMinV1beta1Informer{
		cache:        c,
		client:       client,
		namespace:    namespace,
		resyncPeriod: resyncPeriod,
		indexers:     cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	}
}

func (f *MiniocontrollerMinV1beta1Informer) MinIOInstanceInformer() cache.SharedIndexInformer {
	return f.cache.Write(&v1beta1.MinIOInstance{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options k8smetav1.ListOptions) (runtime.Object, error) {
					return f.client.ListMinIOInstance(context.TODO(), f.namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options k8smetav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchMinIOInstance(context.TODO(), f.namespace, metav1.ListOptions{})
				},
			},
			&v1beta1.MinIOInstance{},
			f.resyncPeriod,
			f.indexers,
		)
	})
}

func (f *MiniocontrollerMinV1beta1Informer) MinIOInstanceLister() *MiniocontrollerMinV1beta1MinIOInstanceLister {
	return NewMiniocontrollerMinV1beta1MinIOInstanceLister(f.MinIOInstanceInformer().GetIndexer())
}

func (f *MiniocontrollerMinV1beta1Informer) MirrorInformer() cache.SharedIndexInformer {
	return f.cache.Write(&v1beta1.Mirror{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options k8smetav1.ListOptions) (runtime.Object, error) {
					return f.client.ListMirror(context.TODO(), f.namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options k8smetav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchMirror(context.TODO(), f.namespace, metav1.ListOptions{})
				},
			},
			&v1beta1.Mirror{},
			f.resyncPeriod,
			f.indexers,
		)
	})
}

func (f *MiniocontrollerMinV1beta1Informer) MirrorLister() *MiniocontrollerMinV1beta1MirrorLister {
	return NewMiniocontrollerMinV1beta1MirrorLister(f.MirrorInformer().GetIndexer())
}

type MiniocontrollerMinV1beta1MinIOInstanceLister struct {
	indexer cache.Indexer
}

func NewMiniocontrollerMinV1beta1MinIOInstanceLister(indexer cache.Indexer) *MiniocontrollerMinV1beta1MinIOInstanceLister {
	return &MiniocontrollerMinV1beta1MinIOInstanceLister{indexer: indexer}
}

func (x *MiniocontrollerMinV1beta1MinIOInstanceLister) List(namespace string, selector labels.Selector) ([]*v1beta1.MinIOInstance, error) {
	var ret []*v1beta1.MinIOInstance
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.MinIOInstance).DeepCopy())
	})
	return ret, err
}

func (x *MiniocontrollerMinV1beta1MinIOInstanceLister) Get(namespace, name string) (*v1beta1.MinIOInstance, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(v1beta1.SchemaGroupVersion.WithResource("minioinstance").GroupResource(), name)
	}
	return obj.(*v1beta1.MinIOInstance).DeepCopy(), nil
}

type MiniocontrollerMinV1beta1MirrorLister struct {
	indexer cache.Indexer
}

func NewMiniocontrollerMinV1beta1MirrorLister(indexer cache.Indexer) *MiniocontrollerMinV1beta1MirrorLister {
	return &MiniocontrollerMinV1beta1MirrorLister{indexer: indexer}
}

func (x *MiniocontrollerMinV1beta1MirrorLister) List(namespace string, selector labels.Selector) ([]*v1beta1.Mirror, error) {
	var ret []*v1beta1.Mirror
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.Mirror).DeepCopy())
	})
	return ret, err
}

func (x *MiniocontrollerMinV1beta1MirrorLister) Get(namespace, name string) (*v1beta1.Mirror, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(v1beta1.SchemaGroupVersion.WithResource("mirror").GroupResource(), name)
	}
	return obj.(*v1beta1.Mirror).DeepCopy(), nil
}
