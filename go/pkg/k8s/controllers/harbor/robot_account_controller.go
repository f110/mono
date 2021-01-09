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
	harborRobotAccountControllerFinalizerName = "harbor-project-controller.harbor.f110.dev/robot-account-finalizer"
)

type HarborRobotAccountController struct {
	config             *rest.Config
	coreClient         *kubernetes.Clientset
	hClient            clientset.Interface
	hraLister          hpLister.HarborRobotAccountLister
	hraListerHasSynced cache.InformerSynced

	queue workqueue.RateLimitingInterface

	harborService     *corev1.Service
	adminPassword     string
	runOutsideCluster bool
}

// +kubebuilder:rbac:groups=harbor.f110.dev,resources=harborrobotaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=harbor.f110.dev,resources=harborrobotaccounts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=*,resources=pods;services,verbs=get
// +kubebuilder:rbac:groups=*,resources=secrets,verbs=get;create;update;delete
// +kubebuilder:rbac:groups=*,resources=pods/portforward,verbs=get;list;create

func NewHarborRobotAccountController(ctx context.Context, coreClient *kubernetes.Clientset, cfg *rest.Config, sharedInformerFactory informers.SharedInformerFactory, harborNamespace, harborName, adminSecretName string, runOutsideCluster bool) (*HarborRobotAccountController, error) {
	adminSecret, err := coreClient.CoreV1().Secrets(harborNamespace).Get(ctx, adminSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	svc, err := coreClient.CoreV1().Services(harborNamespace).Get(ctx, harborName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	hClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	hraInformer := sharedInformerFactory.Harbor().V1alpha1().HarborRobotAccounts()

	c := &HarborRobotAccountController{
		config:             cfg,
		coreClient:         coreClient,
		hClient:            hClient,
		hraLister:          hraInformer.Lister(),
		hraListerHasSynced: hraInformer.Informer().HasSynced,
		queue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "HarborRobotAccount"),
		harborService:      svc,
		adminPassword:      string(adminSecret.Data["HARBOR_ADMIN_PASSWORD"]),
		runOutsideCluster:  runOutsideCluster,
	}

	hraInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addHarborRobotAccount,
		UpdateFunc: c.updateHarborRobotAccount,
		DeleteFunc: c.deleteHarborRobotAccount,
	})

	return c, nil
}

func (c *HarborRobotAccountController) syncHarborRobotAccount(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	currentHRA, err := c.hClient.HarborV1alpha1().HarborRobotAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		klog.V(4).Infof("%s/%s is not found", namespace, name)
		return nil
	} else if err != nil {
		return err
	}
	harborRobotAccount := currentHRA.DeepCopy()

	if harborRobotAccount.DeletionTimestamp.IsZero() {
		if !containsString(harborRobotAccount.Finalizers, harborRobotAccountControllerFinalizerName) {
			harborRobotAccount.ObjectMeta.Finalizers = append(harborRobotAccount.ObjectMeta.Finalizers, harborRobotAccountControllerFinalizerName)
			_, err = c.hClient.HarborV1alpha1().HarborRobotAccounts(harborRobotAccount.Namespace).Update(ctx, harborRobotAccount, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}

	project, err := c.hClient.HarborV1alpha1().HarborProjects(harborRobotAccount.Spec.ProjectNamespace).Get(ctx, harborRobotAccount.Spec.ProjectName, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		klog.Infof("%s/%s is not found", harborRobotAccount.Spec.ProjectNamespace, harborRobotAccount.Spec.ProjectName)
		return nil
	}
	if project.Status.ProjectId == 0 {
		klog.Infof("%s/%s is not shipped yet", project.Namespace, project.Name)
		return nil
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
	if !harborRobotAccount.DeletionTimestamp.IsZero() {
		return c.finalizeHarborRobotAccount(ctx, harborClient, project.Status.ProjectId, harborRobotAccount)
	}

	if harborRobotAccount.Status.Ready {
		return nil
	}

	accounts, err := harborClient.GetRobotAccounts(project.Status.ProjectId)
	if err != nil {
		return err
	}
	created := false
	for _, v := range accounts {
		if strings.HasSuffix(v.Name, "$"+harborRobotAccount.Name) {
			klog.V(4).Infof("%s is already exist", v.Name)
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

func (c *HarborRobotAccountController) finalizeHarborRobotAccount(ctx context.Context, client *harbor.Harbor, projectId int, ra *harborv1alpha1.HarborRobotAccount) error {
	if ra.Status.Ready == false {
		klog.V(4).Infof("Skip finalize project because the project still not shipped: %s", ra.Name)
		ra.Finalizers = removeString(ra.Finalizers, harborRobotAccountControllerFinalizerName)
		return nil
	}

	if err := client.DeleteRobotAccount(projectId, ra.Status.RobotId); err != nil {
		return err
	}

	ra.Finalizers = removeString(ra.Finalizers, harborRobotAccountControllerFinalizerName)
	_, err := c.hClient.HarborV1alpha1().HarborRobotAccounts(ra.Namespace).Update(ctx, ra, metav1.UpdateOptions{})
	return err
}

func (c *HarborRobotAccountController) portForward(ctx context.Context, svc *corev1.Service, port int) (*portforward.PortForwarder, error) {
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

func (c *HarborRobotAccountController) Run(ctx context.Context, workers int) {
	defer c.queue.ShutDown()

	if !cache.WaitForCacheSync(ctx.Done(), c.hraListerHasSynced) {
		klog.Error("Failed to sync informer caches")
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, ctx.Done())
	}

	klog.Info("Start workers of HarborRobotAccountController")
	<-ctx.Done()
	klog.Info("Shutdown workers")
}

func (c *HarborRobotAccountController) worker() {
	defer klog.V(4).Info("Finish worker")

	for c.processNextItem() {
	}
}

func (c *HarborRobotAccountController) processNextItem() bool {
	obj, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	err := func() error {
		defer c.queue.Done(obj)
		defer cancelFunc()

		err := c.syncHarborRobotAccount(ctx, obj.(string))
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

func (c *HarborRobotAccountController) enqueue(hra *harborv1alpha1.HarborRobotAccount) {
	if key, err := cache.MetaNamespaceKeyFunc(hra); err != nil {
		return
	} else {
		c.queue.Add(key)
	}
}

func (c *HarborRobotAccountController) addHarborRobotAccount(obj interface{}) {
	hra := obj.(*harborv1alpha1.HarborRobotAccount)

	c.enqueue(hra)
}

func (c *HarborRobotAccountController) updateHarborRobotAccount(old, cur interface{}) {
	oldHRA := old.(*harborv1alpha1.HarborRobotAccount)
	curHRA := cur.(*harborv1alpha1.HarborRobotAccount)

	if oldHRA.UID != curHRA.UID {
		if key, err := cache.MetaNamespaceKeyFunc(oldHRA); err != nil {
			return
		} else {
			c.deleteHarborRobotAccount(cache.DeletedFinalStateUnknown{Key: key, Obj: oldHRA})
		}
	}

	c.enqueue(curHRA)
}

func (c *HarborRobotAccountController) deleteHarborRobotAccount(obj interface{}) {
	hra, ok := obj.(*harborv1alpha1.HarborRobotAccount)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		hra, ok = tombstone.Obj.(*harborv1alpha1.HarborRobotAccount)
		if !ok {
			return
		}
	}

	c.enqueue(hra)
}
