package client

import (
	"context"
	"reflect"
	"sync"
	"time"

	"go.f110.dev/mono/go/pkg/api/consulv1alpha1"
	"go.f110.dev/mono/go/pkg/api/grafanav1alpha1"
	"go.f110.dev/mono/go/pkg/api/harborv1alpha1"
	"go.f110.dev/mono/go/pkg/api/miniov1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

var (
	Scheme         = runtime.NewScheme()
	ParameterCodec = runtime.NewParameterCodec(Scheme)
)

func init() {
	for _, v := range []func(*runtime.Scheme) error{
		consulv1alpha1.AddToScheme,
		grafanav1alpha1.AddToScheme,
		harborv1alpha1.AddToScheme,
		miniov1alpha1.AddToScheme,
	} {
		if err := v(Scheme); err != nil {
			panic(err)
		}
	}
}

type ConsulV1alpha1 struct {
	client *rest.RESTClient
}

func NewConsulV1alpha1Client(c *rest.Config) (*ConsulV1alpha1, error) {
	client, err := rest.RESTClientFor(c)
	if err != nil {
		return nil, err
	}
	return &ConsulV1alpha1{
		client: client,
	}, nil
}

func (c *ConsulV1alpha1) GetConsulBackup(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*consulv1alpha1.ConsulBackup, error) {
	result := &consulv1alpha1.ConsulBackup{}
	err := c.client.Get().
		Namespace(namespace).
		Resource("consulbackups").
		Name(name).
		VersionedParams(&opts, ParameterCodec).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *ConsulV1alpha1) CreateConsulBackup(ctx context.Context, v *consulv1alpha1.ConsulBackup, opts metav1.CreateOptions) (*consulv1alpha1.ConsulBackup, error) {
	result := &consulv1alpha1.ConsulBackup{}
	err := c.client.Post().
		Namespace(v.Namespace).
		Resource("consulbackups").
		VersionedParams(&opts, ParameterCodec).
		Body(v).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *ConsulV1alpha1) UpdateConsulBackup(ctx context.Context, v *consulv1alpha1.ConsulBackup, opts metav1.UpdateOptions) (*consulv1alpha1.ConsulBackup, error) {
	result := &consulv1alpha1.ConsulBackup{}
	err := c.client.Put().
		Namespace(v.Namespace).
		Resource("consulbackups").
		Name(v.Name).
		VersionedParams(&opts, ParameterCodec).
		Body(v).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *ConsulV1alpha1) DeleteConsulBackup(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(namespace).
		Resource("consulbackups").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *ConsulV1alpha1) ListConsulBackup(ctx context.Context, namespace string, opts metav1.ListOptions) (*consulv1alpha1.ConsulBackupList, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result := &consulv1alpha1.ConsulBackupList{}
	err := c.client.Get().
		Namespace(namespace).
		Resource("consulbackups").
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *ConsulV1alpha1) WatchConsulBackup(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(namespace).
		Resource("consulbackups").
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

type GrafanaV1alpha1 struct {
	client *rest.RESTClient
}

func NewGrafanaV1alpha1Client(c *rest.Config) (*GrafanaV1alpha1, error) {
	client, err := rest.RESTClientFor(c)
	if err != nil {
		return nil, err
	}
	return &GrafanaV1alpha1{
		client: client,
	}, nil
}

func (c *GrafanaV1alpha1) GetGrafana(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*grafanav1alpha1.Grafana, error) {
	result := &grafanav1alpha1.Grafana{}
	err := c.client.Get().
		Namespace(namespace).
		Resource("grafanas").
		Name(name).
		VersionedParams(&opts, ParameterCodec).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *GrafanaV1alpha1) CreateGrafana(ctx context.Context, v *grafanav1alpha1.Grafana, opts metav1.CreateOptions) (*grafanav1alpha1.Grafana, error) {
	result := &grafanav1alpha1.Grafana{}
	err := c.client.Post().
		Namespace(v.Namespace).
		Resource("grafanas").
		VersionedParams(&opts, ParameterCodec).
		Body(v).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *GrafanaV1alpha1) UpdateGrafana(ctx context.Context, v *grafanav1alpha1.Grafana, opts metav1.UpdateOptions) (*grafanav1alpha1.Grafana, error) {
	result := &grafanav1alpha1.Grafana{}
	err := c.client.Put().
		Namespace(v.Namespace).
		Resource("grafanas").
		Name(v.Name).
		VersionedParams(&opts, ParameterCodec).
		Body(v).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *GrafanaV1alpha1) DeleteGrafana(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(namespace).
		Resource("grafanas").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *GrafanaV1alpha1) ListGrafana(ctx context.Context, namespace string, opts metav1.ListOptions) (*grafanav1alpha1.GrafanaList, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result := &grafanav1alpha1.GrafanaList{}
	err := c.client.Get().
		Namespace(namespace).
		Resource("grafanas").
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *GrafanaV1alpha1) WatchGrafana(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(namespace).
		Resource("grafanas").
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

func (c *GrafanaV1alpha1) GetGrafanaUser(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*grafanav1alpha1.GrafanaUser, error) {
	result := &grafanav1alpha1.GrafanaUser{}
	err := c.client.Get().
		Namespace(namespace).
		Resource("grafanausers").
		Name(name).
		VersionedParams(&opts, ParameterCodec).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *GrafanaV1alpha1) CreateGrafanaUser(ctx context.Context, v *grafanav1alpha1.GrafanaUser, opts metav1.CreateOptions) (*grafanav1alpha1.GrafanaUser, error) {
	result := &grafanav1alpha1.GrafanaUser{}
	err := c.client.Post().
		Namespace(v.Namespace).
		Resource("grafanausers").
		VersionedParams(&opts, ParameterCodec).
		Body(v).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *GrafanaV1alpha1) UpdateGrafanaUser(ctx context.Context, v *grafanav1alpha1.GrafanaUser, opts metav1.UpdateOptions) (*grafanav1alpha1.GrafanaUser, error) {
	result := &grafanav1alpha1.GrafanaUser{}
	err := c.client.Put().
		Namespace(v.Namespace).
		Resource("grafanausers").
		Name(v.Name).
		VersionedParams(&opts, ParameterCodec).
		Body(v).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *GrafanaV1alpha1) DeleteGrafanaUser(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(namespace).
		Resource("grafanausers").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *GrafanaV1alpha1) ListGrafanaUser(ctx context.Context, namespace string, opts metav1.ListOptions) (*grafanav1alpha1.GrafanaUserList, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result := &grafanav1alpha1.GrafanaUserList{}
	err := c.client.Get().
		Namespace(namespace).
		Resource("grafanausers").
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *GrafanaV1alpha1) WatchGrafanaUser(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(namespace).
		Resource("grafanausers").
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

type HarborV1alpha1 struct {
	client *rest.RESTClient
}

func NewHarborV1alpha1Client(c *rest.Config) (*HarborV1alpha1, error) {
	client, err := rest.RESTClientFor(c)
	if err != nil {
		return nil, err
	}
	return &HarborV1alpha1{
		client: client,
	}, nil
}

func (c *HarborV1alpha1) GetHarborProject(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*harborv1alpha1.HarborProject, error) {
	result := &harborv1alpha1.HarborProject{}
	err := c.client.Get().
		Namespace(namespace).
		Resource("harborprojects").
		Name(name).
		VersionedParams(&opts, ParameterCodec).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *HarborV1alpha1) CreateHarborProject(ctx context.Context, v *harborv1alpha1.HarborProject, opts metav1.CreateOptions) (*harborv1alpha1.HarborProject, error) {
	result := &harborv1alpha1.HarborProject{}
	err := c.client.Post().
		Namespace(v.Namespace).
		Resource("harborprojects").
		VersionedParams(&opts, ParameterCodec).
		Body(v).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *HarborV1alpha1) UpdateHarborProject(ctx context.Context, v *harborv1alpha1.HarborProject, opts metav1.UpdateOptions) (*harborv1alpha1.HarborProject, error) {
	result := &harborv1alpha1.HarborProject{}
	err := c.client.Put().
		Namespace(v.Namespace).
		Resource("harborprojects").
		Name(v.Name).
		VersionedParams(&opts, ParameterCodec).
		Body(v).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *HarborV1alpha1) DeleteHarborProject(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(namespace).
		Resource("harborprojects").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *HarborV1alpha1) ListHarborProject(ctx context.Context, namespace string, opts metav1.ListOptions) (*harborv1alpha1.HarborProjectList, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result := &harborv1alpha1.HarborProjectList{}
	err := c.client.Get().
		Namespace(namespace).
		Resource("harborprojects").
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *HarborV1alpha1) WatchHarborProject(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(namespace).
		Resource("harborprojects").
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

func (c *HarborV1alpha1) GetHarborRobotAccount(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*harborv1alpha1.HarborRobotAccount, error) {
	result := &harborv1alpha1.HarborRobotAccount{}
	err := c.client.Get().
		Namespace(namespace).
		Resource("harborrobotaccounts").
		Name(name).
		VersionedParams(&opts, ParameterCodec).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *HarborV1alpha1) CreateHarborRobotAccount(ctx context.Context, v *harborv1alpha1.HarborRobotAccount, opts metav1.CreateOptions) (*harborv1alpha1.HarborRobotAccount, error) {
	result := &harborv1alpha1.HarborRobotAccount{}
	err := c.client.Post().
		Namespace(v.Namespace).
		Resource("harborrobotaccounts").
		VersionedParams(&opts, ParameterCodec).
		Body(v).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *HarborV1alpha1) UpdateHarborRobotAccount(ctx context.Context, v *harborv1alpha1.HarborRobotAccount, opts metav1.UpdateOptions) (*harborv1alpha1.HarborRobotAccount, error) {
	result := &harborv1alpha1.HarborRobotAccount{}
	err := c.client.Put().
		Namespace(v.Namespace).
		Resource("harborrobotaccounts").
		Name(v.Name).
		VersionedParams(&opts, ParameterCodec).
		Body(v).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *HarborV1alpha1) DeleteHarborRobotAccount(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(namespace).
		Resource("harborrobotaccounts").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *HarborV1alpha1) ListHarborRobotAccount(ctx context.Context, namespace string, opts metav1.ListOptions) (*harborv1alpha1.HarborRobotAccountList, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result := &harborv1alpha1.HarborRobotAccountList{}
	err := c.client.Get().
		Namespace(namespace).
		Resource("harborrobotaccounts").
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *HarborV1alpha1) WatchHarborRobotAccount(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(namespace).
		Resource("harborrobotaccounts").
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

type MinioV1alpha1 struct {
	client *rest.RESTClient
}

func NewMinioV1alpha1Client(c *rest.Config) (*MinioV1alpha1, error) {
	client, err := rest.RESTClientFor(c)
	if err != nil {
		return nil, err
	}
	return &MinioV1alpha1{
		client: client,
	}, nil
}

func (c *MinioV1alpha1) GetMinIOBucket(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*miniov1alpha1.MinIOBucket, error) {
	result := &miniov1alpha1.MinIOBucket{}
	err := c.client.Get().
		Namespace(namespace).
		Resource("miniobuckets").
		Name(name).
		VersionedParams(&opts, ParameterCodec).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *MinioV1alpha1) CreateMinIOBucket(ctx context.Context, v *miniov1alpha1.MinIOBucket, opts metav1.CreateOptions) (*miniov1alpha1.MinIOBucket, error) {
	result := &miniov1alpha1.MinIOBucket{}
	err := c.client.Post().
		Namespace(v.Namespace).
		Resource("miniobuckets").
		VersionedParams(&opts, ParameterCodec).
		Body(v).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *MinioV1alpha1) UpdateMinIOBucket(ctx context.Context, v *miniov1alpha1.MinIOBucket, opts metav1.UpdateOptions) (*miniov1alpha1.MinIOBucket, error) {
	result := &miniov1alpha1.MinIOBucket{}
	err := c.client.Put().
		Namespace(v.Namespace).
		Resource("miniobuckets").
		Name(v.Name).
		VersionedParams(&opts, ParameterCodec).
		Body(v).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *MinioV1alpha1) DeleteMinIOBucket(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(namespace).
		Resource("miniobuckets").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *MinioV1alpha1) ListMinIOBucket(ctx context.Context, namespace string, opts metav1.ListOptions) (*miniov1alpha1.MinIOBucketList, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result := &miniov1alpha1.MinIOBucketList{}
	err := c.client.Get().
		Namespace(namespace).
		Resource("miniobuckets").
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *MinioV1alpha1) WatchMinIOBucket(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(namespace).
		Resource("miniobuckets").
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

func (c *MinioV1alpha1) GetMinIOUser(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*miniov1alpha1.MinIOUser, error) {
	result := &miniov1alpha1.MinIOUser{}
	err := c.client.Get().
		Namespace(namespace).
		Resource("miniousers").
		Name(name).
		VersionedParams(&opts, ParameterCodec).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *MinioV1alpha1) CreateMinIOUser(ctx context.Context, v *miniov1alpha1.MinIOUser, opts metav1.CreateOptions) (*miniov1alpha1.MinIOUser, error) {
	result := &miniov1alpha1.MinIOUser{}
	err := c.client.Post().
		Namespace(v.Namespace).
		Resource("miniousers").
		VersionedParams(&opts, ParameterCodec).
		Body(v).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *MinioV1alpha1) UpdateMinIOUser(ctx context.Context, v *miniov1alpha1.MinIOUser, opts metav1.UpdateOptions) (*miniov1alpha1.MinIOUser, error) {
	result := &miniov1alpha1.MinIOUser{}
	err := c.client.Put().
		Namespace(v.Namespace).
		Resource("miniousers").
		Name(v.Name).
		VersionedParams(&opts, ParameterCodec).
		Body(v).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *MinioV1alpha1) DeleteMinIOUser(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(namespace).
		Resource("miniousers").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *MinioV1alpha1) ListMinIOUser(ctx context.Context, namespace string, opts metav1.ListOptions) (*miniov1alpha1.MinIOUserList, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result := &miniov1alpha1.MinIOUserList{}
	err := c.client.Get().
		Namespace(namespace).
		Resource("miniousers").
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *MinioV1alpha1) WatchMinIOUser(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(namespace).
		Resource("miniousers").
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

var Factory = NewInformerFactory()

type InformerFactory struct {
	mu        sync.Mutex
	informers map[reflect.Type]cache.SharedIndexInformer
	once      sync.Once
	ctx       context.Context
}

func NewInformerFactory() *InformerFactory {
	return &InformerFactory{informers: make(map[reflect.Type]cache.SharedIndexInformer)}
}

func (f *InformerFactory) InformerFor(obj runtime.Object, newFunc func() cache.SharedIndexInformer) cache.SharedIndexInformer {
	f.mu.Lock()
	defer f.mu.Unlock()

	typ := reflect.TypeOf(obj)
	if v, ok := f.informers[typ]; ok {
		return v
	}
	informer := newFunc()
	f.informers[typ] = informer
	if f.ctx != nil {
		go informer.Run(f.ctx.Done())
	}
	return informer
}
func (f *InformerFactory) Run(ctx context.Context) {
	f.mu.Lock()
	f.once.Do(func() {
		for _, v := range f.informers {
			go v.Run(ctx.Done())
		}
		f.ctx = ctx
	})
	f.mu.Unlock()
}

type ConsulV1alpha1Informer struct {
	factory *InformerFactory
	client  *ConsulV1alpha1
}

func NewConsulV1alpha1Informer(f *InformerFactory, client *ConsulV1alpha1) *ConsulV1alpha1Informer {
	return &ConsulV1alpha1Informer{factory: f, client: client}
}

func (f *ConsulV1alpha1Informer) ConsulBackupInformer(namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return f.factory.InformerFor(&consulv1alpha1.ConsulBackup{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return f.client.ListConsulBackup(context.TODO(), namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchConsulBackup(context.TODO(), namespace, metav1.ListOptions{})
				},
			},
			&consulv1alpha1.ConsulBackup{},
			resyncPeriod,
			indexers,
		)
	},
	)
}

type GrafanaV1alpha1Informer struct {
	factory *InformerFactory
	client  *GrafanaV1alpha1
}

func NewGrafanaV1alpha1Informer(f *InformerFactory, client *GrafanaV1alpha1) *GrafanaV1alpha1Informer {
	return &GrafanaV1alpha1Informer{factory: f, client: client}
}

func (f *GrafanaV1alpha1Informer) GrafanaInformer(namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return f.factory.InformerFor(&grafanav1alpha1.Grafana{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return f.client.ListGrafana(context.TODO(), namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchGrafana(context.TODO(), namespace, metav1.ListOptions{})
				},
			},
			&grafanav1alpha1.Grafana{},
			resyncPeriod,
			indexers,
		)
	},
	)
}
func (f *GrafanaV1alpha1Informer) GrafanaUserInformer(namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return f.factory.InformerFor(&grafanav1alpha1.GrafanaUser{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return f.client.ListGrafanaUser(context.TODO(), namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchGrafanaUser(context.TODO(), namespace, metav1.ListOptions{})
				},
			},
			&grafanav1alpha1.GrafanaUser{},
			resyncPeriod,
			indexers,
		)
	},
	)
}

type HarborV1alpha1Informer struct {
	factory *InformerFactory
	client  *HarborV1alpha1
}

func NewHarborV1alpha1Informer(f *InformerFactory, client *HarborV1alpha1) *HarborV1alpha1Informer {
	return &HarborV1alpha1Informer{factory: f, client: client}
}

func (f *HarborV1alpha1Informer) HarborProjectInformer(namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return f.factory.InformerFor(&harborv1alpha1.HarborProject{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return f.client.ListHarborProject(context.TODO(), namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchHarborProject(context.TODO(), namespace, metav1.ListOptions{})
				},
			},
			&harborv1alpha1.HarborProject{},
			resyncPeriod,
			indexers,
		)
	},
	)
}
func (f *HarborV1alpha1Informer) HarborRobotAccountInformer(namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return f.factory.InformerFor(&harborv1alpha1.HarborRobotAccount{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return f.client.ListHarborRobotAccount(context.TODO(), namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchHarborRobotAccount(context.TODO(), namespace, metav1.ListOptions{})
				},
			},
			&harborv1alpha1.HarborRobotAccount{},
			resyncPeriod,
			indexers,
		)
	},
	)
}

type MinioV1alpha1Informer struct {
	factory *InformerFactory
	client  *MinioV1alpha1
}

func NewMinioV1alpha1Informer(f *InformerFactory, client *MinioV1alpha1) *MinioV1alpha1Informer {
	return &MinioV1alpha1Informer{factory: f, client: client}
}

func (f *MinioV1alpha1Informer) MinIOBucketInformer(namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return f.factory.InformerFor(&miniov1alpha1.MinIOBucket{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return f.client.ListMinIOBucket(context.TODO(), namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchMinIOBucket(context.TODO(), namespace, metav1.ListOptions{})
				},
			},
			&miniov1alpha1.MinIOBucket{},
			resyncPeriod,
			indexers,
		)
	},
	)
}
func (f *MinioV1alpha1Informer) MinIOUserInformer(namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return f.factory.InformerFor(&miniov1alpha1.MinIOUser{}, func() cache.SharedIndexInformer {
		return cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return f.client.ListMinIOUser(context.TODO(), namespace, metav1.ListOptions{})
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return f.client.WatchMinIOUser(context.TODO(), namespace, metav1.ListOptions{})
				},
			},
			&miniov1alpha1.MinIOUser{},
			resyncPeriod,
			indexers,
		)
	},
	)
}

type HarborV1alpha1Lister struct {
	indexer cache.Indexer
}

func NewHarborV1alpha1Lister(indexer cache.Indexer) *HarborV1alpha1Lister {
	return &HarborV1alpha1Lister{indexer: indexer}
}

func (x *HarborV1alpha1Lister) ListHarborProject(namespace string, selector labels.Selector) ([]*harborv1alpha1.HarborProject, error) {
	var ret []*harborv1alpha1.HarborProject
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*harborv1alpha1.HarborProject))
	})
	return ret, err
}

func (x *HarborV1alpha1Lister) GetHarborProject(namespace, name string) (*harborv1alpha1.HarborProject, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(harborv1alpha1.SchemaGroupVersion.WithResource("harborproject").GroupResource(), name)
	}
	return obj.(*harborv1alpha1.HarborProject), nil
}

func (x *HarborV1alpha1Lister) ListHarborRobotAccount(namespace string, selector labels.Selector) ([]*harborv1alpha1.HarborRobotAccount, error) {
	var ret []*harborv1alpha1.HarborRobotAccount
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*harborv1alpha1.HarborRobotAccount))
	})
	return ret, err
}

func (x *HarborV1alpha1Lister) GetHarborRobotAccount(namespace, name string) (*harborv1alpha1.HarborRobotAccount, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(harborv1alpha1.SchemaGroupVersion.WithResource("harborrobotaccount").GroupResource(), name)
	}
	return obj.(*harborv1alpha1.HarborRobotAccount), nil
}

type MinioV1alpha1Lister struct {
	indexer cache.Indexer
}

func NewMinioV1alpha1Lister(indexer cache.Indexer) *MinioV1alpha1Lister {
	return &MinioV1alpha1Lister{indexer: indexer}
}

func (x *MinioV1alpha1Lister) ListMinIOBucket(namespace string, selector labels.Selector) ([]*miniov1alpha1.MinIOBucket, error) {
	var ret []*miniov1alpha1.MinIOBucket
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*miniov1alpha1.MinIOBucket))
	})
	return ret, err
}

func (x *MinioV1alpha1Lister) GetMinIOBucket(namespace, name string) (*miniov1alpha1.MinIOBucket, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(miniov1alpha1.SchemaGroupVersion.WithResource("miniobucket").GroupResource(), name)
	}
	return obj.(*miniov1alpha1.MinIOBucket), nil
}

func (x *MinioV1alpha1Lister) ListMinIOUser(namespace string, selector labels.Selector) ([]*miniov1alpha1.MinIOUser, error) {
	var ret []*miniov1alpha1.MinIOUser
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*miniov1alpha1.MinIOUser))
	})
	return ret, err
}

func (x *MinioV1alpha1Lister) GetMinIOUser(namespace, name string) (*miniov1alpha1.MinIOUser, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(miniov1alpha1.SchemaGroupVersion.WithResource("miniouser").GroupResource(), name)
	}
	return obj.(*miniov1alpha1.MinIOUser), nil
}

type ConsulV1alpha1Lister struct {
	indexer cache.Indexer
}

func NewConsulV1alpha1Lister(indexer cache.Indexer) *ConsulV1alpha1Lister {
	return &ConsulV1alpha1Lister{indexer: indexer}
}

func (x *ConsulV1alpha1Lister) ListConsulBackup(namespace string, selector labels.Selector) ([]*consulv1alpha1.ConsulBackup, error) {
	var ret []*consulv1alpha1.ConsulBackup
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*consulv1alpha1.ConsulBackup))
	})
	return ret, err
}

func (x *ConsulV1alpha1Lister) GetConsulBackup(namespace, name string) (*consulv1alpha1.ConsulBackup, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(consulv1alpha1.SchemaGroupVersion.WithResource("consulbackup").GroupResource(), name)
	}
	return obj.(*consulv1alpha1.ConsulBackup), nil
}

type GrafanaV1alpha1Lister struct {
	indexer cache.Indexer
}

func NewGrafanaV1alpha1Lister(indexer cache.Indexer) *GrafanaV1alpha1Lister {
	return &GrafanaV1alpha1Lister{indexer: indexer}
}

func (x *GrafanaV1alpha1Lister) ListGrafana(namespace string, selector labels.Selector) ([]*grafanav1alpha1.Grafana, error) {
	var ret []*grafanav1alpha1.Grafana
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*grafanav1alpha1.Grafana))
	})
	return ret, err
}

func (x *GrafanaV1alpha1Lister) GetGrafana(namespace, name string) (*grafanav1alpha1.Grafana, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(grafanav1alpha1.SchemaGroupVersion.WithResource("grafana").GroupResource(), name)
	}
	return obj.(*grafanav1alpha1.Grafana), nil
}

func (x *GrafanaV1alpha1Lister) ListGrafanaUser(namespace string, selector labels.Selector) ([]*grafanav1alpha1.GrafanaUser, error) {
	var ret []*grafanav1alpha1.GrafanaUser
	err := cache.ListAllByNamespace(x.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*grafanav1alpha1.GrafanaUser))
	})
	return ret, err
}

func (x *GrafanaV1alpha1Lister) GetGrafanaUser(namespace, name string) (*grafanav1alpha1.GrafanaUser, error) {
	obj, exists, err := x.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, k8serrors.NewNotFound(grafanav1alpha1.SchemaGroupVersion.WithResource("grafanauser").GroupResource(), name)
	}
	return obj.(*grafanav1alpha1.GrafanaUser), nil
}
