package harbor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
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
)

const (
	harborProjectControllerFinalizerName = "harbor-project-controller.harbor.f110.dev/finalizer"
)

type ProjectController struct {
	*controllerutil.ControllerBase

	config     *rest.Config
	coreClient kubernetes.Interface
	hpClient   clientset.Interface
	hpLister   hpLister.HarborProjectLister

	harborService     *corev1.Service
	adminPassword     string
	registryName      string
	transport         http.RoundTripper
	runOutsideCluster bool
}

var _ controllerutil.Controller = &ProjectController{}

// +kubebuilder:rbac:groups=harbor.f110.dev,resources=harborprojects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=harbor.f110.dev,resources=harborprojects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=*,resources=pods;secrets;services;configmaps,verbs=get
// +kubebuilder:rbac:groups=*,resources=pods/portforward,verbs=get;list;create

func NewProjectController(
	ctx context.Context,
	coreClient kubernetes.Interface,
	client clientset.Interface,
	cfg *rest.Config,
	sharedInformerFactory informers.SharedInformerFactory,
	harborNamespace,
	harborName,
	adminSecretName,
	coreConfigMapName string,
	runOutsideCluster bool,
) (*ProjectController, error) {
	adminSecret, err := coreClient.CoreV1().Secrets(harborNamespace).Get(ctx, adminSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	svc, err := coreClient.CoreV1().Services(harborNamespace).Get(ctx, harborName, metav1.GetOptions{})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	cm, err := coreClient.CoreV1().ConfigMaps(harborNamespace).Get(ctx, coreConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	registryUrl, err := url.Parse(cm.Data["EXT_ENDPOINT"])
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	hpInformer := sharedInformerFactory.Harbor().V1alpha1().HarborProjects()

	c := &ProjectController{
		config:            cfg,
		coreClient:        coreClient,
		hpClient:          client,
		hpLister:          hpInformer.Lister(),
		harborService:     svc,
		adminPassword:     string(adminSecret.Data["HARBOR_ADMIN_PASSWORD"]),
		registryName:      registryUrl.Hostname(),
		runOutsideCluster: runOutsideCluster,
	}
	c.ControllerBase = controllerutil.NewBase(
		"harbor-project-controller",
		c,
		coreClient,
		[]cache.SharedIndexInformer{hpInformer.Informer()},
		[]cache.SharedIndexInformer{hpInformer.Informer()},
		[]string{harborProjectControllerFinalizerName},
	)

	return c, nil
}

func (c *ProjectController) Reconcile(ctx context.Context, obj runtime.Object) error {
	currentHP := obj.(*harborv1alpha1.HarborProject)
	harborProject := currentHP.DeepCopy()

	harborClient, err := c.harborClient(ctx)
	if err != nil {
		return err
	}

	if ok, err := harborClient.ExistProject(harborProject.Name); err == nil && !ok {
		if err := c.createProject(harborProject, harborClient); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	projects, err := harborClient.ListProjects()
	if err != nil {
		return err
	}
	for _, v := range projects {
		if v.Name == harborProject.Name {
			harborProject.Status.ProjectId = v.Id
			break
		}
	}

	harborProject.Status.Ready = true
	harborProject.Status.Registry = c.registryName

	if !reflect.DeepEqual(harborProject.Status, currentHP.Status) {
		_, err = c.hpClient.HarborV1alpha1().HarborProjects(currentHP.Namespace).UpdateStatus(ctx, harborProject, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *ProjectController) harborClient(ctx context.Context) (*harbor.Harbor, error) {
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

func (c *ProjectController) createProject(currentHP *harborv1alpha1.HarborProject, client *harbor.Harbor) error {
	newProject := &harbor.NewProjectRequest{ProjectName: currentHP.Name}
	if currentHP.Spec.Public {
		newProject.Metadata.Public = "true"
	}
	if err := client.NewProject(newProject); err != nil {
		return err
	}

	return nil
}

func (c *ProjectController) Finalize(ctx context.Context, obj runtime.Object) error {
	hp := obj.(*harborv1alpha1.HarborProject)

	harborClient, err := c.harborClient(ctx)
	if err != nil {
		return err
	}

	err = harborClient.DeleteProject(hp.Status.ProjectId)
	if err != nil {
		return err
	}
	hp.Finalizers = removeString(hp.Finalizers, harborProjectControllerFinalizerName)
	_, err = c.hpClient.HarborV1alpha1().HarborProjects(hp.Namespace).Update(ctx, hp, metav1.UpdateOptions{})
	return err
}

func (c *ProjectController) portForward(ctx context.Context, svc *corev1.Service, port int) (*portforward.PortForwarder, error) {
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

func (c *ProjectController) ObjectToKeys(obj interface{}) []string {
	hp, ok := obj.(*harborv1alpha1.HarborProject)
	if !ok {
		return nil
	}
	key, err := cache.MetaNamespaceKeyFunc(hp)
	if err != nil {
		return nil
	}

	return []string{key}
}

func (c *ProjectController) GetObject(key string) (runtime.Object, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	hp, err := c.hpLister.HarborProjects(namespace).Get(name)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	return hp, nil
}

func (c *ProjectController) UpdateObject(ctx context.Context, obj runtime.Object) (runtime.Object, error) {
	hp := obj.(*harborv1alpha1.HarborProject)

	hp, err := c.hpClient.HarborV1alpha1().HarborProjects(hp.Namespace).Update(ctx, hp, metav1.UpdateOptions{})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	return hp, nil
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
