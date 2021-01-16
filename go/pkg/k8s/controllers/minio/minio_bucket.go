package minio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/minio/minio-go/v6"
	"github.com/minio/minio-go/v6/pkg/policy"
	miniocontrollerv1beta1 "github.com/minio/minio-operator/pkg/apis/miniocontroller/v1beta1"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/klog"

	miniov1alpha1 "go.f110.dev/mono/go/pkg/api/minio/v1alpha1"
	clientset "go.f110.dev/mono/go/pkg/k8s/client/versioned"
	"go.f110.dev/mono/go/pkg/k8s/controllers/controllerutil"
	informers "go.f110.dev/mono/go/pkg/k8s/informers/externalversions"
	mbLister "go.f110.dev/mono/go/pkg/k8s/listers/minio/v1alpha1"
)

// +kubebuilder:rbac:groups=minio.f110.dev,resources=miniobuckets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=minio.f110.dev,resources=miniobuckets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=miniocontroller.min.io,resources=minioinstances,verbs=get;list
// +kubebuilder:rbac:groups=*,resources=pods;secrets;services,verbs=get
// +kubebuilder:rbac:groups=*,resources=pods/portforward,verbs=get;list;create

const (
	minIOBucketControllerFinalizerName = "minio-bucket-controller.minio.f110.dev/finalizer"
)

type BucketController struct {
	*controllerutil.ControllerBase

	config     *rest.Config
	coreClient *kubernetes.Clientset
	mClient    *clientset.Clientset
	mbLister   mbLister.MinIOBucketLister

	queue *controllerutil.WorkQueue

	runOutsideCluster bool
}

var _ controllerutil.Controller = &BucketController{}

func NewBucketController(ctx context.Context, client *kubernetes.Clientset, cfg *rest.Config, runOutsideCluster bool) (*BucketController, error) {
	mClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	_, apiList, err := client.ServerGroupsAndResources()
	if err != nil {
		return nil, err
	}
	found := false
	for _, v := range apiList {
		if v.GroupVersion == "miniocontroller.min.io/v1beta1" {
			for _, v := range v.APIResources {
				if v.Kind == "MinIOInstance" {
					found = true
					break
				}
			}
		}
	}
	if !found {
		return nil, errors.New("minio-operator is not installed")
	}

	sharedInformerFactory := informers.NewSharedInformerFactory(mClient, 30*time.Second)
	mbInformer := sharedInformerFactory.Minio().V1alpha1().MinIOBuckets()

	c := &BucketController{
		config:            cfg,
		coreClient:        client,
		mClient:           mClient,
		mbLister:          mbInformer.Lister(),
		runOutsideCluster: runOutsideCluster,
	}
	c.ControllerBase = controllerutil.NewBase(
		"minio-bucket-operator",
		c,
		client,
		[]cache.SharedIndexInformer{mbInformer.Informer()},
		[]cache.SharedIndexInformer{},
		[]string{minIOBucketControllerFinalizerName},
	)

	sharedInformerFactory.Start(ctx.Done())

	return c, nil
}

func (c *BucketController) ObjectToKeys(obj interface{}) []string {
	bucket, ok := obj.(*miniov1alpha1.MinIOBucket)
	if !ok {
		return nil
	}
	key, err := cache.MetaNamespaceKeyFunc(bucket)
	if err != nil {
		return nil
	}

	return []string{key}
}

func (c *BucketController) GetObject(key string) (runtime.Object, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	bucket, err := c.mbLister.MinIOBuckets(namespace).Get(name)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	return bucket, nil
}

func (c *BucketController) UpdateObject(ctx context.Context, obj runtime.Object) (runtime.Object, error) {
	bucket := obj.(*miniov1alpha1.MinIOBucket)

	bucket, err := c.mClient.MinioV1alpha1().MinIOBuckets(bucket.Namespace).Update(ctx, bucket, metav1.UpdateOptions{})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	return bucket, nil
}

func (c *BucketController) Reconcile(ctx context.Context, obj runtime.Object) error {
	currentBucket := obj.(*miniov1alpha1.MinIOBucket)
	minioBucket := currentBucket.DeepCopy()

	instances, err := c.mClient.MinV1beta1().MinIOInstances(minioBucket.Namespace).List(ctx, metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(&minioBucket.Spec.Selector)})
	if err != nil {
		return err
	}
	if len(instances.Items) == 0 {
		klog.V(4).Infof("%s not found", metav1.FormatLabelSelector(&minioBucket.Spec.Selector))
		return nil
	}
	if len(instances.Items) > 1 {
		return errors.New("found some instances")
	}

	for _, instance := range instances.Items {
		creds, err := c.coreClient.CoreV1().Secrets(instance.Namespace).Get(ctx, instance.Spec.CredsSecret.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		instanceEndpoint, forwarder, err := c.getMinIOInstanceEndpoint(ctx, instance)
		if err != nil {
			return err
		}
		if forwarder != nil {
			defer forwarder.Close()
		}

		mc, err := minio.New(instanceEndpoint, string(creds.Data["accesskey"]), string(creds.Data["secretkey"]), false)
		if err != nil {
			return err
		}
		if err := c.ensureBucket(mc, minioBucket.Name); err != nil {
			return err
		}

		if err := c.ensureBucketPolicy(mc, minioBucket); err != nil {
			return err
		}

		if err := c.ensureIndexFile(mc, minioBucket); err != nil {
			return err
		}
	}

	minioBucket.Status.Ready = true

	if !reflect.DeepEqual(minioBucket.Status, currentBucket.Status) {
		_, err = c.mClient.MinioV1alpha1().MinIOBuckets(minioBucket.Namespace).UpdateStatus(ctx, minioBucket, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *BucketController) ensureBucket(mc *minio.Client, name string) error {
	if exists, err := mc.BucketExists(name); err != nil {
		return err
	} else if exists {
		klog.V(4).Infof("%s already exists", name)
		return nil
	}
	klog.V(4).Infof("%s is created", name)

	if err := mc.MakeBucket(name, ""); err != nil {
		return err
	}

	return nil
}

func (c *BucketController) ensureBucketPolicy(mc *minio.Client, spec *miniov1alpha1.MinIOBucket) error {
	var statements []policy.Statement
	switch spec.Spec.Policy {
	case "", miniov1alpha1.PolicyPrivate:
		// If .Spec.Policy is an empty value, We must not change anything.
		return mc.SetBucketPolicyWithContext(context.TODO(), spec.Name, "")
	case miniov1alpha1.PolicyPublic:
		statements = policy.SetPolicy(nil, policy.BucketPolicyReadWrite, spec.Name, "*")
	case miniov1alpha1.PolicyReadOnly:
		statements = policy.SetPolicy(nil, policy.BucketPolicyReadOnly, spec.Name, "*")
	}

	p := map[string]interface{}{
		"Version":   "2012-10-17",
		"Statement": statements,
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	klog.V(4).Infof("SetBucketPolicy: %s", string(b))
	if err := mc.SetBucketPolicyWithContext(context.TODO(), spec.Name, string(b)); err != nil {
		return err
	}

	return nil
}

func (c *BucketController) ensureIndexFile(mc *minio.Client, spec *miniov1alpha1.MinIOBucket) error {
	if !spec.Spec.CreateIndexFile {
		return nil
	}

	stat, err := mc.StatObjectWithContext(context.TODO(), spec.Name, "index.html", minio.StatObjectOptions{})
	if err != nil {
		mErr, ok := err.(minio.ErrorResponse)
		if !ok {
			return err
		}

		if mErr.Code != "NoSuchKey" {
			return err
		}
	}
	if stat.Key != "" {
		klog.V(4).Info("Skip create index file because file already exists")
		return nil
	}

	klog.V(4).Info("Create index.html")
	_, err = mc.PutObjectWithContext(context.TODO(), spec.Name, "index.html", strings.NewReader(""), 0, minio.PutObjectOptions{})
	return err
}

func (c *BucketController) Finalize(ctx context.Context, obj runtime.Object) error {
	bucket := obj.(*miniov1alpha1.MinIOBucket)
	if bucket.Spec.BucketFinalizePolicy == "" || bucket.Spec.BucketFinalizePolicy == miniov1alpha1.BucketKeep {
		// If Spec.BucketFinalizePolicy is Keep, then we shouldn't delete the bucket.
		// We are going to delete the finalizer only.
		bucket.Finalizers = removeString(bucket.Finalizers, minIOBucketControllerFinalizerName)
		_, err := c.mClient.MinioV1alpha1().MinIOBuckets(bucket.Namespace).Update(ctx, bucket, metav1.UpdateOptions{})
		return err
	}

	instances, err := c.mClient.MinV1beta1().MinIOInstances(bucket.Namespace).List(ctx, metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(&bucket.Spec.Selector)})
	if err != nil {
		return err
	}

	for _, instance := range instances.Items {
		creds, err := c.coreClient.CoreV1().Secrets(instance.Namespace).Get(ctx, instance.Spec.CredsSecret.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		instanceEndpoint, forwarder, err := c.getMinIOInstanceEndpoint(ctx, instance)
		if err != nil {
			return err
		}
		if forwarder != nil {
			defer forwarder.Close()
		}

		mc, err := minio.New(instanceEndpoint, string(creds.Data["accesskey"]), string(creds.Data["secretkey"]), false)
		if err != nil {
			return err
		}

		doneCh := make(chan struct{})
		defer close(doneCh)
		for v := range mc.ListObjectsV2(bucket.Name, "", true, doneCh) {
			if err := mc.RemoveObject(bucket.Name, v.Key); err != nil {
				return err
			}
			klog.Infof("%s/%s is removed", bucket.Name, v.Key)
		}

		if err := mc.RemoveBucket(bucket.Name); err != nil {
			return err
		}
		klog.V(4).Infof("Remove bucket %s", bucket.Name)
	}

	bucket.Finalizers = removeString(bucket.Finalizers, minIOBucketControllerFinalizerName)

	_, err = c.mClient.MinioV1alpha1().MinIOBuckets(bucket.Namespace).Update(ctx, bucket, metav1.UpdateOptions{})
	return err
}

func (c *BucketController) getMinIOInstanceEndpoint(ctx context.Context, instance miniocontrollerv1beta1.MinIOInstance) (string, *portforward.PortForwarder, error) {
	svc, err := c.coreClient.CoreV1().Services(instance.Namespace).Get(ctx, fmt.Sprintf("%s-hl-svc", instance.Name), metav1.GetOptions{})
	if err != nil {
		return "", nil, err
	}

	var forwarder *portforward.PortForwarder
	instanceEndpoint := fmt.Sprintf("%s-hl-svc.%s.svc:%d", instance.Name, instance.Namespace, svc.Spec.Ports[0].Port)
	if c.runOutsideCluster {
		forwarder, err = c.portForward(ctx, svc, int(svc.Spec.Ports[0].Port))
		if err != nil {
			return "", nil, err
		}

		ports, err := forwarder.GetPorts()
		if err != nil {
			return "", nil, err
		}
		instanceEndpoint = fmt.Sprintf("127.0.0.1:%d", ports[0].Local)
	}

	return instanceEndpoint, forwarder, nil
}

func (c *BucketController) portForward(ctx context.Context, svc *corev1.Service, port int) (*portforward.PortForwarder, error) {
	selector := labels.SelectorFromSet(svc.Spec.Selector)
	podList, err := c.coreClient.CoreV1().Pods(svc.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
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
		return nil, errors.New("all pods are not running yet")
	}

	req := c.coreClient.CoreV1().RESTClient().Post().Resource("pods").Namespace(svc.Namespace).Name(pod.Name).SubResource("portforward")
	transport, upgrader, err := spdy.RoundTripperFor(c.config)
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
			switch v := err.(type) {
			case *apierrors.StatusError:
				klog.Info(v)
			}
			klog.Error(err)
		}
	}()

	select {
	case <-readyCh:
	case <-time.After(5 * time.Second):
		return nil, errors.New("timed out")
	}

	return pf, nil
}

func removeString(v []string, s string) []string {
	result := make([]string, 0, len(v))
	for _, item := range v {
		if item == s {
			continue
		}

		result = append(result, item)
	}

	return result
}
