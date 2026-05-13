package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"go.f110.dev/kubeproto/go/apis/corev1"
	"go.f110.dev/kubeproto/go/apis/metav1"
	"go.f110.dev/kubeproto/go/k8sclient"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/portforward"

	"go.f110.dev/mono/go/api/harborv1alpha1"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/harbor"
	"go.f110.dev/mono/go/k8s/client"
	"go.f110.dev/mono/go/k8s/controllers/controllerutil"
	"go.f110.dev/mono/go/logger"
)

const (
	harborRobotAccountControllerFinalizerName = "harbor-project-controller.harbor.f110.dev/robot-account-finalizer"
)

type HarborRobotAccountController struct {
	*controllerutil.ControllerBase

	config     *rest.Config
	coreClient *k8sclient.Set
	hClient    *client.HarborV1alpha1
	hraLister  *client.HarborV1alpha1HarborRobotAccountLister

	harborService     *corev1.Service
	adminPassword     string
	transport         http.RoundTripper
	runOutsideCluster bool
}

func NewHarborRobotAccountController(
	ctx context.Context,
	coreClient *k8sclient.Set,
	k8sClient kubernetes.Interface,
	apiClient *client.Set,
	cfg *rest.Config,
	factory *client.InformerFactory,
	harborNamespace,
	harborName,
	adminSecretName string,
	runOutsideCluster bool,
) (*HarborRobotAccountController, error) {
	adminSecret, err := coreClient.CoreV1.GetSecret(ctx, harborNamespace, adminSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	svc, err := coreClient.CoreV1.GetService(ctx, harborNamespace, harborName, metav1.GetOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	informers := client.NewHarborV1alpha1Informer(factory.Cache(), apiClient.HarborV1alpha1, metav1.NamespaceAll, 30*time.Second)
	hraInformer := informers.HarborRobotAccountInformer()

	c := &HarborRobotAccountController{
		config:            cfg,
		coreClient:        coreClient,
		hClient:           apiClient.HarborV1alpha1,
		hraLister:         informers.HarborRobotAccountLister(),
		harborService:     svc,
		adminPassword:     string(adminSecret.Data["HARBOR_ADMIN_PASSWORD"]),
		runOutsideCluster: runOutsideCluster,
	}
	c.ControllerBase = controllerutil.NewBase(
		"harbor-robot-account-controller",
		c,
		k8sClient,
		[]cache.SharedIndexInformer{hraInformer},
		[]cache.SharedIndexInformer{},
		[]string{harborRobotAccountControllerFinalizerName},
	)

	return c, nil
}

func (c *HarborRobotAccountController) ObjectToKeys(obj any) []string {
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

func (c *HarborRobotAccountController) GetObject(key string) (runtime.Object, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	hra, err := c.hraLister.Get(namespace, name)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return hra, nil
}

func (c *HarborRobotAccountController) UpdateObject(ctx context.Context, obj runtime.Object) (runtime.Object, error) {
	hra := obj.(*harborv1alpha1.HarborRobotAccount)

	hra, err := c.hClient.UpdateHarborRobotAccount(ctx, hra, metav1.UpdateOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return hra, nil
}

func (c *HarborRobotAccountController) Reconcile(ctx context.Context, obj runtime.Object) error {
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
		return xerrors.WithStack(err)
	}

	accounts, err := harborClient.GetRobotAccounts(project.Status.ProjectId)
	if err != nil {
		return xerrors.WithStack(err)
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
			return xerrors.WithStack(err)
		}
	}

	accounts, err = harborClient.GetRobotAccounts(project.Status.ProjectId)
	if err != nil {
		return xerrors.WithStack(err)
	}
	for _, v := range accounts {
		if strings.HasSuffix(v.Name, "$"+harborRobotAccount.Name) {
			harborRobotAccount.Status.RobotId = v.Id
		}
	}

	harborRobotAccount.Status.Ready = true

	if !reflect.DeepEqual(harborRobotAccount.Status, currentHRA.Status) {
		_, err = c.hClient.UpdateStatusHarborRobotAccount(ctx, harborRobotAccount, metav1.UpdateOptions{})
		if err != nil {
			return xerrors.WithStack(err)
		}
	}

	return nil
}

func (c *HarborRobotAccountController) getProject(ctx context.Context, hra *harborv1alpha1.HarborRobotAccount) (*harborv1alpha1.HarborProject, error) {
	project, err := c.hClient.GetHarborProject(ctx, hra.Spec.ProjectNamespace, hra.Spec.ProjectName, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		c.Log().Info("Project not found", logger.KubernetesObject("project", hra))
		return nil, xerrors.Define("project not found").WithStack()
	} else if err != nil {
		return nil, xerrors.WithStack(err)
	}

	return project, nil
}

func (c *HarborRobotAccountController) harborClient(ctx context.Context) (*harbor.Harbor, error) {
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

func (c *HarborRobotAccountController) createRobotAccount(ctx context.Context, client *harbor.Harbor, project *harborv1alpha1.HarborProject, robotAccount *harborv1alpha1.HarborRobotAccount) error {
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
		return xerrors.WithStack(err)
	}

	dockerConfig := NewDockerConfig(project.Status.Registry, newAccount.Name, newAccount.Token)
	configBuf := new(bytes.Buffer)
	if err := json.NewEncoder(configBuf).Encode(dockerConfig); err != nil {
		return xerrors.WithStack(err)
	}

	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            robotAccount.Spec.SecretName,
			Namespace:       robotAccount.Namespace,
			OwnerReferences: []metav1.OwnerReference{metav1.NewControllerRef(robotAccount.ObjectMeta, harborv1alpha1.SchemaGroupVersion.WithKind("HarborRobotAccount"))},
		},
		Type: corev1.SecretTypeKubernetesIoDockerconfigjson,
		Data: map[string][]byte{
			".dockerconfigjson": configBuf.Bytes(),
		},
	}
	_, err = c.coreClient.CoreV1.CreateSecret(ctx, newSecret, metav1.CreateOptions{})
	if err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (c *HarborRobotAccountController) Finalize(ctx context.Context, obj runtime.Object) error {
	hra := obj.(*harborv1alpha1.HarborRobotAccount)

	project, err := c.getProject(ctx, hra)
	if err != nil {
		return xerrors.WithStack(err)
	}

	harborClient, err := c.harborClient(ctx)
	if err != nil {
		return xerrors.WithStack(err)
	}

	if err := harborClient.DeleteRobotAccount(project.Status.ProjectId, hra.Status.RobotId); err != nil {
		return xerrors.WithStack(err)
	}

	hra.Finalizers = enumerable.Delete(hra.Finalizers, harborRobotAccountControllerFinalizerName)
	_, err = c.hClient.UpdateHarborRobotAccount(ctx, hra, metav1.UpdateOptions{})
	if err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (c *HarborRobotAccountController) portForward(ctx context.Context, svc *corev1.Service, port int) (*portforward.PortForwarder, error) {
	selector := labels.SelectorFromSet(svc.Spec.Selector)
	podList, err := c.coreClient.CoreV1.ListPod(ctx, svc.Namespace, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, xerrors.WithStack(err)
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

	pf, _, err := c.coreClient.CoreV1.PortForward(ctx, pod, port)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return pf, nil
}
