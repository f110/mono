package harbor

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"time"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"
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

	"go.f110.dev/mono/go/pkg/api/harborv1alpha1"
	"go.f110.dev/mono/go/pkg/harbor"
	"go.f110.dev/mono/go/pkg/k8s/client"
	"go.f110.dev/mono/go/pkg/k8s/controllers/controllerutil"
)

const (
	harborProjectControllerFinalizerName = "harbor-project-controller.harbor.f110.dev/finalizer"
)

type ProjectController struct {
	*controllerutil.ControllerBase

	config     *rest.Config
	coreClient kubernetes.Interface
	hpClient   *client.HarborV1alpha1
	hpLister   *client.HarborV1alpha1HarborProjectLister

	harborService     *corev1.Service
	adminPassword     string
	registryName      string
	transport         http.RoundTripper
	runOutsideCluster bool
}

var _ controllerutil.Controller = &ProjectController{}

func NewProjectController(
	ctx context.Context,
	coreClient kubernetes.Interface,
	apiClient *client.Set,
	cfg *rest.Config,
	factory *client.InformerFactory,
	harborNamespace,
	harborName,
	adminSecretName,
	coreConfigMapName string,
	runOutsideCluster bool,
) (*ProjectController, error) {
	adminSecret, err := coreClient.CoreV1().Secrets(harborNamespace).Get(ctx, adminSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	svc, err := coreClient.CoreV1().Services(harborNamespace).Get(ctx, harborName, metav1.GetOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	cm, err := coreClient.CoreV1().ConfigMaps(harborNamespace).Get(ctx, coreConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	registryUrl, err := url.Parse(cm.Data["EXT_ENDPOINT"])
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	informers := client.NewHarborV1alpha1Informer(factory.Cache(), apiClient.HarborV1alpha1, metav1.NamespaceAll, 30*time.Second)
	hpInformer := informers.HarborProjectInformer()

	c := &ProjectController{
		config:            cfg,
		coreClient:        coreClient,
		hpClient:          apiClient.HarborV1alpha1,
		hpLister:          informers.HarborProjectLister(),
		harborService:     svc,
		adminPassword:     string(adminSecret.Data["HARBOR_ADMIN_PASSWORD"]),
		registryName:      registryUrl.Hostname(),
		runOutsideCluster: runOutsideCluster,
	}
	c.ControllerBase = controllerutil.NewBase(
		"harbor-project-controller",
		c,
		coreClient,
		[]cache.SharedIndexInformer{hpInformer},
		[]cache.SharedIndexInformer{hpInformer},
		[]string{harborProjectControllerFinalizerName},
	)

	return c, nil
}

func (c *ProjectController) Reconcile(ctx context.Context, obj runtime.Object) error {
	currentHP := obj.(*harborv1alpha1.HarborProject)
	harborProject := currentHP.DeepCopy()

	harborClient, err := c.harborClient(ctx)
	if err != nil {
		return xerrors.WithStack(err)
	}

	if ok, err := harborClient.ExistProject(harborProject.Name); err == nil && !ok {
		if err := c.createProject(harborProject, harborClient); err != nil {
			return xerrors.WithStack(err)
		}
	} else if err != nil {
		return xerrors.WithStack(err)
	}

	projects, err := harborClient.ListProjects()
	if err != nil {
		return xerrors.WithStack(err)
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
		_, err = c.hpClient.UpdateStatusHarborProject(ctx, harborProject, metav1.UpdateOptions{})
		if err != nil {
			return xerrors.WithStack(err)
		}
	}

	return nil
}

func (c *ProjectController) harborClient(ctx context.Context) (*harbor.Harbor, error) {
	harborHost := fmt.Sprintf("http://%s.%s.svc", c.harborService.Name, c.harborService.Namespace)
	if c.runOutsideCluster {
		pf, err := c.portForward(ctx, c.harborService, 8080)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		defer pf.Close()

		ports, err := pf.GetPorts()
		if err != nil {
			return nil, xerrors.WithStack(err)
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
		return xerrors.WithStack(err)
	}

	return nil
}

func (c *ProjectController) Finalize(ctx context.Context, obj runtime.Object) error {
	hp := obj.(*harborv1alpha1.HarborProject)

	harborClient, err := c.harborClient(ctx)
	if err != nil {
		return xerrors.WithStack(err)
	}

	err = harborClient.DeleteProject(hp.Status.ProjectId)
	if err != nil {
		return xerrors.WithStack(err)
	}
	hp.Finalizers = removeString(hp.Finalizers, harborProjectControllerFinalizerName)
	_, err = c.hpClient.UpdateHarborProject(ctx, hp, metav1.UpdateOptions{})
	return xerrors.WithStack(err)
}

func (c *ProjectController) portForward(ctx context.Context, svc *corev1.Service, port int) (*portforward.PortForwarder, error) {
	selector := labels.SelectorFromSet(svc.Spec.Selector)
	podList, err := c.coreClient.CoreV1().Pods(svc.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, xerrors.WithStack(err)
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
		return nil, xerrors.WithStack(err)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req.URL())

	readyCh := make(chan struct{})
	pf, err := portforward.New(dialer, []string{fmt.Sprintf(":%d", port)}, context.Background().Done(), readyCh, nil, nil)
	if err != nil {
		return nil, xerrors.WithStack(err)
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
		return nil, xerrors.New("timed out")
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
		return nil, xerrors.WithStack(err)
	}

	hp, err := c.hpLister.Get(namespace, name)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return hp, nil
}

func (c *ProjectController) UpdateObject(ctx context.Context, obj runtime.Object) (runtime.Object, error) {
	hp := obj.(*harborv1alpha1.HarborProject)

	hp, err := c.hpClient.UpdateHarborProject(ctx, hp, metav1.UpdateOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
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
