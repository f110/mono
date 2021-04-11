package minio

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"time"

	"github.com/hashicorp/vault/api"
	miniocontrollerv1beta1 "github.com/minio/minio-operator/pkg/apis/miniocontroller/v1beta1"
	"github.com/minio/minio/pkg/madmin"
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
	"go.f110.dev/mono/go/pkg/k8s/client/versioned/scheme"
	"go.f110.dev/mono/go/pkg/k8s/controllers/controllerutil"
	informers "go.f110.dev/mono/go/pkg/k8s/informers/externalversions"
	miniov1alpha1listers "go.f110.dev/mono/go/pkg/k8s/listers/minio/v1alpha1"
	miniocontrollerv1beta1listers "go.f110.dev/mono/go/pkg/k8s/listers/miniocontroller/v1beta1"
	"go.f110.dev/mono/go/pkg/stringsutil"
)

// +kubebuilder:rbac:groups=minio.f110.dev,resources=miniousers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=minio.f110.dev,resources=miniousers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=miniocontroller.min.io,resources=minioinstances,verbs=get;list
// +kubebuilder:rbac:groups=*,resources=pods;secrets;services,verbs=get
// +kubebuilder:rbac:groups=*,resources=pods/portforward,verbs=get;list;create

const (
	minIOUserControllerFinalizerName = "minio-user-controller.minio.f110.dev/finalizer"
	accessKeyLength                  = 16
	secretKeyLength                  = 24
)

type UserController struct {
	*controllerutil.ControllerBase

	config      *rest.Config
	coreClient  kubernetes.Interface
	vaultClient *api.Client
	mClient     clientset.Interface
	muLister    miniov1alpha1listers.MinIOUserLister

	secretLister   corev1listers.SecretLister
	serviceLister  corev1listers.ServiceLister
	instanceLister miniocontrollerv1beta1listers.MinIOInstanceLister

	queue *controllerutil.WorkQueue

	transport         http.RoundTripper
	runOutsideCluster bool
}

var _ controllerutil.Controller = &UserController{}

func NewUserController(
	coreClient kubernetes.Interface,
	client clientset.Interface,
	cfg *rest.Config,
	coreSharedInformerFactory kubeinformers.SharedInformerFactory,
	sharedInformerFactory informers.SharedInformerFactory,
	vaultClient *api.Client,
	runOutsideCluster bool,
) (*UserController, error) {
	muInformer := sharedInformerFactory.Minio().V1alpha1().MinIOUsers()
	instanceInformer := sharedInformerFactory.Miniocontroller().V1beta1().MinIOInstances()
	serviceInformer := coreSharedInformerFactory.Core().V1().Services()
	secretInformer := coreSharedInformerFactory.Core().V1().Secrets()

	c := &UserController{
		config:            cfg,
		coreClient:        coreClient,
		vaultClient:       vaultClient,
		mClient:           client,
		muLister:          muInformer.Lister(),
		secretLister:      secretInformer.Lister(),
		serviceLister:     serviceInformer.Lister(),
		instanceLister:    instanceInformer.Lister(),
		runOutsideCluster: runOutsideCluster,
	}
	c.ControllerBase = controllerutil.NewBase(
		"minio-user-operator",
		c,
		coreClient,
		[]cache.SharedIndexInformer{muInformer.Informer()},
		[]cache.SharedIndexInformer{instanceInformer.Informer(), serviceInformer.Informer(), secretInformer.Informer()},
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
		return nil, xerrors.Errorf(": %w", err)
	}

	user, err := c.muLister.MinIOUsers(namespace).Get(name)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	return user, nil
}

func (c *UserController) UpdateObject(ctx context.Context, obj runtime.Object) (runtime.Object, error) {
	user := obj.(*miniov1alpha1.MinIOUser)

	user, err := c.mClient.MinioV1alpha1().MinIOUsers(user.Namespace).Update(ctx, user, metav1.UpdateOptions{})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	return user, nil
}

func (c *UserController) Reconcile(ctx context.Context, obj runtime.Object) error {
	currentUser := obj.(*miniov1alpha1.MinIOUser)
	minioUser := currentUser.DeepCopy()

	s, err := metav1.LabelSelectorAsSelector(&minioUser.Spec.Selector)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	instances, err := c.instanceLister.MinIOInstances(minioUser.Namespace).List(s)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if len(instances) == 0 {
		klog.V(4).Infof("%s not found", metav1.FormatLabelSelector(&minioUser.Spec.Selector))
		return nil
	}
	if len(instances) > 1 {
		return errors.New("found some instances")
	}

	for _, instance := range instances {
		creds, err := c.secretLister.Secrets(instance.Namespace).Get(instance.Spec.CredsSecret.Name)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		instanceEndpoint, forwarder, err := c.getMinIOInstanceEndpoint(ctx, instance)
		if err != nil {
			return xerrors.Errorf(": %w", err)
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
			return xerrors.Errorf(": %w", err)
		}
		if c.transport != nil {
			adminClient.SetCustomTransport(c.transport)
		}

		secret, err := c.ensureUser(ctx, adminClient, minioUser)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		if minioUser.Spec.Path != "" && !minioUser.Status.Vault {
			if err := c.saveAccessKeyToVault(minioUser, secret); err != nil {
				return xerrors.Errorf(": %w", err)
			}
		}
	}

	if err := c.setStatus(minioUser); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	if !reflect.DeepEqual(minioUser.Status, currentUser.Status) {
		_, err = c.mClient.MinioV1alpha1().MinIOUsers(minioUser.Namespace).UpdateStatus(
			ctx,
			minioUser,
			metav1.UpdateOptions{},
		)
		if err != nil {
			return xerrors.Errorf(": %w", err)
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
		return xerrors.Errorf(": %w", err)
	}
	user.Status.Ready = true
	user.Status.AccessKey = string(secret.Data["accesskey"])
	if user.Spec.Path != "" {
		user.Status.Vault = true
	}

	return nil
}

func (c *UserController) ensureUser(ctx context.Context, client *madmin.AdminClient, user *miniov1alpha1.MinIOUser) (*corev1.Secret, error) {
	secret, err := c.secretLister.Secrets(user.Namespace).Get(fmt.Sprintf("%s-accesskey", user.Name))
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, xerrors.Errorf(": %w", err)
	}
	if err == nil {
		return secret, nil
	}

	accessKey := stringsutil.RandomString(accessKeyLength)
	secretKey := stringsutil.RandomString(secretKeyLength)
	if err := client.AddUser(ctx, accessKey, secretKey); err != nil {
		return nil, xerrors.Errorf(": %w", err)
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
	controllerutil.SetOwner(secret, user, scheme.Scheme)
	secret, err = c.coreClient.CoreV1().Secrets(user.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return secret, nil
}

func (c *UserController) saveAccessKeyToVault(user *miniov1alpha1.MinIOUser, secret *corev1.Secret) error {
	data := map[string]interface{}{
		"data": map[string]string{
			"accesskey": string(secret.Data["accesskey"]),
			"secretkey": string(secret.Data["secretkey"]),
		},
	}

	_, err := c.vaultClient.Logical().Write(
		"/"+path.Join(user.Spec.MountPath, "data", user.Spec.Path),
		data,
	)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (c *UserController) Finalize(ctx context.Context, obj runtime.Object) error {
	minioUser := obj.(*miniov1alpha1.MinIOUser)

	s, err := metav1.LabelSelectorAsSelector(&minioUser.Spec.Selector)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	instances, err := c.instanceLister.MinIOInstances(minioUser.Namespace).List(s)
	if err != nil {
		return err
	}

	for _, instance := range instances {
		creds, err := c.secretLister.Secrets(instance.Namespace).Get(instance.Spec.CredsSecret.Name)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		instanceEndpoint, forwarder, err := c.getMinIOInstanceEndpoint(ctx, instance)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		if forwarder != nil {
			defer forwarder.Close()
		}

		secret, err := c.secretLister.Secrets(minioUser.Namespace).Get(fmt.Sprintf("%s-accesskey", minioUser.Name))
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		adminClient, err := madmin.New(instanceEndpoint, string(creds.Data["accesskey"]), string(creds.Data["secretkey"]), false)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		if err := adminClient.RemoveUser(ctx, string(secret.Data["accesskey"])); err != nil {
			return xerrors.Errorf(": %w", err)
		}
		klog.V(4).Infof("Remove minio user %s", minioUser.Name)

		if err := c.coreClient.CoreV1().Secrets(secret.Namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{}); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	minioUser.Finalizers = removeString(minioUser.Finalizers, minIOUserControllerFinalizerName)

	_, err = c.mClient.MinioV1alpha1().MinIOUsers(minioUser.Namespace).Update(ctx, minioUser, metav1.UpdateOptions{})
	return err
}

func (c *UserController) getMinIOInstanceEndpoint(
	ctx context.Context,
	instance *miniocontrollerv1beta1.MinIOInstance,
) (string, *portforward.PortForwarder, error) {
	svc, err := c.serviceLister.Services(instance.Namespace).Get(fmt.Sprintf("%s-hl-svc", instance.Name))
	if err != nil {
		return "", nil, xerrors.Errorf(": %w", err)
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
		return nil, errors.New("all pods are not running yet")
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
