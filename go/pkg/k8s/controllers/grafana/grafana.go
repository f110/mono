package grafana

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/xerrors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	grafanav1alpha1 "go.f110.dev/mono/go/pkg/api/grafana/v1alpha1"
	"go.f110.dev/mono/go/pkg/collections/set"
	"go.f110.dev/mono/go/pkg/grafana"
	clientset "go.f110.dev/mono/go/pkg/k8s/client/versioned"
	"go.f110.dev/mono/go/pkg/k8s/controllers/controllerutil"
	informers "go.f110.dev/mono/go/pkg/k8s/informers/externalversions"
	listers "go.f110.dev/mono/go/pkg/k8s/listers/grafana/v1alpha1"
	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/parallel"
)

const (
	grafanaAdminControllerFinalizerName = "grafana-admin-controller.grafana.f110.dev/finalizer"
)

type AdminController struct {
	*controllerutil.ControllerBase

	client clientset.Interface

	secretLister  corev1listers.SecretLister
	serviceLister corev1listers.ServiceLister
	appLister     listers.GrafanaLister
	userLister    listers.GrafanaUserLister

	queue *controllerutil.WorkQueue
}

func NewAdminController(
	coreSharedInformerFactory kubeinformers.SharedInformerFactory,
	sharedInformerFactory informers.SharedInformerFactory,
	client clientset.Interface,
) (*AdminController, error) {
	secretInformer := coreSharedInformerFactory.Core().V1().Secrets()
	serviceInformer := coreSharedInformerFactory.Core().V1().Services()
	appInformer := sharedInformerFactory.Grafana().V1alpha1().Grafanas()
	userInformer := sharedInformerFactory.Grafana().V1alpha1().GrafanaUsers()

	a := &AdminController{
		ControllerBase: controllerutil.NewBase(),
		client:         client,
		secretLister:   secretInformer.Lister(),
		serviceLister:  serviceInformer.Lister(),
		appLister:      appInformer.Lister(),
		userLister:     userInformer.Lister(),
		queue:          controllerutil.NewWorkQueue(),
	}
	a.UseInformer(secretInformer.Informer())
	a.UseInformer(serviceInformer.Informer())
	a.UseInformer(appInformer.Informer())
	a.UseInformer(userInformer.Informer())

	appInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    a.addApp,
		UpdateFunc: a.updateApp,
		DeleteFunc: a.deleteApp,
	})

	userInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    a.addUser,
		UpdateFunc: a.updateUser,
		DeleteFunc: a.deleteUser,
	})

	return a, nil
}

func (a *AdminController) Run(ctx context.Context, workers int) {
	logger.Log.Info("Wait for informer caches to sync")
	if !a.WaitForSync(ctx) {
		return
	}

	sv := parallel.NewSupervisor(ctx)
	for i := 0; i < workers; i++ {
		sv.Add(a.worker)
	}

	<-ctx.Done()

	a.queue.Shutdown()
	sCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	sv.Shutdown(sCtx)
	cancel()
}

func (a *AdminController) syncGrafana(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	app, err := a.appLister.Grafanas(namespace).Get(name)
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	if app.DeletionTimestamp.IsZero() {
		if !containsString(app.Finalizers, grafanaAdminControllerFinalizerName) {
			app.ObjectMeta.Finalizers = append(app.ObjectMeta.Finalizers, grafanaAdminControllerFinalizerName)
			app, err = a.client.GrafanaV1alpha1().Grafanas(namespace).Update(ctx, app, metav1.UpdateOptions{})
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
		}
	}

	if !app.DeletionTimestamp.IsZero() {
		return a.finalizeGrafana(ctx, app)
	}

	sel, err := metav1.LabelSelectorAsSelector(&app.Spec.UserSelector)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	users, err := a.userLister.GrafanaUsers(namespace).List(sel)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	if err := a.ensureUsers(app, users); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (a *AdminController) finalizeGrafana(ctx context.Context, app *grafanav1alpha1.Grafana) error {
	return nil
}

func (a *AdminController) ensureUsers(app *grafanav1alpha1.Grafana, users []*grafanav1alpha1.GrafanaUser) error {
	secret, err := a.secretLister.Secrets(app.Namespace).Get(app.Spec.AdminPasswordSecret.Name)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	password, ok := secret.Data[app.Spec.AdminPasswordSecret.Key]
	if !ok {
		return xerrors.Errorf("%s is not found in %s", app.Spec.AdminPasswordSecret.Key, app.Spec.AdminPasswordSecret.Name)
	}
	svc, err := a.serviceLister.Services(app.Namespace).Get(app.Spec.Service.Name)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	grafanaClient := grafana.NewClient(fmt.Sprintf("http://%s.%s.svc:%d", svc.Name, app.Namespace, 3000), app.Spec.AdminUser, string(password))

	allUsers := make(map[string]*grafanav1alpha1.GrafanaUser)
	for _, v := range users {
		allUsers[v.Spec.Email] = v
	}

	currentUsers, err := grafanaClient.Users()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	currentUsersMap := make(map[string]*grafana.User)
	for _, v := range currentUsers {
		currentUsersMap[v.Email] = v
	}

	currentUsersSet := set.New()
	for _, v := range currentUsers {
		currentUsersSet.Add(v.Email)
	}
	idealUsersSet := set.New()
	for _, v := range users {
		idealUsersSet.Add(v.Spec.Email)
	}

	missingUsersSet := idealUsersSet.Diff(currentUsersSet)
	for _, v := range missingUsersSet.ToSlice() {
		email := v.(string)
		u := allUsers[email]
		s := strings.Split(u.Spec.Email, "@")
		name := s[0]
		logger.Log.Info("Add User", zap.String("email", u.Spec.Email), zap.String("name", name))
		if err := grafanaClient.AddUser(&grafana.User{Name: name, Login: name, Email: u.Spec.Email, Password: randomString(32)}); err != nil {
			logger.Log.Warn("Failed add user", zap.String("email", email), zap.Error(err))
		}
	}

	redundantUsersSet := currentUsersSet.Diff(idealUsersSet)
	for _, v := range redundantUsersSet.ToSlice() {
		email := v.(string)
		// Admin user should not delete
		if email == "admin@localhost" {
			continue
		}
		u := currentUsersMap[email]
		logger.Log.Info("Delete User", zap.Int("id", u.Id))
		if err := grafanaClient.DeleteUser(u.Id); err != nil {
			logger.Log.Warn("Failed delete user", zap.String("email", u.Email), zap.Int("id", u.Id), zap.Error(err))
		}
	}

	currentUsers, err = grafanaClient.Users()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	for _, v := range currentUsers {
		u, ok := allUsers[v.Email]
		if !ok {
			continue
		}
		if u.Spec.Admin != v.IsAdmin {
			logger.Log.Info("Change user permission", zap.Int("id", v.Id), zap.String("email", v.Email), zap.Bool("admin", u.Spec.Admin))
			if err := grafanaClient.ChangeUserPermission(v.Id, u.Spec.Admin); err != nil {
				logger.Log.Warn("Failed change user permission", zap.String("email", v.Email), zap.Bool("admin", v.IsAdmin))
			}
		}
	}

	return nil
}

func (a *AdminController) addApp(obj interface{}) {
	app := obj.(*grafanav1alpha1.Grafana)

	a.enqueue(app)
}

func (a *AdminController) updateApp(old, cur interface{}) {
	oldA := old.(*grafanav1alpha1.Grafana)
	curA := cur.(*grafanav1alpha1.Grafana)

	if oldA.UID != curA.UID {
		if key, err := cache.MetaNamespaceKeyFunc(oldA); err != nil {
			return
		} else {
			a.deleteUser(cache.DeletedFinalStateUnknown{Key: key, Obj: oldA})
		}
	}

	a.enqueue(curA)
}

func (a *AdminController) deleteApp(obj interface{}) {
	app, ok := obj.(*grafanav1alpha1.Grafana)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		app, ok = tombstone.Obj.(*grafanav1alpha1.Grafana)
		if !ok {
			return
		}
	}

	a.enqueue(app)
}

func (a *AdminController) addUser(obj interface{}) {
	u := obj.(*grafanav1alpha1.GrafanaUser)

	a.superordinateEnqueue(u)
}

func (a *AdminController) updateUser(old, cur interface{}) {
	oldU := old.(*grafanav1alpha1.GrafanaUser)
	curU := cur.(*grafanav1alpha1.GrafanaUser)

	if oldU.UID != curU.UID {
		if key, err := cache.MetaNamespaceKeyFunc(oldU); err != nil {
			return
		} else {
			a.deleteUser(cache.DeletedFinalStateUnknown{Key: key, Obj: oldU})
		}
	}

	a.superordinateEnqueue(curU)
}

func (a *AdminController) deleteUser(obj interface{}) {
	u, ok := obj.(*grafanav1alpha1.GrafanaUser)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		u, ok = tombstone.Obj.(*grafanav1alpha1.GrafanaUser)
		if !ok {
			return
		}
	}

	a.superordinateEnqueue(u)
}

func (a *AdminController) enqueue(app *grafanav1alpha1.Grafana) {
	if key, err := cache.MetaNamespaceKeyFunc(app); err != nil {
		return
	} else {
		a.queue.Add(key)
	}
}

func (a *AdminController) superordinateEnqueue(obj runtime.Object) {
	accessor, ok := obj.(metav1.Object)
	if !ok {
		return
	}

	apps, err := a.appLister.List(labels.Everything())
	if err != nil {
		return
	}

	for _, v := range apps {
		sel, err := metav1.LabelSelectorAsSelector(&v.Spec.UserSelector)
		if err != nil {
			continue
		}
		if sel.Matches(labels.Set(accessor.GetLabels())) {
			a.enqueue(v)
		}
	}
}

func (a *AdminController) worker(ctx context.Context) {
	for {
		var obj interface{}
		select {
		case v, ok := <-a.queue.Get():
			if !ok {
				return
			}
			obj = v
		}
		logger.Log.Debug("Get next queue", zap.Any("queue", obj))

		wCtx, cancelFunc := context.WithCancel(ctx)
		err := func(obj interface{}) error {
			defer a.queue.Done(obj)
			defer cancelFunc()

			err := a.syncGrafana(wCtx, obj.(string))
			if err != nil {
				a.queue.AddRateLimited(obj)
				return err
			}

			a.queue.Forget(obj)
			return nil
		}(obj)
		if err != nil {
			logger.Log.Warn("Reconcile has error", zap.Error(err))
		}
	}
}

func containsString(v []string, s string) bool {
	for _, item := range v {
		if item == s {
			return true
		}
	}

	return false
}

var chars = []rune("ABVDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz1234567890")

func randomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	return b.String()
}
