package harbor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"go.uber.org/zap"
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

	harborv1alpha1 "go.f110.dev/mono/go/pkg/api/harbor/v1alpha1"
	"go.f110.dev/mono/go/pkg/harbor"
	clientset "go.f110.dev/mono/go/pkg/k8s/client/versioned"
	"go.f110.dev/mono/go/pkg/k8s/controllers/controllerutil"
	informers "go.f110.dev/mono/go/pkg/k8s/informers/externalversions"
	hpLister "go.f110.dev/mono/go/pkg/k8s/listers/harbor/v1alpha1"
	"go.f110.dev/mono/go/pkg/logger"
)

const (
	harborRobotAccountControllerFinalizerName = "harbor-project-controller.harbor.f110.dev/robot-account-finalizer"
)

type RobotAccountController struct {
	*controllerutil.ControllerBase

	config     *rest.Config
	coreClient kubernetes.Interface
	hClient    clientset.Interface
	hraLister  hpLister.HarborRobotAccountLister

	harborService     *corev1.Service
	adminPassword     string
	transport         http.RoundTripper
	runOutsideCluster bool
}

// +kubebuilder:rbac:groups=harbor.f110.dev,resources=harborrobotaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=harbor.f110.dev,resources=harborrobotaccounts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=*,resources=pods;services,verbs=get
// +kubebuilder:rbac:groups=*,resources=secrets,verbs=get;create;update;delete
// +kubebuilder:rbac:groups=*,resources=pods/portforward,verbs=get;list;create

func NewRobotAccountController(
	ctx context.Context,
	coreClient kubernetes.Interface,
	client clientset.Interface,
	cfg *rest.Config,
	sharedInformerFactory informers.SharedInformerFactory,
	harborNamespace,
	harborName,
	adminSecretName string,
	runOutsideCluster bool,
) (*RobotAccountController, error) {
	adminSecret, err := coreClient.CoreV1().Secrets(harborNamespace).Get(ctx, adminSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	svc, err := coreClient.CoreV1().Services(harborNamespace).Get(ctx, harborName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	hraInformer := sharedInformerFactory.Harbor().V1alpha1().HarborRobotAccounts()

	c := &RobotAccountController{
		config:            cfg,
		coreClient:        coreClient,
		hClient:           client,
		hraLister:         hraInformer.Lister(),
		harborService:     svc,
		adminPassword:     string(adminSecret.Data["HARBOR_ADMIN_PASSWORD"]),
		runOutsideCluster: runOutsideCluster,
	}
	c.ControllerBase = controllerutil.NewBase(
		"harbor-robot-account-controller",
		c,
		coreClient,
		[]cache.SharedIndexInformer{hraInformer.Informer()},
		[]cache.SharedIndexInformer{},
		[]string{harborRobotAccountControllerFinalizerName},
	)

	return c, nil
}

func (c *RobotAccountController) ObjectToKeys(obj interface{}) []string {
	hra, ok := obj.(*harborv1alpha1.HarborRobotAccount)
	if !ok {
		return nil
	}
	key, err := cache.MetaNamespaceKeyFunc(hra)
	if err != nil {
		return nil
	}

	return []string{key}
}

func (c *RobotAccountController) GetObject(key string) (runtime.Object, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	hra, err := c.hraLister.HarborRobotAccounts(namespace).Get(name)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	return hra, nil
}

func (c *RobotAccountController) UpdateObject(ctx context.Context, obj runtime.Object) (runtime.Object, error) {
	hra := obj.(*harborv1alpha1.HarborRobotAccount)

	hra, err := c.hClient.HarborV1alpha1().HarborRobotAccounts(hra.Namespace).Update(ctx, hra, metav1.UpdateOptions{})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	return hra, nil
}

func (c *RobotAccountController) Reconcile(ctx context.Context, obj runtime.Object) error {
	currentHRA := obj.(*harborv1alpha1.HarborRobotAccount)
	harborRobotAccount := currentHRA.DeepCopy()

	project, err := c.getProject(ctx, harborRobotAccount)
	if err != nil {
		return err
	}
	if project.Status.ProjectId == 0 {
		c.Log().Info("Project is not shipped yet", logger.KubernetesObject("project", project))
		return nil
	}

	if harborRobotAccount.Status.Ready {
		return nil
	}

	harborClient, err := c.harborClient(ctx)
	if err != nil {
		return err
	}

	accounts, err := harborClient.GetRobotAccounts(project.Status.ProjectId)
	if err != nil {
		return err
	}
	created := false
	for _, v := range accounts {
		if strings.HasSuffix(v.Name, "$"+harborRobotAccount.Name) {
			c.Log().Info("Account already exists", zap.String("name", v.Name))
			created = true
		}
	}

	if !created {
		if err := c.createRobotAccount(ctx, harborClient, project, harborRobotAccount); err != nil {
			return err
		}
	}

	accounts, err = harborClient.GetRobotAccounts(project.Status.ProjectId)
	if err != nil {
		return err
	}
	for _, v := range accounts {
		if strings.HasSuffix(v.Name, "$"+harborRobotAccount.Name) {
			harborRobotAccount.Status.RobotId = v.Id
		}
	}

	harborRobotAccount.Status.Ready = true

	if !reflect.DeepEqual(harborRobotAccount.Status, currentHRA.Status) {
		_, err = c.hClient.HarborV1alpha1().HarborRobotAccounts(currentHRA.Namespace).UpdateStatus(ctx, harborRobotAccount, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *RobotAccountController) getProject(ctx context.Context, hra *harborv1alpha1.HarborRobotAccount) (*harborv1alpha1.HarborProject, error) {
	project, err := c.hClient.HarborV1alpha1().HarborProjects(hra.Spec.ProjectNamespace).Get(ctx, hra.Spec.ProjectName, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		c.Log().Info("Project not found", logger.KubernetesObject("project", hra))
		return nil, errors.New("project not found")
	} else if err != nil {
		return nil, err
	}

	return project, nil
}

func (c *RobotAccountController) harborClient(ctx context.Context) (*harbor.Harbor, error) {
	harborHost := fmt.Sprintf("http://%s.%s.svc", c.harborService.Name, c.harborService.Namespace)
	if c.runOutsideCluster {
		pf, err := c.portForward(ctx, c.harborService, 8080)
		if err != nil {
			return nil, err
		}
		defer pf.Close()

		ports, err := pf.GetPorts()
		if err != nil {
			return nil, err
		}
		harborHost = fmt.Sprintf("http://127.0.0.1:%d", ports[0].Local)
	}

	harborClient := harbor.New(harborHost, "admin", c.adminPassword)
	if c.transport != nil {
		harborClient.SetTransport(c.transport)
	}
	return harborClient, nil
}

func (c *RobotAccountController) createRobotAccount(ctx context.Context, client *harbor.Harbor, project *harborv1alpha1.HarborProject, robotAccount *harborv1alpha1.HarborRobotAccount) error {
	newAccount, err := client.CreateRobotAccount(
		project.Status.ProjectId,
		&harbor.NewRobotAccountRequest{
			Name: robotAccount.Name,
			Access: []harbor.Access{
				{Resource: fmt.Sprintf("/project/%d/repository", project.Status.ProjectId), Action: "push"},
				{Resource: fmt.Sprintf("/project/%d/repository", project.Status.ProjectId), Action: "pull"},
			},
		},
	)
	if err != nil {
		return err
	}

	dockerConfig := NewDockerConfig(project.Status.Registry, newAccount.Name, newAccount.Token)
	configBuf := new(bytes.Buffer)
	if err := json.NewEncoder(configBuf).Encode(dockerConfig); err != nil {
		return err
	}

	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            robotAccount.Spec.SecretName,
			Namespace:       robotAccount.Namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(robotAccount, harborv1alpha1.SchemeGroupVersion.WithKind("HarborRobotAccount"))},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": configBuf.Bytes(),
		},
	}
	_, err = c.coreClient.CoreV1().Secrets(newSecret.Namespace).Create(ctx, newSecret, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (c *RobotAccountController) Finalize(ctx context.Context, obj runtime.Object) error {
	hra := obj.(*harborv1alpha1.HarborRobotAccount)

	project, err := c.getProject(ctx, hra)
	if err != nil {
		return err
	}

	harborClient, err := c.harborClient(ctx)
	if err != nil {
		return err
	}

	if err := harborClient.DeleteRobotAccount(project.Status.ProjectId, hra.Status.RobotId); err != nil {
		return err
	}

	hra.Finalizers = removeString(hra.Finalizers, harborRobotAccountControllerFinalizerName)
	_, err = c.hClient.HarborV1alpha1().HarborRobotAccounts(hra.Namespace).Update(ctx, hra, metav1.UpdateOptions{})
	return err
}

func (c *RobotAccountController) portForward(ctx context.Context, svc *corev1.Service, port int) (*portforward.PortForwarder, error) {
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
			c.Log().Warn("Failed port forwarding", zap.Error(err))
			switch err.(type) {
			case *apierrors.StatusError:
				c.Log().Info("Got error", zap.Error(err))
			}
		}
	}()

	select {
	case <-readyCh:
	case <-time.After(5 * time.Second):
		return nil, errors.New("timed out")
	}

	return pf, nil
}
