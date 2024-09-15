package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v6"
	"github.com/minio/minio-go/v6/pkg/policy"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	"go.f110.dev/mono/go/api/miniov1alpha1"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/k8s/client"
	"go.f110.dev/mono/go/k8s/controllers/controllerutil"
	"go.f110.dev/mono/go/k8s/k8sfactory"
	"go.f110.dev/mono/go/k8s/portforward"
	"go.f110.dev/mono/go/stringsutil"
)

const (
	minIOClusterControllerFinalizerName = "minio-cluster-controller.minio.f110.dev/finalizer"
	defaultMinIOClusterAdminUser        = "root"
)

type MinIOClusterController struct {
	*controllerutil.GenericControllerBase[*miniov1alpha1.MinIOCluster]

	config        *rest.Config
	coreClient    kubernetes.Interface
	mClient       *client.MinioV1alpha1
	podLister     corev1listers.PodLister
	pvcLister     corev1listers.PersistentVolumeClaimLister
	serviceLister corev1listers.ServiceLister
	secretLister  corev1listers.SecretLister

	runOutsideCluster bool
}

func NewMinIOClusterController(
	coreClient kubernetes.Interface,
	apiClient *client.Set,
	cfg *rest.Config,
	coreSharedInformerFactory kubeinformers.SharedInformerFactory,
	factory *client.InformerFactory,
	runOutsideCluster bool,
) *MinIOClusterController {
	serviceInformer := coreSharedInformerFactory.Core().V1().Services()
	podInformer := coreSharedInformerFactory.Core().V1().Pods()
	pvcInformer := coreSharedInformerFactory.Core().V1().PersistentVolumeClaims()
	secretInformer := coreSharedInformerFactory.Core().V1().Secrets()
	informers := client.NewMinioV1alpha1Informer(factory.Cache(), apiClient.MinioV1alpha1, metav1.NamespaceAll, 30*time.Second)
	mcInformer := informers.MinIOClusterInformer()
	mcLister := informers.MinIOClusterLister()

	c := &MinIOClusterController{
		runOutsideCluster: runOutsideCluster,
		config:            cfg,
		coreClient:        coreClient,
		mClient:           apiClient.MinioV1alpha1,
		podLister:         podInformer.Lister(),
		pvcLister:         pvcInformer.Lister(),
		serviceLister:     serviceInformer.Lister(),
		secretLister:      secretInformer.Lister(),
	}
	c.GenericControllerBase = controllerutil.NewGenericControllerBase[*miniov1alpha1.MinIOCluster](
		"minio-cluster-controller",
		c.newReconciler,
		coreClient,
		[]cache.SharedIndexInformer{mcInformer},
		[]cache.SharedIndexInformer{serviceInformer.Informer(), podInformer.Informer(), secretInformer.Informer(), pvcInformer.Informer()},
		[]string{minIOClusterControllerFinalizerName},
		mcLister.Get,
		apiClient.MinioV1alpha1.UpdateMinIOCluster,
	)

	return c
}

func (c *MinIOClusterController) newReconciler() controllerutil.GenericReconciler[*miniov1alpha1.MinIOCluster] {
	return &minIOClusterReconciler{
		config:            c.config,
		coreClient:        c.coreClient,
		mClient:           c.mClient,
		podLister:         c.podLister,
		pvcLister:         c.pvcLister,
		serviceLister:     c.serviceLister,
		secretLister:      c.secretLister,
		runOutsideCluster: c.runOutsideCluster,
		logger:            c.Log(),
		recorder:          c.EventRecorder(),
	}
}

type minIOClusterReconciler struct {
	config        *rest.Config
	coreClient    kubernetes.Interface
	mClient       *client.MinioV1alpha1
	podLister     corev1listers.PodLister
	pvcLister     corev1listers.PersistentVolumeClaimLister
	serviceLister corev1listers.ServiceLister
	secretLister  corev1listers.SecretLister

	logger            *zap.Logger
	recorder          record.EventRecorder
	runOutsideCluster bool

	changed bool
}

var _ controllerutil.GenericReconciler[*miniov1alpha1.MinIOCluster] = (*minIOClusterReconciler)(nil)

func (m *minIOClusterReconciler) Reconcile(ctx context.Context, obj *miniov1alpha1.MinIOCluster) error {
	m.logger.Debug("Start reconciling MinIOCluster")
	if m.logger.Level() == zapcore.DebugLevel {
		defer m.logger.Debug("Finished reconciling MinIOCluster")
	}
	rCtx, err := m.newContext(obj)
	if err != nil {
		return err
	}

	for i := range rCtx.Obj.Spec.Nodes {
		var existPVC *corev1.PersistentVolumeClaim
		for _, v := range rCtx.pvc {
			if v.Name == fmt.Sprintf("%s-data-%d", rCtx.Obj.Name, i+1) {
				existPVC = v
				break
			}
		}
		if existPVC == nil {
			pvc := m.pvc(rCtx.Obj, i+1)
			m.changed = true
			if _, err := m.coreClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(ctx, pvc, metav1.CreateOptions{}); err != nil {
				return controllerutil.WrapRetryError(xerrors.WithStack(err))
			}
		}

		var existPod *corev1.Pod
		for _, v := range rCtx.pods {
			if v.Name == fmt.Sprintf("%s-%d", rCtx.Obj.Name, i+1) {
				existPod = v
				break
			}
		}
		pod := m.pod(rCtx.Obj, i+1)
		if existPod == nil {
			m.changed = true
			if _, err := m.coreClient.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
				return controllerutil.WrapRetryError(xerrors.WithStack(err))
			}
		} else if reflect.DeepEqual(existPod.Spec, pod.Spec) {
			m.changed = true
			if err := m.coreClient.CoreV1().Pods(existPod.Namespace).Delete(ctx, existPod.Name, metav1.DeleteOptions{}); err != nil {
				return controllerutil.WrapRetryError(xerrors.WithStack(err))
			}
			if _, err := m.coreClient.CoreV1().Pods(existPod.Namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
				return controllerutil.WrapRetryError(xerrors.WithStack(err))
			}
		}
	}

	svcs := m.services(rCtx.Obj)
	existSvc := make(map[string]*corev1.Service)
	for _, v := range rCtx.svcs {
		existSvc[v.Name] = v
	}
	for _, svc := range svcs {
		if oldSvc, ok := existSvc[svc.Name]; !ok {
			m.changed = true
			if _, err := m.coreClient.CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{}); err != nil {
				return controllerutil.WrapRetryError(xerrors.WithStack(err))
			}
		} else if !reflect.DeepEqual(svc, oldSvc) {
			m.changed = true
			if _, err := m.coreClient.CoreV1().Services(svc.Namespace).Update(ctx, svc, metav1.UpdateOptions{}); err != nil {
				return controllerutil.WrapRetryError(xerrors.WithStack(err))
			}
		}
	}

	if rCtx.secret == nil {
		secret := m.secret(rCtx.Obj)
		m.changed = true
		if _, err := m.coreClient.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
			return controllerutil.WrapRetryError(xerrors.WithStack(err))
		}
	}

	if m.changed {
		if err := m.reloadContext(rCtx); err != nil {
			return err
		}
	}
	if rCtx.Obj.Spec.Nodes == len(rCtx.pods) {
		ready := true
		for _, pod := range rCtx.pods {
			if pod.Status.Phase != corev1.PodRunning {
				ready = false
			}

			idx := enumerable.Index(pod.Status.Conditions, func(cond corev1.PodCondition) bool { return cond.Type == corev1.PodReady })
			if idx != -1 {
				if pod.Status.Conditions[idx].Status != corev1.ConditionTrue {
					ready = false
				}
			} else {
				ready = false
			}
		}
		rCtx.Obj.Status.Ready = ready
	}
	rCtx.Obj.Status.Phase = rCtx.CurrentPhase()
	if rCtx.StatusChanged() {
		m.logger.Debug("Update MinIOCluster status", zap.String("name", rCtx.Obj.Name), zap.String("namespace", rCtx.Obj.Namespace), zap.Any("status", rCtx.Obj.Status))
		if _, err := m.mClient.UpdateStatusMinIOCluster(ctx, rCtx.Obj, metav1.UpdateOptions{}); err != nil {
			return controllerutil.WrapRetryError(xerrors.WithStack(err))
		}
	}

	if rCtx.Obj.Status.Phase == miniov1alpha1.ClusterPhaseRunning {
		instanceEndpoint := fmt.Sprintf("%s.%s.svc:9000", obj.Name, obj.Namespace)
		if m.runOutsideCluster {
			forwarder, port, err := portforward.PortForward(ctx, rCtx.svcs[0], 9000, m.config, m.coreClient, m.podLister)
			if err != nil {
				return xerrors.WithStack(err)
			}
			defer forwarder.Close()
			instanceEndpoint = fmt.Sprintf("127.0.0.1:%d", port)
		}

		mc, err := minio.New(instanceEndpoint, defaultMinIOClusterAdminUser, string(rCtx.secret.Data["password"]), false)
		if err != nil {
			return xerrors.WithStack(err)
		}
		for _, bucket := range rCtx.Obj.Spec.Buckets {
			if exists, err := mc.BucketExistsWithContext(ctx, bucket.Name); err != nil {
				return xerrors.WithStack(err)
			} else if !exists {
				m.logger.Info("Make bucket", zap.String("bucket", bucket.Name), zap.String("name", rCtx.Obj.Name), zap.String("namespace", rCtx.Obj.Namespace))
				if err := mc.MakeBucketWithContext(ctx, bucket.Name, ""); err != nil {
					return xerrors.WithStack(err)
				}
			}

			gotPolicyString, err := mc.GetBucketPolicy(bucket.Name)
			if err != nil {
				return xerrors.WithStack(err)
			}
			var currentPolicy *policy.BucketAccessPolicy
			if gotPolicyString != "" {
				cp := &policy.BucketAccessPolicy{}
				if err := json.Unmarshal([]byte(gotPolicyString), cp); err != nil {
					return xerrors.WithStack(err)
				}
				currentPolicy = cp
			}
			p := &policy.BucketAccessPolicy{
				Version: "2012-10-17",
			}
			switch bucket.Policy {
			case "", miniov1alpha1.BucketPolicyPrivate:
				m.logger.Debug("Set bucket policy to private", zap.String("bucket", bucket.Name), zap.String("name", rCtx.Obj.Name), zap.String("namespace", rCtx.Obj.Namespace))
				if err := mc.SetBucketPolicyWithContext(ctx, bucket.Name, ""); err != nil {
					return xerrors.WithStack(err)
				}
			case miniov1alpha1.BucketPolicyPublic:
				p.Statements = policy.SetPolicy(nil, policy.BucketPolicyReadWrite, bucket.Name, "*")
			case miniov1alpha1.BucketPolicyReadOnly:
				p.Statements = policy.SetPolicy(nil, policy.BucketPolicyReadOnly, bucket.Name, "*")
			}
			if len(p.Statements) > 0 && currentPolicy != nil && !reflect.DeepEqual(p.Statements, currentPolicy.Statements) {
				b, err := json.Marshal(p)
				if err != nil {
					return xerrors.WithStack(err)
				}
				m.logger.Debug("Set bucket policy", zap.String("bucket", bucket.Name), zap.String("name", rCtx.Obj.Name), zap.String("namespace", rCtx.Obj.Namespace))
				if err := mc.SetBucketPolicyWithContext(ctx, bucket.Name, string(b)); err != nil {
					return xerrors.WithStack(err)
				}
			}

			if bucket.CreateIndexFile {
				stat, err := mc.StatObjectWithContext(ctx, bucket.Name, "index.html", minio.StatObjectOptions{})
				if err != nil {
					var mErr minio.ErrorResponse
					if errors.As(err, &mErr) {
						if mErr.Code != "NoSuchKey" {
							return xerrors.WithStack(err)
						}
						// NoSuchKey is not error
					} else {
						return xerrors.WithStack(err)
					}
				}
				if stat.Key == "" {
					m.logger.Debug("Create index file", zap.String("bucket", bucket.Name), zap.String("name", rCtx.Obj.Name), zap.String("namespace", rCtx.Obj.Namespace))
					if _, err := mc.PutObjectWithContext(ctx, bucket.Name, "index.html", strings.NewReader(""), 0, minio.PutObjectOptions{}); err != nil {
						return xerrors.WithStack(err)
					}
				}
			}
		}
	}
	return nil
}

func (m *minIOClusterReconciler) Finalize(ctx context.Context, obj *miniov1alpha1.MinIOCluster) error {
	m.logger.Debug("Start finalizing MinIOCluster")
	if m.logger.Level() == zapcore.DebugLevel {
		defer m.logger.Debug("Finished finalizing MinIOCluster")
	}
	rCtx, err := m.newContext(obj)
	if err != nil {
		return err
	}

	for _, p := range rCtx.pods {
		m.changed = true
		if err := m.coreClient.CoreV1().Pods(p.Namespace).Delete(ctx, p.Name, metav1.DeleteOptions{}); err != nil {
			return controllerutil.WrapRetryError(xerrors.WithStack(err))
		}
	}
	for _, pvc := range rCtx.pvc {
		m.changed = true
		if err := m.coreClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Delete(ctx, pvc.Name, metav1.DeleteOptions{}); err != nil {
			return controllerutil.WrapRetryError(xerrors.WithStack(err))
		}
	}
	if rCtx.svcs != nil {
		m.changed = true
		for _, svc := range rCtx.svcs {
			if err := m.coreClient.CoreV1().Services(svc.Namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{}); err != nil {
				return controllerutil.WrapRetryError(xerrors.WithStack(err))
			}
		}
	}
	if rCtx.secret != nil {
		m.changed = true
		if err := m.coreClient.CoreV1().Secrets(rCtx.secret.Namespace).Delete(ctx, rCtx.secret.Name, metav1.DeleteOptions{}); err != nil {
			return controllerutil.WrapRetryError(xerrors.WithStack(err))
		}
	}

	// Remove finalizer
	if m.changed {
		if err := m.reloadContext(rCtx); err != nil {
			return err
		}
	}
	if rCtx.NoResources() {
		rCtx.Obj.Finalizers = enumerable.Delete(rCtx.Obj.Finalizers, minIOClusterControllerFinalizerName)
		m.logger.Debug("Update MinIOCluster", zap.String("name", rCtx.Obj.Name), zap.String("namespace", rCtx.Obj.Namespace))
		_, err = m.mClient.UpdateMinIOCluster(ctx, rCtx.Obj, metav1.UpdateOptions{})
		if err != nil {
			return xerrors.WithStack(err)
		}
	}
	return nil
}

func (m *minIOClusterReconciler) newContext(obj *miniov1alpha1.MinIOCluster) (*reconcileContext, error) {
	ctx := &reconcileContext{original: obj.DeepCopy(), Obj: obj}
	if err := m.reloadContext(ctx); err != nil {
		return nil, err
	}
	return ctx, nil
}

func (m *minIOClusterReconciler) reloadContext(ctx *reconcileContext) error {
	r, err := labels.NewRequirement(miniov1alpha1.LabelNameMinIOName, selection.Equals, []string{ctx.Obj.Name})
	if err != nil {
		return xerrors.WithStack(err)
	}
	pods, err := m.podLister.Pods(ctx.Obj.Namespace).List(labels.NewSelector().Add(*r))
	if err != nil {
		return xerrors.WithStack(err)
	}
	owned := make([]*corev1.Pod, 0)
	for _, v := range pods {
		if len(v.OwnerReferences) == 0 {
			continue
		}
		if !v.DeletionTimestamp.IsZero() {
			continue
		}
		for _, ref := range v.OwnerReferences {
			if ref.UID == ctx.Obj.UID {
				owned = append(owned, v.DeepCopy())
				break
			}
		}
	}
	sort.Slice(owned, func(i, j int) bool { return owned[i].Name < owned[j].Name })
	ctx.pods = owned

	p, err := m.pvcLister.PersistentVolumeClaims(ctx.Obj.Namespace).List(labels.NewSelector().Add(*r))
	if err != nil {
		return xerrors.WithStack(err)
	}
	pvcs := make([]*corev1.PersistentVolumeClaim, 0)
	for _, pvc := range p {
		if len(pvc.OwnerReferences) == 0 {
			continue
		}
		if !pvc.DeletionTimestamp.IsZero() {
			continue
		}
		for _, ref := range pvc.OwnerReferences {
			if ref.UID == ctx.Obj.UID {
				pvcs = append(pvcs, pvc)
				break
			}
		}
	}
	ctx.pvc = pvcs

	svc, err := m.serviceLister.Services(ctx.Obj.Namespace).Get(ctx.Obj.Name)
	if err != nil && !kerrors.IsNotFound(err) {
		return xerrors.WithStack(err)
	} else if svc != nil {
		ctx.svcs = []*corev1.Service{svc}
	} else {
		ctx.svcs = nil
	}
	if ctx.Obj.Spec.Nodes > 1 {
		svc, err = m.serviceLister.Services(ctx.Obj.Namespace).Get(fmt.Sprintf("%s-hl", ctx.Obj.Name))
		if err != nil && !kerrors.IsNotFound(err) {
			return xerrors.WithStack(err)
		} else if svc != nil {
			ctx.svcs = append(ctx.svcs, svc)
		}
	}

	secret, err := m.secretLister.Secrets(ctx.Obj.Namespace).Get(ctx.Obj.Name)
	if err != nil && !kerrors.IsNotFound(err) {
		return xerrors.WithStack(err)
	}
	ctx.secret = secret

	m.changed = false
	return nil
}

func (m *minIOClusterReconciler) pods(obj *miniov1alpha1.MinIOCluster) []*corev1.Pod {
	pods := make([]*corev1.Pod, 0)
	for i := range obj.Spec.Nodes {
		pods = append(pods, m.pod(obj, i+1))
	}

	return pods
}

func (m *minIOClusterReconciler) pod(obj *miniov1alpha1.MinIOCluster, index int) *corev1.Pod {
	dataVolumeSource := k8sfactory.NewPersistentVolumeClaimVolumeSource("data", "/data", fmt.Sprintf("%s-data-%d", obj.Name, index))
	container := k8sfactory.ContainerFactory(nil,
		k8sfactory.Name("minio"),
		k8sfactory.Image(obj.Spec.Image, nil),
		k8sfactory.Args("server", "--address=:9000", "--console-address=:8080", dataVolumeSource.Mount.MountPath),
		k8sfactory.EnvVar("MINIO_BROWSER_LOGIN_ANIMATION", "off"),
		k8sfactory.EnvVar("MINIO_BROWSER", "on"),
		k8sfactory.EnvVar("MINIO_ROOT_USER", defaultMinIOClusterAdminUser),
		k8sfactory.EnvFromSecret("MINIO_ROOT_PASSWORD", obj.Name, "password"),
		k8sfactory.Volume(dataVolumeSource),
		k8sfactory.Port("api", corev1.ProtocolTCP, 9000),
		k8sfactory.Port("http", corev1.ProtocolTCP, 8080),
		k8sfactory.LivenessProbe(
			k8sfactory.ProbeFactory(nil,
				k8sfactory.ProbeHandler(k8sfactory.HTTPProbe(9000, "/minio/health/live")),
			),
		),
		k8sfactory.ReadinessProbe(
			k8sfactory.ProbeFactory(nil,
				k8sfactory.ProbeHandler(k8sfactory.HTTPProbe(9000, "/minio/health/ready")),
			),
		),
	)
	pod := k8sfactory.PodFactory(nil,
		k8sfactory.Namef("%s-%d", obj.Name, index),
		k8sfactory.Namespace(obj.Namespace),
		k8sfactory.Label(miniov1alpha1.LabelNameMinIOName, obj.Name),
		k8sfactory.Volume(dataVolumeSource),
		k8sfactory.ControlledBy(obj, client.Scheme),
	)

	// HA mode
	if obj.Spec.Nodes > 1 {
		subdomain := fmt.Sprintf("%s-hl", obj.Name)
		pod = k8sfactory.PodFactory(pod,
			k8sfactory.Subdomain(subdomain),
			k8sfactory.Hostname(pod.Name),
			k8sfactory.PreferredInterPodAntiAffinity(
				100,
				k8sfactory.MatchExpression(metav1.LabelSelectorRequirement{
					Key:      miniov1alpha1.LabelNameMinIOName,
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{obj.Name},
				}),
				"kubernetes.io/hostname",
			),
		)
		container = k8sfactory.ContainerFactory(container,
			k8sfactory.EnvVar("MINIO_VOLUMES", fmt.Sprintf("http://%s-{1...%d}.%s.%s.svc:9000/data", obj.Name, obj.Spec.Nodes, subdomain, obj.Namespace)),
			k8sfactory.LivenessProbe(
				k8sfactory.ProbeFactory(nil,
					k8sfactory.ProbeHandler(k8sfactory.HTTPProbe(9000, "/minio/health/live")),
					k8sfactory.InitialDelay(60),
				),
			),
			k8sfactory.ReadinessProbe(
				k8sfactory.ProbeFactory(nil,
					k8sfactory.ProbeHandler(k8sfactory.HTTPProbe(9000, "/minio/health/ready")),
					k8sfactory.InitialDelay(60),
				),
			),
		)
	}

	return k8sfactory.PodFactory(pod, k8sfactory.Container(container))
}

func (m *minIOClusterReconciler) pvc(obj *miniov1alpha1.MinIOCluster, index int) *corev1.PersistentVolumeClaim {
	nodeSize := obj.Spec.TotalSize / obj.Spec.Nodes
	pvc := k8sfactory.PersistentVolumeClaimFactory(nil,
		k8sfactory.Namef("%s-data-%d", obj.Name, index),
		k8sfactory.Namespace(obj.Namespace),
		k8sfactory.Labels(map[string]string{miniov1alpha1.LabelNameMinIOName: obj.Name}),
		k8sfactory.Requests(corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse(fmt.Sprintf("%dGi", nodeSize)),
		}),
		k8sfactory.AccessModes(corev1.ReadWriteOnce),
		k8sfactory.ControlledBy(obj, client.Scheme),
	)
	if obj.Spec.StorageClassName != "" {
		pvc = k8sfactory.PersistentVolumeClaimFactory(pvc, k8sfactory.StorageClassName(obj.Spec.StorageClassName))
	}
	return pvc
}

func (m *minIOClusterReconciler) secret(obj *miniov1alpha1.MinIOCluster) *corev1.Secret {
	return k8sfactory.SecretFactory(nil,
		k8sfactory.Name(obj.Name),
		k8sfactory.Namespace(obj.Namespace),
		k8sfactory.Labels(map[string]string{miniov1alpha1.LabelNameMinIOName: obj.Name}),
		k8sfactory.Data("password", []byte(stringsutil.RandomString(32))),
		k8sfactory.ControlledBy(obj, client.Scheme),
	)
}

func (m *minIOClusterReconciler) services(obj *miniov1alpha1.MinIOCluster) []*corev1.Service {
	services := make([]*corev1.Service, 1)
	services[0] = k8sfactory.ServiceFactory(nil,
		k8sfactory.Name(obj.Name),
		k8sfactory.Namespace(obj.Namespace),
		k8sfactory.Labels(map[string]string{miniov1alpha1.LabelNameMinIOName: obj.Name}),
		k8sfactory.Port("api", corev1.ProtocolTCP, 9000),
		k8sfactory.Port("http", corev1.ProtocolTCP, 8080),
		k8sfactory.Selector(miniov1alpha1.LabelNameMinIOName, obj.Name),
		k8sfactory.ControlledBy(obj, client.Scheme),
	)

	// HA mode
	if obj.Spec.Nodes > 1 {
		s := k8sfactory.ServiceFactory(nil,
			k8sfactory.Namef("%s-hl", obj.Name),
			k8sfactory.Namespace(obj.Namespace),
			k8sfactory.Labels(map[string]string{miniov1alpha1.LabelNameMinIOName: obj.Name}),
			k8sfactory.Selector(miniov1alpha1.LabelNameMinIOName, obj.Name),
			k8sfactory.ClusterIP,
			k8sfactory.IPNone,
			k8sfactory.PublishNotReadyAddresses,
			k8sfactory.Port("api", corev1.ProtocolTCP, 9000),
			k8sfactory.ControlledBy(obj, client.Scheme),
		)
		services = append(services, s)
	}
	return services
}

type reconcileContext struct {
	Obj *miniov1alpha1.MinIOCluster

	original *miniov1alpha1.MinIOCluster

	pods   []*corev1.Pod
	pvc    []*corev1.PersistentVolumeClaim
	svcs   []*corev1.Service
	secret *corev1.Secret
}

func (c *reconcileContext) NoResources() bool {
	return len(c.pods) == 0 && len(c.pvc) == 0 && len(c.svcs) == 0 && c.secret == nil
}

func (c *reconcileContext) StatusChanged() bool {
	return !reflect.DeepEqual(c.original.Status, c.Obj.Status)
}

func (c *reconcileContext) CurrentPhase() miniov1alpha1.ClusterPhase {
	if c.Obj.Spec.Nodes != len(c.pods) {
		return miniov1alpha1.ClusterPhaseCreating
	}

	for _, pod := range c.pods {
		if pod.Status.Phase != corev1.PodRunning {
			return miniov1alpha1.ClusterPhaseCreating
		}
		for _, v := range pod.Status.Conditions {
			if v.Type == corev1.PodReady {
				if v.Status != corev1.ConditionTrue {
					return miniov1alpha1.ClusterPhaseCreating
				}
			}
		}
	}

	return miniov1alpha1.ClusterPhaseRunning
}
