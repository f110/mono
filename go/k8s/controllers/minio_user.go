package controllers

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.f110.dev/kubeproto/go/apis/corev1"
	"go.f110.dev/kubeproto/go/apis/metav1"
	"go.f110.dev/kubeproto/go/k8sclient"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/portforward"

	"go.f110.dev/mono/go/api/miniov1alpha1"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/k8s/client"
	"go.f110.dev/mono/go/k8s/controllers/controllerutil"
	"go.f110.dev/mono/go/k8s/k8sfactory"
	"go.f110.dev/mono/go/k8s/thirdpartyapi/minio-operator/miniocontrollerv1beta1"
	"go.f110.dev/mono/go/k8s/thirdpartyclient"
	"go.f110.dev/mono/go/stringsutil"
	"go.f110.dev/mono/go/vault"
)

const (
	minIOUserControllerFinalizerName = "minio-user-controller.minio.f110.dev/finalizer"
	accessKeyLength                  = 16
	secretKeyLength                  = 24
)

type MinIOUserController struct {
	*controllerutil.GenericControllerBase[*miniov1alpha1.MinIOUser]

	coreClient  *k8sclient.Set
	vaultClient *vault.Client
	mClient     *client.MinioV1alpha1

	configMapLister *k8sclient.CoreV1ConfigMapLister
	secretLister    *k8sclient.CoreV1SecretLister
	serviceLister   *k8sclient.CoreV1ServiceLister
	instanceLister  *thirdpartyclient.MiniocontrollerMinV1beta1MinIOInstanceLister
	clusterLister   *client.MinioV1alpha1MinIOClusterLister

	transport         http.RoundTripper
	runOutsideCluster bool
}

func NewMinIOUserController(
	coreClient *k8sclient.Set,
	k8sClient kubernetes.Interface,
	apiClient *client.Set,
	tpClient *thirdpartyclient.Set,
	coreSharedInformerFactory *k8sclient.InformerFactory,
	factory *client.InformerFactory,
	tpFactory *thirdpartyclient.InformerFactory,
	vaultClient *vault.Client,
	runOutsideCluster bool,
) (*MinIOUserController, error) {
	minioInformers := client.NewMinioV1alpha1Informer(factory.Cache(), apiClient.MinioV1alpha1, metav1.NamespaceAll, 30*time.Second)
	minioUserInformer := minioInformers.MinIOUserInformer()
	minioUserLister := minioInformers.MinIOUserLister()

	controllerInformers := thirdpartyclient.NewMiniocontrollerMinV1beta1Informer(tpFactory.Cache(), tpClient.MiniocontrollerMinV1beta1, metav1.NamespaceAll, 30*time.Second)
	instanceInformer := controllerInformers.MinIOInstanceInformer()
	instanceLister := controllerInformers.MinIOInstanceLister()
	clusterLister := minioInformers.MinIOClusterLister()

	coreInformer := k8sclient.NewCoreV1Informer(coreSharedInformerFactory.Cache(), coreClient.CoreV1, metav1.NamespaceAll, 30*time.Second)
	configMapInformer := coreInformer.ConfigMapInformer()
	serviceInformer := coreInformer.ServiceInformer()
	secretInformer := coreInformer.SecretInformer()

	c := &MinIOUserController{
		coreClient:        coreClient,
		vaultClient:       vaultClient,
		mClient:           apiClient.MinioV1alpha1,
		configMapLister:   coreInformer.ConfigMapLister(),
		secretLister:      coreInformer.SecretLister(),
		serviceLister:     coreInformer.ServiceLister(),
		instanceLister:    instanceLister,
		clusterLister:     clusterLister,
		runOutsideCluster: runOutsideCluster,
	}
	c.GenericControllerBase = controllerutil.NewGenericControllerBase[*miniov1alpha1.MinIOUser](
		"minio-user-controller",
		c.newReconciler,
		k8sClient,
		[]cache.SharedIndexInformer{minioUserInformer},
		[]cache.SharedIndexInformer{instanceInformer, configMapInformer, serviceInformer, secretInformer},
		[]string{minIOUserControllerFinalizerName},
		minioUserLister.Get,
		apiClient.MinioV1alpha1.UpdateMinIOUser,
	)

	return c, nil
}

func (c *MinIOUserController) newReconciler() controllerutil.GenericReconciler[*miniov1alpha1.MinIOUser] {
	return &minIOUserReconciler{
		coreClient:        c.coreClient,
		mClient:           c.mClient,
		vaultClient:       c.vaultClient,
		configMapLister:   c.configMapLister,
		secretLister:      c.secretLister,
		serviceLister:     c.serviceLister,
		instanceLister:    c.instanceLister,
		clusterLister:     c.clusterLister,
		logger:            c.Log(),
		transport:         c.transport,
		runOutsideCluster: c.runOutsideCluster,
	}
}

type minIOUserReconciler struct {
	mClient     *client.MinioV1alpha1
	coreClient  *k8sclient.Set
	vaultClient *vault.Client

	configMapLister *k8sclient.CoreV1ConfigMapLister
	secretLister    *k8sclient.CoreV1SecretLister
	serviceLister   *k8sclient.CoreV1ServiceLister
	instanceLister  *thirdpartyclient.MiniocontrollerMinV1beta1MinIOInstanceLister
	clusterLister   *client.MinioV1alpha1MinIOClusterLister

	logger            *zap.Logger
	transport         http.RoundTripper
	runOutsideCluster bool
}

var _ controllerutil.GenericReconciler[*miniov1alpha1.MinIOUser] = (*minIOUserReconciler)(nil)

func (u *minIOUserReconciler) Reconcile(ctx context.Context, obj *miniov1alpha1.MinIOUser) error {
	var instances []*miniocontrollerv1beta1.MinIOInstance

	currentUser := obj
	minioUser := currentUser.DeepCopy()

	if minioUser.Spec.Selector != nil {
		s, err := metav1.LabelSelectorAsSelector(minioUser.Spec.Selector)
		if err != nil {
			return xerrors.WithStack(err)
		}
		clusters, err := u.clusterLister.List(minioUser.Namespace, s)
		if err != nil {
			return xerrors.WithStack(err)
		}
		switch len(clusters) {
		case 0:
		case 1:
			if err := u.makeUserForCluster(ctx, minioUser, clusters[0]); err != nil {
				return err
			}
			goto StatusUpdate
		default:
			return xerrors.New("found some clusters")
		}

		instances, err = u.instanceLister.List(minioUser.Namespace, s)
		if err != nil {
			return xerrors.WithStack(err)
		}
		if len(instances) == 0 {
			u.logger.Debug("MinIO instance not found", zap.String("selector", metav1.FormatLabelSelector(minioUser.Spec.Selector)))
			return nil
		}
		if len(instances) > 1 {
			return xerrors.New("found some instances")
		}
		if err := u.makeUserForMinIOInstance(ctx, minioUser, instances[0]); err != nil {
			return err
		}
	}

	if minioUser.Spec.InstanceRef != nil {
		namespace := minioUser.Spec.InstanceRef.Namespace
		if namespace == "" {
			namespace = minioUser.Namespace
		}
		cluster, err := u.clusterLister.Get(namespace, minioUser.Spec.InstanceRef.Name)
		if err != nil {
			return xerrors.WithStack(err)
		}
		if err := u.makeUserForCluster(ctx, minioUser, cluster); err != nil {
			return err
		}
	}

StatusUpdate:
	if !reflect.DeepEqual(minioUser.Status, currentUser.Status) {
		_, err := u.mClient.UpdateStatusMinIOUser(ctx, minioUser, metav1.UpdateOptions{})
		if err != nil {
			return xerrors.WithStack(err)
		}
	}

	return nil
}

func (u *minIOUserReconciler) makeUserForMinIOInstance(ctx context.Context, minioUser *miniov1alpha1.MinIOUser, instance *miniocontrollerv1beta1.MinIOInstance) error {
	creds, err := u.secretLister.Get(instance.Namespace, instance.Spec.CredsSecret.Name)
	if err != nil {
		return xerrors.WithStack(err)
	}

	instanceEndpoint, forwarder, err := u.getMinIOInstanceEndpoint(ctx, instance)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if forwarder != nil {
		defer forwarder.Close()
	}

	opts := &madmin.Options{
		Creds:  credentials.NewStaticV4(string(creds.Data["accesskey"]), string(creds.Data["secretkey"]), ""),
		Secure: false,
	}
	if u.transport != nil {
		opts.Transport = u.transport
	}
	adminClient, err := madmin.NewWithOptions(instanceEndpoint, opts)
	if err != nil {
		return xerrors.WithStack(err)
	}

	secret, err := u.ensureUser(ctx, adminClient, minioUser)
	if err != nil {
		return err
	}

	if u.vaultClient != nil && minioUser.Spec.Path != "" && !minioUser.Status.Vault {
		if err := u.saveAccessKeyToVault(minioUser, secret); err != nil {
			return err
		}
	}

	if err := u.ensureConfigMap(ctx, minioUser, secret); err != nil {
		return err
	}
	if err := u.setStatus(minioUser); err != nil {
		return err
	}
	return nil
}

func (u *minIOUserReconciler) makeUserForCluster(ctx context.Context, minioUser *miniov1alpha1.MinIOUser, cluster *miniov1alpha1.MinIOCluster) error {
	sc, err := u.secretLister.Get(cluster.Namespace, cluster.Name)
	if err != nil {
		return xerrors.WithStack(err)
	}

	instanceEndpoint, forwarder, err := u.getMinIOClusterEndpoint(ctx, cluster)
	if err != nil {
		return err
	}
	if forwarder != nil {
		defer forwarder.Close()
	}

	opts := &madmin.Options{Creds: credentials.NewStaticV4(defaultMinIOClusterAdminUser, string(sc.Data["password"]), ""), Secure: false}
	if u.transport != nil {
		opts.Transport = u.transport
	}
	adminClient, err := madmin.NewWithOptions(instanceEndpoint, opts)
	if err != nil {
		return xerrors.WithStack(err)
	}

	secret, err := u.ensureUser(ctx, adminClient, minioUser)
	if err != nil {
		return err
	}
	if u.vaultClient != nil && minioUser.Spec.Path != "" && !minioUser.Status.Vault {
		if err := u.saveAccessKeyToVault(minioUser, secret); err != nil {
			return err
		}
	}

	if err := u.ensureConfigMap(ctx, minioUser, secret); err != nil {
		return err
	}
	if err := u.setStatus(minioUser); err != nil {
		return err
	}
	return nil
}

func (u *minIOUserReconciler) ensureConfigMap(ctx context.Context, user *miniov1alpha1.MinIOUser, secret *corev1.Secret) error {
	cm := k8sfactory.ConfigMapFactory(nil,
		k8sfactory.Namef("%s-accesskey", user.Name),
		k8sfactory.Namespace(user.Namespace),
		k8sfactory.Data("accesskey", secret.Data["accesskey"]),
	)
	controllerutil.SetOwner(cm, user, client.Scheme)

	oldCM, err := u.configMapLister.Get(cm.Namespace, cm.Name)
	if apierrors.IsNotFound(err) {
		u.logger.Info("Create ConfigMap", zap.String("name", cm.Name), zap.String("namespace", cm.Namespace))
		u.coreClient.CoreV1.CreateConfigMap(ctx, cm, metav1.CreateOptions{})
	} else if err != nil {
		return xerrors.WithStack(err)
	} else {
		if v, ok := oldCM.Data["accesskey"]; !ok || v != string(secret.Data["accesskey"]) {
			newCM := oldCM.DeepCopy()
			newCM.Data = cm.Data
			u.logger.Info("Update ConfigMap", zap.String("name", cm.Name), zap.String("namespace", cm.Namespace))
			if _, err := u.coreClient.CoreV1.UpdateConfigMap(ctx, newCM, metav1.UpdateOptions{}); err != nil {
				return xerrors.WithStack(err)
			}
		}
	}
	return nil
}

func (u *minIOUserReconciler) setStatus(user *miniov1alpha1.MinIOUser) error {
	secret, err := u.secretLister.Get(user.Namespace, fmt.Sprintf("%s-accesskey", user.Name))
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return xerrors.WithStack(err)
	}
	user.Status.Ready = true
	user.Status.AccessKey = string(secret.Data["accesskey"])
	if user.Spec.Path != "" {
		user.Status.Vault = true
	}

	return nil
}

func (u *minIOUserReconciler) ensureUser(ctx context.Context, adminClient *madmin.AdminClient, user *miniov1alpha1.MinIOUser) (*corev1.Secret, error) {
	secret, err := u.secretLister.Get(user.Namespace, fmt.Sprintf("%s-accesskey", user.Name))
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, xerrors.WithStack(err)
	}
	if err == nil {
		return secret, nil
	}

	accessKey := stringsutil.RandomString(accessKeyLength)
	secretKey := stringsutil.RandomString(secretKeyLength)
	u.logger.Info("Create user", zap.String("accesskey", accessKey))
	if err := adminClient.AddUser(ctx, accessKey, secretKey); err != nil {
		return nil, xerrors.WithStack(err)
	}
	if user.Spec.Policy != "" {
		_, err = adminClient.AttachPolicy(ctx, madmin.PolicyAssociationReq{
			User:     accessKey,
			Policies: []string{user.Spec.Policy},
		})
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
	}

	secret = k8sfactory.SecretFactory(nil,
		k8sfactory.Namef("%s-accesskey", user.Name),
		k8sfactory.Namespace(user.Namespace),
		k8sfactory.Data("accesskey", []byte(accessKey)),
		k8sfactory.Data("secretkey", []byte(secretKey)),
	)
	controllerutil.SetOwner(secret, user, client.Scheme)
	secret, err = u.coreClient.CoreV1.CreateSecret(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	return secret, nil
}

func (u *minIOUserReconciler) saveAccessKeyToVault(user *miniov1alpha1.MinIOUser, secret *corev1.Secret) error {
	data := map[string]string{
		"accesskey": string(secret.Data["accesskey"]),
		"secretkey": string(secret.Data["secretkey"]),
	}

	err := u.vaultClient.Set(context.Background(), user.Spec.MountPath, user.Spec.Path, data)
	if err != nil {
		return err
	}

	return nil
}

func (u *minIOUserReconciler) Finalize(ctx context.Context, obj *miniov1alpha1.MinIOUser) error {
	minioUser := obj
	u.logger.Debug("Start finalizing MinIOUser")
	if u.logger.Level() == zapcore.DebugLevel {
		defer u.logger.Debug("Finished finalizing MinIOUser")
	}
	var instances []*miniocontrollerv1beta1.MinIOInstance

	s, err := metav1.LabelSelectorAsSelector(minioUser.Spec.Selector)
	if err != nil {
		return xerrors.WithStack(err)
	}
	clusters, err := u.clusterLister.List(minioUser.Namespace, s)
	if err != nil {
		return xerrors.WithStack(err)
	}
	switch len(clusters) {
	case 0:
	case 1:
		if err := u.deleteUserFromCluster(ctx, minioUser, clusters[0]); err != nil {
			return err
		}
		goto StatusUpdate
	default:
		return xerrors.New("found some clusters")
	}

	instances, err = u.instanceLister.List(minioUser.Namespace, s)
	if err != nil {
		return err
	}
	switch len(instances) {
	case 0:
		u.logger.Debug("MinIO instance not found", zap.String("selector", metav1.FormatLabelSelector(minioUser.Spec.Selector)))
		return nil
	case 1:
		if err := u.deleteUserFromInstance(ctx, minioUser, instances[0]); err != nil {
			return err
		}
	default:
		return xerrors.New("found multiple MinIO instances")
	}

StatusUpdate:
	minioUser.Finalizers = enumerable.Delete(minioUser.Finalizers, minIOUserControllerFinalizerName)

	_, err = u.mClient.UpdateMinIOUser(ctx, minioUser, metav1.UpdateOptions{})
	return err
}

func (u *minIOUserReconciler) deleteUserFromInstance(ctx context.Context, minioUser *miniov1alpha1.MinIOUser, instance *miniocontrollerv1beta1.MinIOInstance) error {
	creds, err := u.secretLister.Get(instance.Namespace, instance.Spec.CredsSecret.Name)
	if err != nil {
		return xerrors.WithStack(err)
	}

	instanceEndpoint, forwarder, err := u.getMinIOInstanceEndpoint(ctx, instance)
	if err != nil {
		return err
	}
	if forwarder != nil {
		defer forwarder.Close()
	}

	secret, err := u.secretLister.Get(minioUser.Namespace, fmt.Sprintf("%s-accesskey", minioUser.Name))
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return xerrors.WithStack(err)
	}

	adminClient, err := madmin.New(instanceEndpoint, string(creds.Data["accesskey"]), string(creds.Data["secretkey"]), false)
	if err != nil {
		return xerrors.WithStack(err)
	}

	if err := adminClient.RemoveUser(ctx, string(secret.Data["accesskey"])); err != nil {
		return xerrors.WithStack(err)
	}
	u.logger.Debug("Remove minio user", zap.String("name", minioUser.Name))

	if err := u.coreClient.CoreV1.DeleteSecret(ctx, secret.Namespace, secret.Name, metav1.DeleteOptions{}); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (u *minIOUserReconciler) deleteUserFromCluster(ctx context.Context, minioUser *miniov1alpha1.MinIOUser, cluster *miniov1alpha1.MinIOCluster) error {
	sc, err := u.secretLister.Get(cluster.Namespace, cluster.Name)
	if err != nil {
		return xerrors.WithStack(err)
	}

	instanceEndpoint, forwarder, err := u.getMinIOClusterEndpoint(ctx, cluster)
	if err != nil {
		return err
	}
	if forwarder != nil {
		defer forwarder.Close()
	}

	accessKeySecret, err := u.secretLister.Get(minioUser.Namespace, fmt.Sprintf("%s-accesskey", minioUser.Name))
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return xerrors.WithStack(err)
	}

	adminClient, err := madmin.New(instanceEndpoint, defaultMinIOClusterAdminUser, string(sc.Data["password"]), false)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if u.transport != nil {
		adminClient.SetCustomTransport(u.transport)
	}

	if err := adminClient.RemoveUser(ctx, string(accessKeySecret.Data["accesskey"])); err != nil {
		return xerrors.WithStack(err)
	}
	u.logger.Debug("Remove minio user", zap.String("name", minioUser.Name))

	if err := u.coreClient.CoreV1.DeleteSecret(ctx, accessKeySecret.Namespace, accessKeySecret.Name, metav1.DeleteOptions{}); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (u *minIOUserReconciler) getMinIOInstanceEndpoint(ctx context.Context, instance *miniocontrollerv1beta1.MinIOInstance) (string, *portforward.PortForwarder, error) {
	svc, err := u.serviceLister.Get(instance.Namespace, fmt.Sprintf("%s-hl-svc", instance.Name))
	if err != nil {
		return "", nil, xerrors.WithStack(err)
	}

	var forwarder *portforward.PortForwarder
	instanceEndpoint := fmt.Sprintf("%s-hl-svc.%s.svc:%d", instance.Name, instance.Namespace, svc.Spec.Ports[0].Port)
	if u.runOutsideCluster {
		forwarder, err = u.portForward(ctx, svc, int(svc.Spec.Ports[0].Port))
		if err != nil {
			return "", nil, err
		}

		ports, err := forwarder.GetPorts()
		if err != nil {
			return "", nil, xerrors.WithStack(err)
		}
		instanceEndpoint = fmt.Sprintf("127.0.0.1:%d", ports[0].Local)
	}

	return instanceEndpoint, forwarder, nil
}

func (u *minIOUserReconciler) getMinIOClusterEndpoint(ctx context.Context, cluster *miniov1alpha1.MinIOCluster) (string, *portforward.PortForwarder, error) {
	svc, err := u.serviceLister.Get(cluster.Namespace, cluster.Name)
	if err != nil {
		return "", nil, xerrors.WithStack(err)
	}

	var forwarder *portforward.PortForwarder
	instanceEndpoint := fmt.Sprintf("%s.%s.svc:%d", cluster.Name, cluster.Namespace, svc.Spec.Ports[0].Port)
	if u.runOutsideCluster {
		forwarder, err = u.portForward(ctx, svc, int(svc.Spec.Ports[0].Port))
		if err != nil {
			return "", nil, err
		}

		ports, err := forwarder.GetPorts()
		if err != nil {
			return "", nil, xerrors.WithStack(err)
		}
		instanceEndpoint = fmt.Sprintf("127.0.0.1:%d", ports[0].Local)
	}
	return instanceEndpoint, forwarder, nil
}

func (u *minIOUserReconciler) portForward(ctx context.Context, svc *corev1.Service, port int) (*portforward.PortForwarder, error) {
	selector := labels.SelectorFromSet(svc.Spec.Selector)
	podList, err := u.coreClient.CoreV1.ListPod(ctx, svc.Namespace, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	var pod *corev1.Pod
	for i, v := range podList.Items {
		if v.Status.Phase == corev1.PodPhaseRunning {
			pod = &podList.Items[i]
			break
		}
	}
	if pod == nil {
		return nil, xerrors.New("all pods are not running yet")
	}

	pf, _, err := u.coreClient.CoreV1.PortForward(ctx, pod, port)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return pf, nil
}
