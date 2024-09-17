package controllers

import (
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/api/grafanav1alpha1"
	"go.f110.dev/mono/go/grafana"
	"go.f110.dev/mono/go/k8s/k8sfactory"
)

func TestGrafanaUserController(t *testing.T) {
	runner, controller := newGrafanaUserController(t)
	target, fixtures := grafanaFixture()

	mockTransport := httpmock.NewMockTransport()
	controller.transport = mockTransport
	mockTransport.RegisterResponder(
		http.MethodGet,
		"http://grafana.default.svc:3000/api/users",
		httpmock.NewJsonResponderOrPanic(http.StatusOK, []grafana.User{
			{
				Id:    1,
				Email: "user2@example.com",
			},
		}),
	)
	mockTransport.RegisterResponder(
		http.MethodPost,
		"http://grafana.default.svc:3000/api/admin/users",
		httpmock.NewStringResponder(http.StatusOK, ""),
	)
	mockTransport.RegisterResponder(
		http.MethodDelete,
		"http://grafana.default.svc:3000/api/admin/users/1",
		httpmock.NewStringResponder(http.StatusOK, ""),
	)

	user := k8sfactory.GrafanaUserFactory(nil,
		k8sfactory.Name("user1"),
		k8sfactory.DefaultNamespace,
		k8sfactory.Labels(target.Spec.UserSelector.MatchLabels),
		k8sfactory.UserEmail("user1@example.com"),
	)
	runner.RegisterFixture(user)
	runner.RegisterFixture(fixtures...)

	err := runner.Reconcile(controller.newReconciler(), target)
	require.NoError(t, err)

	expect := target.DeepCopy()
	expect.Status.ObservedGeneration = 2
	runner.AssertUpdateAction(t, "status", expect)
	runner.AssertNoUnexpectedAction(t)
}

func grafanaFixture() (*grafanav1alpha1.Grafana, []runtime.Object) {
	secret := k8sfactory.SecretFactory(nil,
		k8sfactory.Name("admin"),
		k8sfactory.DefaultNamespace,
		k8sfactory.Data("password", []byte("foobar")),
	)
	service := k8sfactory.ServiceFactory(nil,
		k8sfactory.Name("grafana"),
		k8sfactory.DefaultNamespace,
	)

	target := k8sfactory.GrafanaFactory(nil,
		k8sfactory.Name("test"),
		k8sfactory.DefaultNamespace,
		k8sfactory.Generation(2),
		k8sfactory.UserSelector(k8sfactory.MatchLabel(map[string]string{"app": "test"})),
		k8sfactory.ServiceReference(k8sfactory.LocalObjectReference(service)),
		k8sfactory.AdminPasswordSecret(k8sfactory.SecretKeySelector(secret, "password")),
	)
	return target, []runtime.Object{secret, service}
}
