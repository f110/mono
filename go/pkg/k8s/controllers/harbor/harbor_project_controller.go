package harbor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	harborv1alpha1 "go.f110.dev/mono/go/pkg/api/harbor/v1alpha1"
	"go.f110.dev/mono/go/pkg/harbor"
	clientset "go.f110.dev/mono/go/pkg/k8s/client/versioned"
	informers "go.f110.dev/mono/go/pkg/k8s/informers/externalversions"
	hpLister "go.f110.dev/mono/go/pkg/k8s/listers/harbor/v1alpha1"
)

const (
	harborProjectControllerFinalizerName = "harbor-project-controller.harbor.f110.dev/finalizer"
)

type HarborProjectController struct {
	config            *rest.Config
	coreClient        *kubernetes.Clientset
	hpClient          clientset.Interface
	hpLister          hpLister.HarborProjectLister
	hpListerHasSynced cache.InformerSynced

	queue workqueue.RateLimitingInterface

	harborService     *corev1.Service
	adminPassword     string
	registryName      string
	runOutsideCluster bool
}

// +kubebuilder:rbac:groups=harbor.f110.dev,resources=harborprojects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=harbor.f110.dev,resources=harborprojects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=*,resources=pods;secrets;services;configmaps,verbs=get
// +kubebuilder:rbac:groups=*,resources=pods/portforward,verbs=get;list;create

func NewHarborProjectController(ctx context.Context, coreClient *kubernetes.Clientset, cfg *rest.Config, sharedInformerFactory informers.SharedInformerFactory, harborNamespace, harborName, adminSecretName, coreConfigMapName string, runOutsideCluster bool) (*HarborProjectController, error) {
	adminSecret, err := coreClient.CoreV1().Secrets(harborNamespace).Get(ctx, adminSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	svc, err := coreClient.CoreV1().Services(harborNamespace).Get(ctx, harborName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	cm, err := coreClient.CoreV1().ConfigMaps(harborNamespace).Get(ctx, coreConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	registryUrl, err := url.Parse(cm.Data["EXT_ENDPOINT"])
	if err != nil {
		return nil, err
	}

	hpClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	hpInformer := sharedInformerFactory.Harbor().V1alpha1().HarborProjects()

	c := &HarborProjectController{
		config:            cfg,
		coreClient:        coreClient,
		hpClient:          hpClient,
		hpLister:          hpInformer.Lister(),
		hpListerHasSynced: hpInformer.Informer().HasSynced,
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "HarborProject"),
		harborService:     svc,
		adminPassword:     string(adminSecret.Data["HARBOR_ADMIN_PASSWORD"]),
		registryName:      registryUrl.Hostname(),
		runOutsideCluster: runOutsideCluster,
	}

	hpInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addHarborProject,
		UpdateFunc: c.updateHarborProject,
		DeleteFunc: c.deleteHarborProject,
	})

	return c, nil
}

func (c *HarborProjectController) syncHarborProject(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	currentHP, err := c.hpClient.HarborV1alpha1().HarborProjects(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		klog.V(4).Infof("%s/%s is not found", namespace, name)
		return nil
	} else if err != nil {
		return err
	}
	harborProject := currentHP.DeepCopy()

	if harborProject.DeletionTimestamp.IsZero() {
		if !containsString(harborProject.Finalizers, harborProjectControllerFinalizerName) {
			harborProject.ObjectMeta.Finalizers = append(harborProject.ObjectMeta.Finalizers, harborProjectControllerFinalizerName)
			_, err = c.hpClient.HarborV1alpha1().HarborProjects(harborProject.Namespace).Update(ctx, harborProject, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}

	harborHost := fmt.Sprintf("http://%s.%s.svc", c.harborService.Name, c.harborService.Namespace)
	if c.runOutsideCluster {
		pf, err := c.portForward(ctx, c.harborService, 8080)
		if err != nil {
			return err
		}
		defer pf.Close()

		ports, err := pf.GetPorts()
		if err != nil {
			return err
		}
		harborHost = fmt.Sprintf("http://127.0.0.1:%d", ports[0].Local)
	}
	harborClient := harbor.New(harborHost, "admin", c.adminPassword)

	// Object has been deleted
	if !harborProject.DeletionTimestamp.IsZero() {
		return c.finalizeHarborProject(ctx, harborClient, harborProject)
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

func (c *HarborProjectController) createProject(currentHP *harborv1alpha1.HarborProject, client *harbor.Harbor) error {
	newProject := &harbor.NewProjectRequest{ProjectName: currentHP.Name}
	if currentHP.Spec.Public {
		newProject.Metadata.Public = "true"
	}
	if err := client.NewProject(newProject); err != nil {
		return err
	}

	return nil
}

func (c *HarborProjectController) finalizeHarborProject(ctx context.Context, client *harbor.Harbor, hp *harborv1alpha1.HarborProject) error {
	if hp.Status.Ready == false {
		klog.V(4).Infof("Skip finalize project because the project still not shipped: %s", hp.Name)
		hp.Finalizers = removeString(hp.Finalizers, harborProjectControllerFinalizerName)
		return nil
	}

	err := client.DeleteProject(hp.Status.ProjectId)
	if err != nil {
		return err
	}
	hp.Finalizers = removeString(hp.Finalizers, harborProjectControllerFinalizerName)
	_, err = c.hpClient.HarborV1alpha1().HarborProjects(hp.Namespace).Update(ctx, hp, metav1.UpdateOptions{})
	return err
}

func (c *HarborProjectController) portForward(ctx context.Context, svc *corev1.Service, port int) (*portforward.PortForwarder, error) {
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
			klog.Error(err)
			switch v := err.(type) {
			case *apierrors.StatusError:
				klog.Info(v)
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

func (c *HarborProjectController) Run(ctx context.Context, workers int) {
	defer c.queue.ShutDown()

	if !cache.WaitForCacheSync(ctx.Done(), c.hpListerHasSynced) {
		klog.Error("Failed to sync informer caches")
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, ctx.Done())
	}

	klog.Info("Start workers of HarborProjectController")
	<-ctx.Done()
	klog.Info("Shutdown workers")
}

func (c *HarborProjectController) worker() {
	defer klog.V(4).Info("Finish worker")

	for c.processNextItem() {
	}
}

func (c *HarborProjectController) processNextItem() bool {
	obj, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	err := func() error {
		defer c.queue.Done(obj)
		defer cancelFunc()

		err := c.syncHarborProject(ctx, obj.(string))
		if err != nil {
			c.queue.AddRateLimited(obj)
			return err
		}

		c.queue.Forget(obj)
		return nil
	}()
	if err != nil {
		klog.Info(err)
		return true
	}

	return true
}

func (c *HarborProjectController) enqueue(hp *harborv1alpha1.HarborProject) {
	if key, err := cache.MetaNamespaceKeyFunc(hp); err != nil {
		return
	} else {
		c.queue.Add(key)
	}
}

func (c *HarborProjectController) addHarborProject(obj interface{}) {
	hp := obj.(*harborv1alpha1.HarborProject)

	c.enqueue(hp)
}

func (c *HarborProjectController) updateHarborProject(old, cur interface{}) {
	oldHP := old.(*harborv1alpha1.HarborProject)
	curHP := cur.(*harborv1alpha1.HarborProject)

	if oldHP.UID != curHP.UID {
		if key, err := cache.MetaNamespaceKeyFunc(oldHP); err != nil {
			return
		} else {
			c.deleteHarborProject(cache.DeletedFinalStateUnknown{Key: key, Obj: oldHP})
		}
	}

	c.enqueue(curHP)
}

func (c *HarborProjectController) deleteHarborProject(obj interface{}) {
	hp, ok := obj.(*harborv1alpha1.HarborProject)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		hp, ok = tombstone.Obj.(*harborv1alpha1.HarborProject)
		if !ok {
			return
		}
	}

	c.enqueue(hp)
}

func containsString(v []string, s string) bool {
	for _, item := range v {
		if item == s {
			return true
		}
	}

	return false
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
