package grafana

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"reflect"
	"strings"
	"time"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"go.f110.dev/mono/go/api/grafanav1alpha1"
	"go.f110.dev/mono/go/pkg/collections/set"
	"go.f110.dev/mono/go/pkg/grafana"
	"go.f110.dev/mono/go/pkg/k8s/client"
	"go.f110.dev/mono/go/pkg/k8s/controllers/controllerutil"
)

const (
	grafanaUserControllerFinalizerName = "grafana-user-controller.grafana.f110.dev/finalizer"
)

type UserController struct {
	*controllerutil.ControllerBase

	client *client.GrafanaV1alpha1

	secretLister  corev1listers.SecretLister
	serviceLister corev1listers.ServiceLister
	appLister     *client.GrafanaV1alpha1GrafanaLister
	userLister    *client.GrafanaV1alpha1GrafanaUserLister

	// for testing
	transport http.RoundTripper
}

var _ controllerutil.Controller = &UserController{}

func NewUserController(
	coreSharedInformerFactory kubeinformers.SharedInformerFactory,
	factory *client.InformerFactory,
	coreClient kubernetes.Interface,
	apiClient *client.Set,
) (*UserController, error) {
	secretInformer := coreSharedInformerFactory.Core().V1().Secrets()
	serviceInformer := coreSharedInformerFactory.Core().V1().Services()
	grafanaInformers := client.NewGrafanaV1alpha1Informer(factory.Cache(), apiClient.GrafanaV1alpha1, metav1.NamespaceAll, 30*time.Second)
	appInformer := grafanaInformers.GrafanaInformer()
	userInformer := grafanaInformers.GrafanaUserInformer()

	a := &UserController{
		client:        apiClient.GrafanaV1alpha1,
		secretLister:  secretInformer.Lister(),
		serviceLister: serviceInformer.Lister(),
		appLister:     grafanaInformers.GrafanaLister(),
		userLister:    grafanaInformers.GrafanaUserLister(),
	}
	a.ControllerBase = controllerutil.NewBase(
		"grafana-user-controller",
		a,
		coreClient,
		[]cache.SharedIndexInformer{appInformer, userInformer},
		[]cache.SharedIndexInformer{secretInformer.Informer(), serviceInformer.Informer()},
		[]string{grafanaUserControllerFinalizerName},
	)

	return a, nil
}

func (u *UserController) ObjectToKeys(obj interface{}) []string {
	switch v := obj.(type) {
	case *grafanav1alpha1.Grafana:
		return []string{u.toKey(v)}
	case *grafanav1alpha1.GrafanaUser:
		apps, err := u.appLister.List(metav1.NamespaceAll, labels.Everything())
		if err != nil {
			return nil
		}

		keys := make([]string, 0)
		for _, app := range apps {
			sel, err := metav1.LabelSelectorAsSelector(&app.Spec.UserSelector)
			if err != nil {
				continue
			}
			if sel.Matches(labels.Set(v.GetLabels())) {
				key := u.toKey(app)
				if key != "" {
					keys = append(keys, key)
				}
			}
		}

		return keys
	default:
		return nil
	}
}

func (u *UserController) toKey(v *grafanav1alpha1.Grafana) string {
	key, err := cache.MetaNamespaceKeyFunc(v)
	if err != nil {
		return ""
	}
	return key
}

func (u *UserController) Reconcile(ctx context.Context, obj runtime.Object) error {
	app := obj.(*grafanav1alpha1.Grafana)

	sel, err := metav1.LabelSelectorAsSelector(&app.Spec.UserSelector)
	if err != nil {
		return xerrors.WithStack(err)
	}
	users, err := u.userLister.List(app.Namespace, sel)
	if err != nil {
		return xerrors.WithStack(err)
	}

	if err := u.ensureUsers(app, users); err != nil {
		return xerrors.WithStack(err)
	}

	newA := app.DeepCopy()
	newA.Status.ObservedGeneration = app.ObjectMeta.Generation
	if !reflect.DeepEqual(newA.Status, app.Status) {
		_, err = u.client.UpdateStatusGrafana(ctx, newA, metav1.UpdateOptions{})
		if err != nil {
			return controllerutil.WrapRetryError(err)
		}
	}
	return nil
}

func (u *UserController) Finalize(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (u *UserController) GetObject(key string) (runtime.Object, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	obj, err := u.appLister.Get(namespace, name)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return obj, nil
}

func (u *UserController) UpdateObject(ctx context.Context, obj runtime.Object) (runtime.Object, error) {
	app, ok := obj.(*grafanav1alpha1.Grafana)
	if !ok {
		return nil, xerrors.Newf("unexpected object type: %v", obj)
	}

	app, err := u.client.UpdateGrafana(ctx, app, metav1.UpdateOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return app, nil
}

func (u *UserController) ensureUsers(app *grafanav1alpha1.Grafana, users []*grafanav1alpha1.GrafanaUser) error {
	u.Log().Debug("users", zap.Int("len", len(users)))
	secret, err := u.secretLister.Secrets(app.Namespace).Get(app.Spec.AdminPasswordSecret.Name)
	if err != nil {
		return xerrors.WithStack(err)
	}
	password, ok := secret.Data[app.Spec.AdminPasswordSecret.Key]
	if !ok {
		return xerrors.Newf("%s is not found in %s", app.Spec.AdminPasswordSecret.Key, app.Spec.AdminPasswordSecret.Name)
	}
	svc, err := u.serviceLister.Services(app.Namespace).Get(app.Spec.Service.Name)
	if err != nil {
		return xerrors.WithStack(err)
	}
	grafanaClient := grafana.NewClient(
		fmt.Sprintf("http://%s.%s.svc:%d", svc.Name, app.Namespace, 3000),
		app.Spec.AdminUser,
		string(password),
		u.transport,
	)

	allUsers := make(map[string]*grafanav1alpha1.GrafanaUser)
	for _, v := range users {
		allUsers[v.Spec.Email] = v
	}

	currentUsers, err := grafanaClient.Users()
	if err != nil {
		return xerrors.WithStack(err)
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
		grafanaUser := allUsers[email]
		s := strings.Split(grafanaUser.Spec.Email, "@")
		name := s[0]
		u.Log().Info("Add User", zap.String("email", grafanaUser.Spec.Email), zap.String("name", name))
		if err := grafanaClient.AddUser(&grafana.User{Name: name, Login: name, Email: grafanaUser.Spec.Email, Password: randomString(32)}); err != nil {
			u.Log().Warn("Failed add user", zap.String("email", email), zap.Error(err))
		}
	}

	redundantUsersSet := currentUsersSet.Diff(idealUsersSet)
	for _, v := range redundantUsersSet.ToSlice() {
		email := v.(string)
		// Admin user should not delete
		if email == "admin@localhost" {
			continue
		}
		grafanaUser := currentUsersMap[email]
		u.Log().Info("Delete User", zap.Int("id", grafanaUser.Id))
		if err := grafanaClient.DeleteUser(grafanaUser.Id); err != nil {
			u.Log().Warn("Failed delete user", zap.String("email", grafanaUser.Email), zap.Int("id", grafanaUser.Id), zap.Error(err))
		}
	}

	currentUsers, err = grafanaClient.Users()
	if err != nil {
		return xerrors.WithStack(err)
	}
	for _, v := range currentUsers {
		grafanaUser, ok := allUsers[v.Email]
		if !ok {
			continue
		}
		if grafanaUser.Spec.Admin != v.IsAdmin {
			u.Log().Info("Change user permission", zap.Int("id", v.Id), zap.String("email", v.Email), zap.Bool("admin", grafanaUser.Spec.Admin))
			if err := grafanaClient.ChangeUserPermission(v.Id, grafanaUser.Spec.Admin); err != nil {
				u.Log().Warn("Failed change user permission", zap.String("email", v.Email), zap.Bool("admin", v.IsAdmin))
			}
		}
	}

	return nil
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
