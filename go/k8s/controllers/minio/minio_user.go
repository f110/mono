package minio

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"time"

	miniocontrollerv1beta1 "github.com/minio/minio-operator/pkg/apis/miniocontroller/v1beta1"
	"github.com/minio/minio/pkg/madmin"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
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

	"go.f110.dev/mono/go/api/miniov1alpha1"
	"go.f110.dev/mono/go/k8s/client"
	"go.f110.dev/mono/go/k8s/controllers/controllerutil"
	"go.f110.dev/mono/go/stringsutil"
	"go.f110.dev/mono/go/vault"
)

const (
	minIOUserControllerFinalizerName = "minio-user-controller.minio.f110.dev/finalizer"
	accessKeyLength                  = 16
	secretKeyLength                  = 24
)

type UserController struct {
	*controllerutil.ControllerBase

	config      *rest.Config
	coreClient  kubernetes.Interface
	vaultClient *vault.Client
	mClient     *client.MinioV1alpha1
	muLister    *client.MinioV1alpha1MinIOUserLister

	secretLister   corev1listers.SecretLister
	serviceLister  corev1listers.ServiceLister
	instanceLister *client.MiniocontrollerV1beta1MinIOInstanceLister

	queue *controllerutil.WorkQueue

	transport         http.RoundTripper
	runOutsideCluster bool
}

var _ controllerutil.Controller = &UserController{}

func NewUserController(
	coreClient kubernetes.Interface,
	apiClient *client.Set,
	cfg *rest.Config,
	coreSharedInformerFactory kubeinformers.SharedInformerFactory,
	factory *client.InformerFactory,
	vaultClient *vault.Client,
	runOutsideCluster bool,
) (*UserController, error) {
	minioInformers := client.NewMinioV1alpha1Informer(factory.Cache(), apiClient.MinioV1alpha1, metav1.NamespaceAll, 30*time.Second)
	minioUserInformer := minioInformers.MinIOUserInformer()
	minioUserLister := minioInformers.MinIOUserLister()

	controllerInformers := client.NewMiniocontrollerV1beta1Informer(factory.Cache(), apiClient.MiniocontrollerV1beta1, metav1.NamespaceAll, 30*time.Second)
	instanceInformer := controllerInformers.MinIOInstanceInformer()
	instanceLister := controllerInformers.MinIOInstanceLister()

	serviceInformer := coreSharedInformerFactory.Core().V1().Services()
	secretInformer := coreSharedInformerFactory.Core().V1().Secrets()

	c := &UserController{
		config:            cfg,
		coreClient:        coreClient,
		vaultClient:       vaultClient,
		mClient:           apiClient.MinioV1alpha1,
		muLister:          minioUserLister,
		secretLister:      secretInformer.Lister(),
		serviceLister:     serviceInformer.Lister(),
		instanceLister:    instanceLister,
		runOutsideCluster: runOutsideCluster,
	}
	c.ControllerBase = controllerutil.NewBase(
		"minio-user-operator",
		c,
		coreClient,
		[]cache.SharedIndexInformer{minioUserInformer},
		[]cache.SharedIndexInformer{instanceInformer, serviceInformer.Informer(), secretInformer.Informer()},
		[]string{minIOBucketControllerFinalizerName},
	)

	return c, nil
}

func (c *UserController) ObjectToKeys(obj interface{}) []string {
	user, ok := obj.(*miniov1alpha1.MinIOUser)
	if !ok {
		return nil
	}
	key, err := cache.MetaNamespaceKeyFunc(user)
	if err != nil {
		return nil
	}

	return []string{key}
}

func (c *UserController) GetObject(key string) (runtime.Object, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	user, err := c.muLister.Get(namespace, name)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return user, nil
}

func (c *UserController) UpdateObject(ctx context.Context, obj runtime.Object) (runtime.Object, error) {
	user := obj.(*miniov1alpha1.MinIOUser)

	user, err := c.mClient.UpdateMinIOUser(ctx, user, metav1.UpdateOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return user, nil
}

func (c *UserController) Reconcile(ctx context.Context, obj runtime.Object) error {
	currentUser := obj.(*miniov1alpha1.MinIOUser)
	minioUser := currentUser.DeepCopy()

	s, err := metav1.LabelSelectorAsSelector(&minioUser.Spec.Selector)
	if err != nil {
		return xerrors.WithStack(err)
	}
	instances, err := c.instanceLister.List(minioUser.Namespace, s)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if len(instances) == 0 {
		c.Log().Debug("MinIO instance not found", zap.String("selector", metav1.FormatLabelSelector(&minioUser.Spec.Selector)))
		return nil
	}
	if len(instances) > 1 {
		return xerrors.New("found some instances")
	}

	for _, instance := range instances {
		creds, err := c.secretLister.Secrets(instance.Namespace).Get(instance.Spec.CredsSecret.Name)
		if err != nil {
			return xerrors.WithStack(err)
		}

		instanceEndpoint, forwarder, err := c.getMinIOInstanceEndpoint(ctx, instance)
		if err != nil {
			return xerrors.WithStack(err)
		}
		if forwarder != nil {
			defer forwarder.Close()
		}

		adminClient, err := madmin.New(
			instanceEndpoint,
			string(creds.Data["accesskey"]),
			string(creds.Data["secretkey"]),
			false,
		)
		if err != nil {
			return xerrors.WithStack(err)
		}
		if c.transport != nil {
			adminClient.SetCustomTransport(c.transport)
		}

		secret, err := c.ensureUser(ctx, adminClient, minioUser)
		if err != nil {
			return xerrors.WithStack(err)
		}

		if minioUser.Spec.Path != "" && !minioUser.Status.Vault {
			if err := c.saveAccessKeyToVault(minioUser, secret); err != nil {
				return xerrors.WithStack(err)
			}
		}
	}

	if err := c.setStatus(minioUser); err != nil {
		return xerrors.WithStack(err)
	}

	if !reflect.DeepEqual(minioUser.Status, currentUser.Status) {
		_, err = c.mClient.UpdateStatusMinIOUser(ctx, minioUser, metav1.UpdateOptions{})
		if err != nil {
			return xerrors.WithStack(err)
		}
	}

	return nil
}

func (c *UserController) setStatus(user *miniov1alpha1.MinIOUser) error {
	secret, err := c.secretLister.Secrets(user.Namespace).Get(fmt.Sprintf("%s-accesskey", user.Name))
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

func (c *UserController) ensureUser(ctx context.Context, adminClient *madmin.AdminClient, user *miniov1alpha1.MinIOUser) (*corev1.Secret, error) {
	secret, err := c.secretLister.Secrets(user.Namespace).Get(fmt.Sprintf("%s-accesskey", user.Name))
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, xerrors.WithStack(err)
	}
	if err == nil {
		return secret, nil
	}

	accessKey := stringsutil.RandomString(accessKeyLength)
	secretKey := stringsutil.RandomString(secretKeyLength)
	if err := adminClient.AddUser(ctx, accessKey, secretKey); err != nil {
		return nil, xerrors.WithStack(err)
	}

	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-accesskey", user.Name),
			Namespace: user.Namespace,
		},
		Data: map[string][]byte{
			"accesskey": []byte(accessKey),
			"secretkey": []byte(secretKey),
		},
	}
	controllerutil.SetOwner(secret, user, client.Scheme)
	secret, err = c.coreClient.CoreV1().Secrets(user.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	return secret, nil
}

func (c *UserController) saveAccessKeyToVault(user *miniov1alpha1.MinIOUser, secret *corev1.Secret) error {
	data := map[string]string{
		"accesskey": string(secret.Data["accesskey"]),
		"secretkey": string(secret.Data["secretkey"]),
	}

	err := c.vaultClient.Set(context.Background(), user.Spec.MountPath, user.Spec.Path, data)
	if err != nil {
		return err
	}

	return nil
}

func (c *UserController) Finalize(ctx context.Context, obj runtime.Object) error {
	minioUser := obj.(*miniov1alpha1.MinIOUser)

	s, err := metav1.LabelSelectorAsSelector(&minioUser.Spec.Selector)
	if err != nil {
		return xerrors.WithStack(err)
	}
	instances, err := c.instanceLister.List(minioUser.Namespace, s)
	if err != nil {
		return err
	}

	for _, instance := range instances {
		creds, err := c.secretLister.Secrets(instance.Namespace).Get(instance.Spec.CredsSecret.Name)
		if err != nil {
			return xerrors.WithStack(err)
		}

		instanceEndpoint, forwarder, err := c.getMinIOInstanceEndpoint(ctx, instance)
		if err != nil {
			return xerrors.WithStack(err)
		}
		if forwarder != nil {
			defer forwarder.Close()
		}

		secret, err := c.secretLister.Secrets(minioUser.Namespace).Get(fmt.Sprintf("%s-accesskey", minioUser.Name))
		if apierrors.IsNotFound(err) {
			continue
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
		c.Log().Debug("Remove minio user", zap.String("name", minioUser.Name))

		if err := c.coreClient.CoreV1().Secrets(secret.Namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{}); err != nil {
			return xerrors.WithStack(err)
		}
	}

	minioUser.Finalizers = removeString(minioUser.Finalizers, minIOUserControllerFinalizerName)

	_, err = c.mClient.UpdateMinIOUser(ctx, minioUser, metav1.UpdateOptions{})
	return err
}

func (c *UserController) getMinIOInstanceEndpoint(
	ctx context.Context,
	instance *miniocontrollerv1beta1.MinIOInstance,
) (string, *portforward.PortForwarder, error) {
	svc, err := c.serviceLister.Services(instance.Namespace).Get(fmt.Sprintf("%s-hl-svc", instance.Name))
	if err != nil {
		return "", nil, xerrors.WithStack(err)
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

func (c *UserController) portForward(
	ctx context.Context,
	svc *corev1.Service,
	port int,
) (*portforward.PortForwarder, error) {
	selector := labels.SelectorFromSet(svc.Spec.Selector)
	podList, err := c.coreClient.CoreV1().Pods(svc.Namespace).List(
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

	req := c.coreClient.CoreV1().RESTClient().Post().Resource("pods").Namespace(svc.Namespace).Name(pod.Name).SubResource("portforward")
	transport, upgrader, err := spdy.RoundTripperFor(c.config)
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
				c.Log().Debug("StatusError", zap.Error(v))
			}
			c.Log().Error("Failed port forwarding", zap.Error(err))
		}
	}()

	select {
	case <-readyCh:
	case <-time.After(5 * time.Second):
		return nil, xerrors.New("timed out")
	}

	return pf, nil
}
