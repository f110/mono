package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/policy"
	"go.f110.dev/kubeproto/go/apis/corev1"
	"go.f110.dev/kubeproto/go/apis/metav1"
	"go.f110.dev/kubeproto/go/k8sclient"
	"go.f110.dev/xerrors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/portforward"

	"go.f110.dev/mono/go/api/miniov1alpha1"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/k8s/client"
	"go.f110.dev/mono/go/k8s/controllers/controllerutil"
	"go.f110.dev/mono/go/k8s/thirdpartyapi/minio-operator/miniocontrollerv1beta1"
	"go.f110.dev/mono/go/k8s/thirdpartyclient"
	"go.f110.dev/mono/go/logger/slogger"
)

const (
	minIOBucketControllerFinalizerName = "minio-bucket-controller.minio.f110.dev/finalizer"
)

type MinIOBucketController struct {
	*controllerutil.ControllerBase

	coreClient     *k8sclient.Set
	mClient        *client.MinioV1alpha1
	secretLister   *k8sclient.CoreV1SecretLister
	serviceLister  *k8sclient.CoreV1ServiceLister
	podLister      *k8sclient.CoreV1PodLister
	mbLister       *client.MinioV1alpha1MinIOBucketLister
	instanceLister *thirdpartyclient.MiniocontrollerMinV1beta1MinIOInstanceLister

	queue *controllerutil.WorkQueue

	transport         http.RoundTripper
	runOutsideCluster bool
}

var _ controllerutil.Controller = &MinIOBucketController{}

func NewMinIOBucketController(
	coreClient *k8sclient.Set,
	k8sClient kubernetes.Interface,
	apiClient *client.Set,
	tpClient *thirdpartyclient.Set,
	coreSharedInformerFactory *k8sclient.InformerFactory,
	factory *client.InformerFactory,
	tpFactory *thirdpartyclient.InformerFactory,
	runOutsideCluster bool,
) (*MinIOBucketController, error) {
	coreInformers := k8sclient.NewCoreV1Informer(coreSharedInformerFactory.Cache(), coreClient.CoreV1, metav1.NamespaceAll, 30*time.Second)
	serviceInformer := coreInformers.ServiceInformer()
	secretInformer := coreInformers.SecretInformer()
	podInformer := coreInformers.PodInformer()

	informers := client.NewMinioV1alpha1Informer(factory.Cache(), apiClient.MinioV1alpha1, metav1.NamespaceAll, 30*time.Second)
	mbInformer := informers.MinIOBucketInformer()
	minioControllerInformers := thirdpartyclient.NewMiniocontrollerMinV1beta1Informer(tpFactory.Cache(), tpClient.MiniocontrollerMinV1beta1, metav1.NamespaceNone, 30*time.Second)
	miInformer := minioControllerInformers.MinIOInstanceInformer()

	c := &MinIOBucketController{
		coreClient:        coreClient,
		mClient:           apiClient.MinioV1alpha1,
		mbLister:          informers.MinIOBucketLister(),
		serviceLister:     coreInformers.ServiceLister(),
		secretLister:      coreInformers.SecretLister(),
		podLister:         coreInformers.PodLister(),
		instanceLister:    minioControllerInformers.MinIOInstanceLister(),
		runOutsideCluster: runOutsideCluster,
	}
	c.ControllerBase = controllerutil.NewBase(
		"minio-bucket-controller",
		c,
		k8sClient,
		[]cache.SharedIndexInformer{mbInformer},
		[]cache.SharedIndexInformer{
			miInformer,
			serviceInformer,
			secretInformer,
			podInformer,
		},
		[]string{minIOBucketControllerFinalizerName},
	)

	return c, nil
}

func (c *MinIOBucketController) ObjectToKeys(obj any) []string {
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

func (c *MinIOBucketController) GetObject(key string) (runtime.Object, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	bucket, err := c.mbLister.Get(namespace, name)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return bucket, nil
}

func (c *MinIOBucketController) UpdateObject(ctx context.Context, obj runtime.Object) (runtime.Object, error) {
	bucket := obj.(*miniov1alpha1.MinIOBucket)

	b, err := c.mClient.UpdateMinIOBucket(ctx, bucket, metav1.UpdateOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	return b, nil
}

func (c *MinIOBucketController) NewReconciler(log *slog.Logger) controllerutil.Reconciler {
	return NewBucketReconciler(
		c.coreClient,
		c.mClient,
		c.serviceLister,
		c.podLister,
		c.secretLister,
		c.instanceLister,
		c.runOutsideCluster,
		c.transport,
		log,
	)
}

func (c *MinIOBucketController) Reconcile(ctx context.Context, obj runtime.Object) error {
	panic("Unreachable")
}

func (c *MinIOBucketController) Finalize(ctx context.Context, obj runtime.Object) error {
	panic("Unreachable")
}

type BucketReconciler struct {
	CoreClient     *k8sclient.Set
	Client         *client.MinioV1alpha1
	secretLister   *k8sclient.CoreV1SecretLister
	serviceLister  *k8sclient.CoreV1ServiceLister
	podLister      *k8sclient.CoreV1PodLister
	instanceLister *thirdpartyclient.MiniocontrollerMinV1beta1MinIOInstanceLister

	ctx               context.Context
	runOutsideCluster bool
	transport         http.RoundTripper
	logger            *slog.Logger

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
	coreClient *k8sclient.Set,
	client *client.MinioV1alpha1,
	serviceLister *k8sclient.CoreV1ServiceLister,
	podLister *k8sclient.CoreV1PodLister,
	secretLister *k8sclient.CoreV1SecretLister,
	instanceLister *thirdpartyclient.MiniocontrollerMinV1beta1MinIOInstanceLister,
	runOutsideCluster bool,
	transport http.RoundTripper,
	log *slog.Logger,
) *BucketReconciler {
	r := &BucketReconciler{
		CoreClient:        coreClient,
		Client:            client,
		instanceLister:    instanceLister,
		serviceLister:     serviceLister,
		podLister:         podLister,
		secretLister:      secretLister,
		transport:         transport,
		runOutsideCluster: runOutsideCluster,
		logger:            log,
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

	if r.Obj.Spec.BucketFinalizePolicy == "" || r.Obj.Spec.BucketFinalizePolicy == miniov1alpha1.BucketFinalizePolicyKeep {
		// If Spec.BucketFinalizePolicy is Keep, then we shouldn't delete the bucket.
		// We are going to delete the finalizer only.
		r.Obj.Finalizers = enumerable.Delete(r.Obj.Finalizers, minIOBucketControllerFinalizerName)
		_, err := r.Client.UpdateMinIOBucket(ctx, r.Obj, metav1.UpdateOptions{})
		return xerrors.WithStack(err)
	}

	sel, err := metav1.LabelSelectorAsSelector(&r.Obj.Spec.Selector)
	if err != nil {
		return xerrors.WithStack(err)
	}
	instances, err := r.instanceLister.List(r.Obj.Namespace, sel)
	if err != nil {
		return xerrors.WithStack(err)
	}

	for _, instance := range instances {
		creds, err := r.secretLister.Get(instance.Namespace, instance.Spec.CredsSecret.Name)
		if err != nil {
			return xerrors.WithStack(err)
		}

		instanceEndpoint, forwarder, err := r.getMinIOInstanceEndpoint(ctx, instance)
		if err != nil {
			return xerrors.WithStack(err)
		}
		if forwarder != nil {
			defer forwarder.Close()
		}

		minioCreds := credentials.NewStaticV4(string(creds.Data["accesskey"]), string(creds.Data["secretkey"]), "")
		mc, err := minio.New(instanceEndpoint, &minio.Options{
			Creds:  minioCreds,
			Secure: false,
		})
		if err != nil {
			return xerrors.WithStack(err)
		}

		doneCh := make(chan struct{})
		defer close(doneCh)
		for v := range mc.ListObjects(ctx, r.Obj.Name, minio.ListObjectsOptions{Recursive: true}) {
			if err := mc.RemoveObject(ctx, r.Obj.Name, v.Key, minio.RemoveObjectOptions{}); err != nil {
				return xerrors.WithStack(err)
			}
			r.logger.Info("Object removed", slog.String("name", r.Obj.Name))
		}

		if err := mc.RemoveBucket(ctx, r.Obj.Name); err != nil {
			return xerrors.WithStack(err)
		}
		r.logger.Debug("Remove bucket", slog.String("name", r.Obj.Name))
	}

	r.Obj.Finalizers = enumerable.Delete(r.Obj.Finalizers, minIOBucketControllerFinalizerName)

	_, err = r.Client.UpdateMinIOBucket(ctx, r.Obj, metav1.UpdateOptions{})
	if err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (r *BucketReconciler) init(_ context.Context) (fsm.State, error) {
	sel, err := metav1.LabelSelectorAsSelector(&r.Obj.Spec.Selector)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	instances, err := r.instanceLister.List(r.Obj.Namespace, sel)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	if len(instances) == 0 {
		r.logger.Debug("MinIO instance is not found", slog.String("selector", metav1.FormatLabelSelector(&r.Obj.Spec.Selector)))
		return fsm.Next(bucketStateUpdateStatus)
	}
	if len(instances) > 1 {
		return fsm.Error(xerrors.New("found some instances"))
	}
	r.MinIOInstances = instances

	creds, err := r.secretLister.Get(instances[0].Namespace, instances[0].Spec.CredsSecret.Name)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	r.Secret = creds

	instanceEndpoint, forwarder, err := r.getMinIOInstanceEndpoint(r.ctx, instances[0])
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	r.PortForwarder = forwarder

	minioCreds := credentials.NewStaticV4(string(creds.Data["accesskey"]), string(creds.Data["secretkey"]), "")
	mc, err := minio.New(instanceEndpoint, &minio.Options{
		Creds:     minioCreds,
		Secure:    false,
		Transport: r.transport,
	})
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	r.MinIOClient = mc

	return fsm.Next(bucketStateEnsureBucket)
}

func (r *BucketReconciler) ensureBucket(_ context.Context) (fsm.State, error) {
	if exists, err := r.MinIOClient.BucketExists(r.ctx, r.Obj.Name); err != nil {
		return fsm.Error(xerrors.WithStack(err))
	} else if exists {
		r.logger.Debug("Already exists", slog.String("name", r.Obj.Name))
		return fsm.Next(bucketStateEnsureBucketPolicy)
	}
	r.logger.Debug("Created", slog.String("name", r.Obj.Name))

	if err := r.MinIOClient.MakeBucket(r.ctx, r.Obj.Name, minio.MakeBucketOptions{}); err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}

	return fsm.Next(bucketStateEnsureBucketPolicy)
}

func (r *BucketReconciler) ensureBucketPolicy(ctx context.Context) (fsm.State, error) {
	current, err := r.MinIOClient.GetBucketPolicy(ctx, r.Obj.Name)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	var currentPolicy *policy.BucketAccessPolicy
	if current != "" {
		cp := &policy.BucketAccessPolicy{}
		if err := json.Unmarshal([]byte(current), cp); err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		currentPolicy = cp
	}

	p := &policy.BucketAccessPolicy{
		Version: "2012-10-17",
	}
	switch r.Obj.Spec.Policy {
	case "", miniov1alpha1.BucketPolicyPrivate:
		// If .Spec.Policy is an empty value, We must not change anything.
		err := r.MinIOClient.SetBucketPolicy(r.ctx, r.Obj.Name, "")
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
	case miniov1alpha1.BucketPolicyPublic:
		p.Statements = policy.SetPolicy(nil, policy.BucketPolicyReadWrite, r.Obj.Name, "*")
	case miniov1alpha1.BucketPolicyReadOnly:
		p.Statements = policy.SetPolicy(nil, policy.BucketPolicyReadOnly, r.Obj.Name, "*")
	}
	if len(p.Statements) > 0 && currentPolicy != nil {
		if reflect.DeepEqual(p.Statements, currentPolicy.Statements) {
			slogger.Log.Debug("Skip set bucket policy because already set same policy")
			return fsm.Next(bucketStateEnsureIndexFile)
		}
	}

	b, err := json.Marshal(p)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	r.logger.Debug("SetBucketPolicy", slog.String("name", r.Obj.Name), slog.String("policy", string(b)))
	if err := r.MinIOClient.SetBucketPolicy(r.ctx, r.Obj.Name, string(b)); err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}

	return fsm.Next(bucketStateEnsureIndexFile)
}

func (r *BucketReconciler) ensureIndexFile(_ context.Context) (fsm.State, error) {
	if !r.Obj.Spec.CreateIndexFile {
		return fsm.Next(bucketStateUpdateStatus)
	}

	stat, err := r.MinIOClient.StatObject(r.ctx, r.Obj.Name, "index.html", minio.StatObjectOptions{})
	if err != nil {
		mErr, ok := err.(minio.ErrorResponse)
		if !ok {
			return fsm.Error(xerrors.WithStack(err))
		}

		if mErr.Code != "NoSuchKey" {
			return fsm.Error(xerrors.WithStack(err))
		}
	}
	if stat.Key != "" {
		r.logger.Debug("Skip create index file", slog.String("name", r.Obj.Name))
		return fsm.Next(bucketStateUpdateStatus)
	}

	r.logger.Debug("Create index.html", slog.String("name", r.Obj.Name))
	_, err = r.MinIOClient.PutObject(
		r.ctx,
		r.Obj.Name,
		"index.html",
		strings.NewReader(""),
		0,
		minio.PutObjectOptions{},
	)
	if err != nil {
		return fsm.Error(xerrors.WithStack(err))
	}
	return fsm.Next(bucketStateUpdateStatus)
}

func (r *BucketReconciler) updateStatus(_ context.Context) (fsm.State, error) {
	r.Obj.Status.Ready = true

	if !reflect.DeepEqual(r.Original.Status, r.Obj.Status) {
		_, err := r.Client.UpdateStatusMinIOBucket(r.ctx, r.Obj, metav1.UpdateOptions{})
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
	}

	return fsm.Next(bucketStateCleanup)
}

func (r *BucketReconciler) cleanup(_ context.Context) (fsm.State, error) {
	if r.PortForwarder != nil {
		r.PortForwarder.Close()
	}

	return fsm.Finish()
}

func (r *BucketReconciler) getMinIOInstanceEndpoint(
	ctx context.Context,
	instance *miniocontrollerv1beta1.MinIOInstance,
) (string, *portforward.PortForwarder, error) {
	svc, err := r.serviceLister.Get(instance.Namespace, fmt.Sprintf("%s-hl-svc", instance.Name))
	if err != nil {
		return "", nil, xerrors.WithStack(err)
	}

	var forwarder *portforward.PortForwarder
	instanceEndpoint := fmt.Sprintf("%s-hl-svc.%s.svc:%d", instance.Name, instance.Namespace, svc.Spec.Ports[0].Port)
	if r.runOutsideCluster {
		forwarder, err = r.portForward(ctx, svc, int(svc.Spec.Ports[0].Port))
		if err != nil {
			return "", nil, xerrors.WithStack(err)
		}

		ports, err := forwarder.GetPorts()
		if err != nil {
			return "", nil, xerrors.WithStack(err)
		}
		instanceEndpoint = fmt.Sprintf("127.0.0.1:%d", ports[0].Local)
	}

	return instanceEndpoint, forwarder, nil
}

func (r *BucketReconciler) portForward(ctx context.Context, svc *corev1.Service, port int) (*portforward.PortForwarder, error) {
	selector := labels.SelectorFromSet(svc.Spec.Selector)
	podList, err := r.podLister.List(svc.Namespace, selector)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	var pod *corev1.Pod
	for _, v := range podList {
		if v.Status.Phase == corev1.PodPhaseRunning {
			pod = v
			break
		}
	}
	if pod == nil {
		return nil, xerrors.New("all pods are not running yet")
	}

	pf, _, err := r.CoreClient.CoreV1.PortForward(ctx, pod, port)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return pf, nil
}
