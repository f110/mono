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
	"go.f110.dev/mono/go/pkg/fsm"
	"go.f110.dev/mono/go/pkg/logger"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
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
	mclisters "go.f110.dev/mono/go/pkg/k8s/listers/miniocontroller/v1beta1"
)

// +kubebuilder:rbac:groups=minio.f110.dev,resources=miniobuckets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=minio.f110.dev,resources=miniobuckets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=miniocontroller.min.io,resources=minioinstances,verbs=get;list;watch
// +kubebuilder:rbac:groups=*,resources=pods;secrets;services,verbs=get;list;watch
// +kubebuilder:rbac:groups=*,resources=pods/portforward,verbs=get;list;create

const (
	minIOBucketControllerFinalizerName = "minio-bucket-controller.minio.f110.dev/finalizer"
)

type BucketController struct {
	*controllerutil.ControllerBase

	config         *rest.Config
	coreClient     kubernetes.Interface
	mClient        clientset.Interface
	secretLister   corev1listers.SecretLister
	serviceLister  corev1listers.ServiceLister
	podLister      corev1listers.PodLister
	mbLister       mbLister.MinIOBucketLister
	instanceLister mclisters.MinIOInstanceLister

	queue *controllerutil.WorkQueue

	transport         http.RoundTripper
	runOutsideCluster bool
}

var _ controllerutil.Controller = &BucketController{}

func NewBucketController(
	coreClient kubernetes.Interface,
	client clientset.Interface,
	cfg *rest.Config,
	coreSharedInformerFactory kubeinformers.SharedInformerFactory,
	sharedInformerFactory informers.SharedInformerFactory,
	runOutsideCluster bool,
) (*BucketController, error) {
	serviceInformer := coreSharedInformerFactory.Core().V1().Services()
	secretInformer := coreSharedInformerFactory.Core().V1().Secrets()
	podInformer := coreSharedInformerFactory.Core().V1().Pods()
	mbInformer := sharedInformerFactory.Minio().V1alpha1().MinIOBuckets()
	miInformer := sharedInformerFactory.Miniocontroller().V1beta1().MinIOInstances()

	c := &BucketController{
		config:            cfg,
		coreClient:        coreClient,
		mClient:           client,
		mbLister:          mbInformer.Lister(),
		serviceLister:     serviceInformer.Lister(),
		secretLister:      secretInformer.Lister(),
		podLister:         podInformer.Lister(),
		instanceLister:    miInformer.Lister(),
		runOutsideCluster: runOutsideCluster,
	}
	c.ControllerBase = controllerutil.NewBase(
		"minio-bucket-operator",
		c,
		coreClient,
		[]cache.SharedIndexInformer{mbInformer.Informer()},
		[]cache.SharedIndexInformer{
			miInformer.Informer(),
			serviceInformer.Informer(),
			secretInformer.Informer(),
			podInformer.Informer(),
		},
		[]string{minIOBucketControllerFinalizerName},
	)

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

	b, err := c.mClient.MinioV1alpha1().MinIOBuckets(bucket.Namespace).Update(ctx, bucket, metav1.UpdateOptions{})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return b, nil
}

func (c *BucketController) NewReconciler() controllerutil.Reconciler {
	return NewBucketReconciler(
		c.coreClient,
		c.mClient,
		c.config,
		c.serviceLister,
		c.podLister,
		c.secretLister,
		c.instanceLister,
		c.runOutsideCluster,
		c.transport,
	)
}

func (c *BucketController) Reconcile(ctx context.Context, obj runtime.Object) error {
	panic("Unreachable")
}

func (c *BucketController) Finalize(ctx context.Context, obj runtime.Object) error {
	panic("Unreachable")
}

type BucketReconciler struct {
	CoreClient     kubernetes.Interface
	Client         clientset.Interface
	secretLister   corev1listers.SecretLister
	serviceLister  corev1listers.ServiceLister
	podLister      corev1listers.PodLister
	instanceLister mclisters.MinIOInstanceLister

	ctx               context.Context
	runOutsideCluster bool
	config            *rest.Config
	transport         http.RoundTripper

	Original *miniov1alpha1.MinIOBucket
	Obj      *miniov1alpha1.MinIOBucket

	MinIOInstances []*miniocontrollerv1beta1.MinIOInstance
	Secret         *corev1.Secret
	MinIOClient    *minio.Client
	PortForwarder  *portforward.PortForwarder
}

const (
	bucketStateInit fsm.State = iota
	bucketStateEnsureBucket
	bucketStateEnsureBucketPolicy
	bucketStateEnsureIndexFile
	bucketStateUpdateStatus
	bucketStateCleanup
)

func NewBucketReconciler(
	coreClient kubernetes.Interface,
	client clientset.Interface,
	config *rest.Config,
	serviceLister corev1listers.ServiceLister,
	podLister corev1listers.PodLister,
	secretLister corev1listers.SecretLister,
	instanceLister mclisters.MinIOInstanceLister,
	runOutsideCluster bool,
	transport http.RoundTripper,
) *BucketReconciler {
	r := &BucketReconciler{
		CoreClient:        coreClient,
		Client:            client,
		config:            config,
		instanceLister:    instanceLister,
		serviceLister:     serviceLister,
		podLister:         podLister,
		secretLister:      secretLister,
		transport:         transport,
		runOutsideCluster: runOutsideCluster,
	}
	return r
}

func (r *BucketReconciler) Reconcile(ctx context.Context, obj runtime.Object) error {
	bucket := obj.(*miniov1alpha1.MinIOBucket)
	r.Original = bucket
	r.Obj = bucket.DeepCopy()
	r.ctx = ctx

	f := fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			bucketStateInit:               r.init,
			bucketStateEnsureBucket:       r.ensureBucket,
			bucketStateEnsureBucketPolicy: r.ensureBucketPolicy,
			bucketStateEnsureIndexFile:    r.ensureIndexFile,
			bucketStateUpdateStatus:       r.updateStatus,
			bucketStateCleanup:            r.cleanup,
		},
		bucketStateInit,
		bucketStateUpdateStatus,
	)

	return f.Loop()
}

func (r *BucketReconciler) Finalize(ctx context.Context, obj runtime.Object) error {
	bucket := obj.(*miniov1alpha1.MinIOBucket)
	r.Original = bucket
	r.Obj = bucket.DeepCopy()

	if r.Obj.Spec.BucketFinalizePolicy == "" || r.Obj.Spec.BucketFinalizePolicy == miniov1alpha1.BucketKeep {
		// If Spec.BucketFinalizePolicy is Keep, then we shouldn't delete the bucket.
		// We are going to delete the finalizer only.
		r.Obj.Finalizers = removeString(r.Obj.Finalizers, minIOBucketControllerFinalizerName)
		_, err := r.Client.MinioV1alpha1().MinIOBuckets(r.Obj.Namespace).Update(ctx, r.Obj, metav1.UpdateOptions{})
		return xerrors.Errorf(": %w", err)
	}

	sel, err := metav1.LabelSelectorAsSelector(&r.Obj.Spec.Selector)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	instances, err := r.instanceLister.MinIOInstances(r.Obj.Namespace).List(sel)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	for _, instance := range instances {
		creds, err := r.secretLister.Secrets(instance.Namespace).Get(instance.Spec.CredsSecret.Name)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		instanceEndpoint, forwarder, err := r.getMinIOInstanceEndpoint(ctx, instance)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		if forwarder != nil {
			defer forwarder.Close()
		}

		mc, err := minio.New(instanceEndpoint, string(creds.Data["accesskey"]), string(creds.Data["secretkey"]), false)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		doneCh := make(chan struct{})
		defer close(doneCh)
		for v := range mc.ListObjectsV2(r.Obj.Name, "", true, doneCh) {
			if err := mc.RemoveObject(r.Obj.Name, v.Key); err != nil {
				return xerrors.Errorf(": %w", err)
			}
			klog.Infof("%s/%s is removed", r.Obj.Name, v.Key)
		}

		if err := mc.RemoveBucket(r.Obj.Name); err != nil {
			return xerrors.Errorf(": %w", err)
		}
		klog.V(4).Infof("Remove bucket %s", r.Obj.Name)
	}

	r.Obj.Finalizers = removeString(r.Obj.Finalizers, minIOBucketControllerFinalizerName)

	_, err = r.Client.MinioV1alpha1().MinIOBuckets(r.Obj.Namespace).Update(ctx, r.Obj, metav1.UpdateOptions{})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	return nil
}

func (r *BucketReconciler) init() (fsm.State, error) {
	sel, err := metav1.LabelSelectorAsSelector(&r.Obj.Spec.Selector)
	if err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}
	instances, err := r.instanceLister.MinIOInstances(r.Obj.Namespace).List(sel)
	if err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}
	if len(instances) == 0 {
		klog.V(4).Infof("%s not found", metav1.FormatLabelSelector(&r.Obj.Spec.Selector))
		return bucketStateUpdateStatus, nil
	}
	if len(instances) > 1 {
		return fsm.Error(errors.New("found some instances"))
	}
	r.MinIOInstances = instances

	creds, err := r.secretLister.Secrets(instances[0].Namespace).Get(instances[0].Spec.CredsSecret.Name)
	if err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}
	r.Secret = creds

	instanceEndpoint, forwarder, err := r.getMinIOInstanceEndpoint(r.ctx, instances[0])
	if err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}
	r.PortForwarder = forwarder

	mc, err := minio.New(instanceEndpoint, string(creds.Data["accesskey"]), string(creds.Data["secretkey"]), false)
	if err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}
	if r.transport != nil {
		mc.SetCustomTransport(r.transport)
	}
	r.MinIOClient = mc

	return bucketStateEnsureBucket, nil
}

func (r *BucketReconciler) ensureBucket() (fsm.State, error) {
	if exists, err := r.MinIOClient.BucketExists(r.Obj.Name); err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	} else if exists {
		klog.V(4).Infof("%s already exists", r.Obj.Name)
		return bucketStateEnsureBucketPolicy, nil
	}
	klog.V(4).Infof("%s is created", r.Obj.Name)

	if err := r.MinIOClient.MakeBucket(r.Obj.Name, ""); err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}

	return bucketStateEnsureBucketPolicy, nil
}

func (r *BucketReconciler) ensureBucketPolicy() (fsm.State, error) {
	current, err := r.MinIOClient.GetBucketPolicy(r.Obj.Name)
	if err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}
	var currentPolicy *policy.BucketAccessPolicy
	if current != "" {
		cp := &policy.BucketAccessPolicy{}
		if err := json.Unmarshal([]byte(current), cp); err != nil {
			return fsm.Error(xerrors.Errorf(": %w", err))
		}
		currentPolicy = cp
	}

	p := &policy.BucketAccessPolicy{
		Version: "2012-10-17",
	}
	switch r.Obj.Spec.Policy {
	case "", miniov1alpha1.PolicyPrivate:
		// If .Spec.Policy is an empty value, We must not change anything.
		err := r.MinIOClient.SetBucketPolicyWithContext(r.ctx, r.Obj.Name, "")
		if err != nil {
			return fsm.Error(xerrors.Errorf(": %w", err))
		}
	case miniov1alpha1.PolicyPublic:
		p.Statements = policy.SetPolicy(nil, policy.BucketPolicyReadWrite, r.Obj.Name, "*")
	case miniov1alpha1.PolicyReadOnly:
		p.Statements = policy.SetPolicy(nil, policy.BucketPolicyReadOnly, r.Obj.Name, "*")
	}
	if len(p.Statements) > 0 && currentPolicy != nil {
		if reflect.DeepEqual(p.Statements, currentPolicy.Statements) {
			logger.Log.Debug("Skip set bucket policy because already set same policy")
			return bucketStateEnsureIndexFile, nil
		}
	}

	b, err := json.Marshal(p)
	if err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}
	klog.V(4).Infof("SetBucketPolicy %s: %s", r.Obj.Name, string(b))
	if err := r.MinIOClient.SetBucketPolicyWithContext(r.ctx, r.Obj.Name, string(b)); err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}

	return bucketStateEnsureIndexFile, nil
}

func (r *BucketReconciler) ensureIndexFile() (fsm.State, error) {
	if !r.Obj.Spec.CreateIndexFile {
		return bucketStateUpdateStatus, nil
	}

	stat, err := r.MinIOClient.StatObjectWithContext(r.ctx, r.Obj.Name, "index.html", minio.StatObjectOptions{})
	if err != nil {
		mErr, ok := err.(minio.ErrorResponse)
		if !ok {
			return fsm.Error(xerrors.Errorf(": %w", err))
		}

		if mErr.Code != "NoSuchKey" {
			return fsm.Error(xerrors.Errorf(": %w", err))
		}
	}
	if stat.Key != "" {
		klog.V(4).Info("Skip create index file because file already exists")
		return bucketStateUpdateStatus, nil
	}

	klog.V(4).Info("Create index.html")
	_, err = r.MinIOClient.PutObjectWithContext(
		r.ctx,
		r.Obj.Name,
		"index.html",
		strings.NewReader(""),
		0,
		minio.PutObjectOptions{},
	)
	if err != nil {
		return fsm.Error(xerrors.Errorf(": %w", err))
	}
	return bucketStateUpdateStatus, nil
}

func (r *BucketReconciler) updateStatus() (fsm.State, error) {
	r.Obj.Status.Ready = true

	if !reflect.DeepEqual(r.Original.Status, r.Obj.Status) {
		_, err := r.Client.MinioV1alpha1().MinIOBuckets(r.Obj.Namespace).UpdateStatus(
			r.ctx,
			r.Obj,
			metav1.UpdateOptions{},
		)
		if err != nil {
			return fsm.Error(xerrors.Errorf(": %w", err))
		}
	}

	return bucketStateCleanup, nil
}

func (r *BucketReconciler) cleanup() (fsm.State, error) {
	if r.PortForwarder != nil {
		r.PortForwarder.Close()
	}

	return fsm.Finish()
}

func (r *BucketReconciler) getMinIOInstanceEndpoint(
	ctx context.Context,
	instance *miniocontrollerv1beta1.MinIOInstance,
) (string, *portforward.PortForwarder, error) {
	svc, err := r.serviceLister.Services(instance.Namespace).Get(fmt.Sprintf("%s-hl-svc", instance.Name))
	if err != nil {
		return "", nil, xerrors.Errorf(": %w", err)
	}

	var forwarder *portforward.PortForwarder
	instanceEndpoint := fmt.Sprintf("%s-hl-svc.%s.svc:%d", instance.Name, instance.Namespace, svc.Spec.Ports[0].Port)
	if r.runOutsideCluster {
		forwarder, err = r.portForward(ctx, svc, int(svc.Spec.Ports[0].Port))
		if err != nil {
			return "", nil, xerrors.Errorf(": %w", err)
		}

		ports, err := forwarder.GetPorts()
		if err != nil {
			return "", nil, xerrors.Errorf(": %w", err)
		}
		instanceEndpoint = fmt.Sprintf("127.0.0.1:%d", ports[0].Local)
	}

	return instanceEndpoint, forwarder, nil
}

func (r *BucketReconciler) portForward(
	ctx context.Context,
	svc *corev1.Service,
	port int,
) (*portforward.PortForwarder, error) {
	selector := labels.SelectorFromSet(svc.Spec.Selector)
	podList, err := r.podLister.Pods(svc.Namespace).List(selector)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	var pod *corev1.Pod
	for _, v := range podList {
		if v.Status.Phase == corev1.PodRunning {
			pod = v
			break
		}
	}
	if pod == nil {
		return nil, errors.New("all pods are not running yet")
	}

	req := r.CoreClient.CoreV1().RESTClient().Post().Resource("pods").Namespace(svc.Namespace).Name(pod.Name).SubResource("portforward")
	transport, upgrader, err := spdy.RoundTripperFor(r.config)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req.URL())

	readyCh := make(chan struct{})
	pf, err := portforward.New(
		dialer,
		[]string{fmt.Sprintf(":%d", port)},
		context.Background().Done(),
		readyCh,
		nil,
		nil,
	)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
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
