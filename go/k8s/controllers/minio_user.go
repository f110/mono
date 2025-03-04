package controllers

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7/pkg/credentials"
	miniocontrollerv1beta1 "github.com/minio/minio-operator/pkg/apis/miniocontroller/v1beta1"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"go.f110.dev/mono/go/api/miniov1alpha1"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/k8s/client"
	"go.f110.dev/mono/go/k8s/controllers/controllerutil"
	"go.f110.dev/mono/go/k8s/k8sfactory"
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

	config      *rest.Config
	coreClient  kubernetes.Interface
	vaultClient *vault.Client
	mClient     *client.MinioV1alpha1

	configMapLister corev1listers.ConfigMapLister
	secretLister    corev1listers.SecretLister
	serviceLister   corev1listers.ServiceLister
	instanceLister  *client.MiniocontrollerV1beta1MinIOInstanceLister
	clusterLister   *client.MinioV1alpha1MinIOClusterLister

	transport         http.RoundTripper
	runOutsideCluster bool
}

func NewMinIOUserController(
	coreClient kubernetes.Interface,
	apiClient *client.Set,
	cfg *rest.Config,
	coreSharedInformerFactory kubeinformers.SharedInformerFactory,
	factory *client.InformerFactory,
	vaultClient *vault.Client,
	runOutsideCluster bool,
) (*MinIOUserController, error) {
	minioInformers := client.NewMinioV1alpha1Informer(factory.Cache(), apiClient.MinioV1alpha1, metav1.NamespaceAll, 30*time.Second)
	minioUserInformer := minioInformers.MinIOUserInformer()
	minioUserLister := minioInformers.MinIOUserLister()

	controllerInformers := client.NewMiniocontrollerV1beta1Informer(factory.Cache(), apiClient.MiniocontrollerV1beta1, metav1.NamespaceAll, 30*time.Second)
	instanceInformer := controllerInformers.MinIOInstanceInformer()
	instanceLister := controllerInformers.MinIOInstanceLister()
	clusterLister := minioInformers.MinIOClusterLister()

	configMapInformer := coreSharedInformerFactory.Core().V1().ConfigMaps()
	serviceInformer := coreSharedInformerFactory.Core().V1().Services()
	secretInformer := coreSharedInformerFactory.Core().V1().Secrets()

	c := &MinIOUserController{
		config:            cfg,
		coreClient:        coreClient,
		vaultClient:       vaultClient,
		mClient:           apiClient.MinioV1alpha1,
		configMapLister:   configMapInformer.Lister(),
		secretLister:      secretInformer.Lister(),
		serviceLister:     serviceInformer.Lister(),
		instanceLister:    instanceLister,
		clusterLister:     clusterLister,
		runOutsideCluster: runOutsideCluster,
	}
	c.GenericControllerBase = controllerutil.NewGenericControllerBase[*miniov1alpha1.MinIOUser](
		"minio-user-controller",
		c.newReconciler,
		coreClient,
		[]cache.SharedIndexInformer{minioUserInformer},
		[]cache.SharedIndexInformer{instanceInformer, configMapInformer.Informer(), serviceInformer.Informer(), secretInformer.Informer()},
		[]string{minIOUserControllerFinalizerName},
		minioUserLister.Get,
		apiClient.MinioV1alpha1.UpdateMinIOUser,
	)

	return c, nil
}

func (c *MinIOUserController) newReconciler() controllerutil.GenericReconciler[*miniov1alpha1.MinIOUser] {
	return &minIOUserReconciler{
		config:            c.config,
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
	config      *rest.Config
	mClient     *client.MinioV1alpha1
	coreClient  kubernetes.Interface
	vaultClient *vault.Client

	configMapLister corev1listers.ConfigMapLister
	secretLister    corev1listers.SecretLister
	serviceLister   corev1listers.ServiceLister
	instanceLister  *client.MiniocontrollerV1beta1MinIOInstanceLister
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
	creds, err := u.secretLister.Secrets(instance.Namespace).Get(instance.Spec.CredsSecret.Name)
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
	sc, err := u.secretLister.Secrets(cluster.Namespace).Get(cluster.Name)
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

	oldCM, err := u.configMapLister.ConfigMaps(cm.Namespace).Get(cm.Name)
	if apierrors.IsNotFound(err) {
		u.logger.Info("Create ConfigMap", zap.String("name", cm.Name), zap.String("namespace", cm.Namespace))
		u.coreClient.CoreV1().ConfigMaps(cm.Namespace).Create(ctx, cm, metav1.CreateOptions{})
	} else if err != nil {
		return xerrors.WithStack(err)
	} else {
		if v, ok := oldCM.Data["accesskey"]; !ok || v != string(secret.Data["accesskey"]) {
			newCM := oldCM.DeepCopy()
			newCM.Data = cm.Data
			u.logger.Info("Update ConfigMap", zap.String("name", cm.Name), zap.String("namespace", cm.Namespace))
			if _, err := u.coreClient.CoreV1().ConfigMaps(cm.Namespace).Update(ctx, newCM, metav1.UpdateOptions{}); err != nil {
				return xerrors.WithStack(err)
			}
		}
	}
	return nil
}

func (u *minIOUserReconciler) setStatus(user *miniov1alpha1.MinIOUser) error {
	secret, err := u.secretLister.Secrets(user.Namespace).Get(fmt.Sprintf("%s-accesskey", user.Name))
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
	secret, err := u.secretLister.Secrets(user.Namespace).Get(fmt.Sprintf("%s-accesskey", user.Name))
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
	secret, err = u.coreClient.CoreV1().Secrets(user.Namespace).Create(ctx, secret, metav1.CreateOptions{})
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
	creds, err := u.secretLister.Secrets(instance.Namespace).Get(instance.Spec.CredsSecret.Name)
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

	secret, err := u.secretLister.Secrets(minioUser.Namespace).Get(fmt.Sprintf("%s-accesskey", minioUser.Name))
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

	if err := u.coreClient.CoreV1().Secrets(secret.Namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{}); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (u *minIOUserReconciler) deleteUserFromCluster(ctx context.Context, minioUser *miniov1alpha1.MinIOUser, cluster *miniov1alpha1.MinIOCluster) error {
	sc, err := u.secretLister.Secrets(cluster.Namespace).Get(cluster.Name)
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

	accessKeySecret, err := u.secretLister.Secrets(minioUser.Namespace).Get(fmt.Sprintf("%s-accesskey", minioUser.Name))
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

	if err := u.coreClient.CoreV1().Secrets(accessKeySecret.Namespace).Delete(ctx, accessKeySecret.Name, metav1.DeleteOptions{}); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (u *minIOUserReconciler) getMinIOInstanceEndpoint(ctx context.Context, instance *miniocontrollerv1beta1.MinIOInstance) (string, *portforward.PortForwarder, error) {
	svc, err := u.serviceLister.Services(instance.Namespace).Get(fmt.Sprintf("%s-hl-svc", instance.Name))
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
	svc, err := u.serviceLister.Services(cluster.Namespace).Get(cluster.Name)
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
	podList, err := u.coreClient.CoreV1().Pods(svc.Namespace).List(
		ctx,
		metav1.ListOptions{LabelSelector: selector.String()},
	)
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
		return nil, xerrors.New("all pods are not running yet")
	}

	req := u.coreClient.CoreV1().RESTClient().Post().Resource("pods").Namespace(svc.Namespace).Name(pod.Name).SubResource("portforward")
	transport, upgrader, err := spdy.RoundTripperFor(u.config)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	go func() {
		err := pf.ForwardPorts()
		if err != nil {
			switch v := err.(type) {
			case *apierrors.StatusError:
				u.logger.Debug("StatusError", zap.Error(v))
			}
			u.logger.Error("Failed port forwarding", zap.Error(err))
		}
	}()

	select {
	case <-readyCh:
	case <-time.After(5 * time.Second):
		return nil, xerrors.New("timed out")
	}

	return pf, nil
}
