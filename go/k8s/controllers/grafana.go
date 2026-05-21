package controllers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"time"

	"go.f110.dev/kubeproto/go/apis/corev1"
	"go.f110.dev/kubeproto/go/apis/metav1"
	"go.f110.dev/kubeproto/go/k8sclient"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"go.f110.dev/mono/go/api/grafanav1alpha1"
	"go.f110.dev/mono/go/collections/set"
	"go.f110.dev/mono/go/grafana"
	"go.f110.dev/mono/go/k8s/client"
	"go.f110.dev/mono/go/k8s/controllers/controllerutil"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/stringsutil"
)

const (
	grafanaControllerFinalizerName = "grafana-user-controller.grafana.f110.dev/finalizer" // historical reason
)

type GrafanaController struct {
	*controllerutil.GenericControllerBase[*grafanav1alpha1.Grafana]

	client *client.GrafanaV1alpha1

	secretLister  *k8sclient.CoreV1SecretLister
	serviceLister *k8sclient.CoreV1ServiceLister
	appLister     *client.GrafanaV1alpha1GrafanaLister
	userLister    *client.GrafanaV1alpha1GrafanaUserLister

	// for testing
	transport http.RoundTripper
}

func NewGrafanaController(
	coreSharedInformerFactory *k8sclient.InformerFactory,
	factory *client.InformerFactory,
	coreClient *k8sclient.Set,
	k8sClient kubernetes.Interface,
	apiClient *client.Set,
) (*GrafanaController, error) {
	secretInformer := coreSharedInformerFactory.InformerFor(&corev1.Secret{})
	serviceInformer := coreSharedInformerFactory.InformerFor(&corev1.Service{})
	grafanaInformers := client.NewGrafanaV1alpha1Informer(factory.Cache(), apiClient.GrafanaV1alpha1, metav1.NamespaceAll, 30*time.Second)
	appInformer := grafanaInformers.GrafanaInformer()
	userInformer := grafanaInformers.GrafanaUserInformer()

	coreInformers := k8sclient.NewCoreV1Informer(coreSharedInformerFactory.Cache(), coreClient.CoreV1, metav1.NamespaceAll, 30*time.Second)
	a := &GrafanaController{
		client:        apiClient.GrafanaV1alpha1,
		secretLister:  coreInformers.SecretLister(),
		serviceLister: coreInformers.ServiceLister(),
		appLister:     grafanaInformers.GrafanaLister(),
		userLister:    grafanaInformers.GrafanaUserLister(),
	}
	a.GenericControllerBase = controllerutil.NewGenericControllerBase[*grafanav1alpha1.Grafana](
		"grafana-controller",
		a.newReconciler,
		k8sClient,
		[]cache.SharedIndexInformer{appInformer, userInformer},
		[]cache.SharedIndexInformer{secretInformer, serviceInformer},
		[]string{grafanaControllerFinalizerName},
		grafanaInformers.GrafanaLister().Get,
		apiClient.GrafanaV1alpha1.UpdateGrafana,
	)

	return a, nil
}

func (u *GrafanaController) newReconciler() controllerutil.GenericReconciler[*grafanav1alpha1.Grafana] {
	return &grafanaReconciler{
		serviceLister: u.serviceLister,
		secretLister:  u.secretLister,
		userLister:    u.userLister,
		client:        u.client,
		logger:        u.Log(),
		transport:     u.transport,
	}
}

type grafanaReconciler struct {
	serviceLister *k8sclient.CoreV1ServiceLister
	secretLister  *k8sclient.CoreV1SecretLister
	userLister    *client.GrafanaV1alpha1GrafanaUserLister
	client        *client.GrafanaV1alpha1

	logger    *zap.Logger
	transport http.RoundTripper
}

var _ controllerutil.GenericReconciler[*grafanav1alpha1.Grafana] = (*grafanaReconciler)(nil)

func (u *grafanaReconciler) Reconcile(ctx context.Context, obj *grafanav1alpha1.Grafana) error {
	app := obj
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

func (u *grafanaReconciler) Finalize(ctx context.Context, obj *grafanav1alpha1.Grafana) error {
	return nil
}

func (u *grafanaReconciler) ensureUsers(app *grafanav1alpha1.Grafana, users []*grafanav1alpha1.GrafanaUser) error {
	u.logger.Debug("users", slog.Int("len", len(users)))
	secret, err := u.secretLister.Get(app.Namespace, app.Spec.AdminPasswordSecret.Name)
	if err != nil {
		return xerrors.WithStack(err)
	}
	password, ok := secret.Data[app.Spec.AdminPasswordSecret.Key]
	if !ok {
		return xerrors.Definef("%s is not found in %s", app.Spec.AdminPasswordSecret.Key, app.Spec.AdminPasswordSecret.Name).WithStack()
	}
	svc, err := u.serviceLister.Get(app.Namespace, app.Spec.Service.Name)
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

	currentUsersSet := set.New[string]()
	for _, v := range currentUsers {
		currentUsersSet.Add(v.Email)
	}
	idealUsersSet := set.New[string]()
	for _, v := range users {
		idealUsersSet.Add(v.Spec.Email)
	}

	missingUsersSet := idealUsersSet.Diff(currentUsersSet)
	for _, email := range missingUsersSet.ToSlice() {
		grafanaUser := allUsers[email]
		s := strings.Split(grafanaUser.Spec.Email, "@")
		name := s[0]
		u.logger.Info("Add User", slog.String("email", grafanaUser.Spec.Email), slog.String("name", name))
		if err := grafanaClient.AddUser(&grafana.User{Name: name, Login: name, Email: grafanaUser.Spec.Email, Password: stringsutil.RandomString(32)}); err != nil {
			u.logger.Warn("Failed add user", slog.String("email", email), slogger.E(err))
		}
	}

	redundantUsersSet := currentUsersSet.Diff(idealUsersSet)
	for _, email := range redundantUsersSet.ToSlice() {
		// Admin user should not delete
		if email == "admin@localhost" {
			continue
		}
		grafanaUser := currentUsersMap[email]
		u.logger.Info("Delete User", slog.Int("id", grafanaUser.Id))
		if err := grafanaClient.DeleteUser(grafanaUser.Id); err != nil {
			u.logger.Warn("Failed delete user", slog.String("email", grafanaUser.Email), slog.Int("id", grafanaUser.Id), slogger.E(err))
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
			u.logger.Info("Change user permission", slog.Int("id", v.Id), slog.String("email", v.Email), slog.Bool("admin", grafanaUser.Spec.Admin))
			if err := grafanaClient.ChangeUserPermission(v.Id, grafanaUser.Spec.Admin); err != nil {
				u.logger.Warn("Failed change user permission", slog.String("email", v.Email), slog.Bool("admin", v.IsAdmin))
			}
		}
	}

	return nil
}
