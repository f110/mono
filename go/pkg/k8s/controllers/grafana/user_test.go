package grafana

import (
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	grafanav1alpha1 "go.f110.dev/mono/go/pkg/api/grafana/v1alpha1"
	"go.f110.dev/mono/go/pkg/grafana"
	"go.f110.dev/mono/go/pkg/k8s/controllers/controllertest"
)

func TestUserController(t *testing.T) {
	runner := controllertest.NewTestRunner()
	controller, err := NewUserController(
		runner.CoreSharedInformerFactory,
		runner.SharedInformerFactory,
		runner.CoreClient,
		runner.Client,
	)
	require.NoError(t, err)
	target, fixtures := grafanaFixture()

	mockTransport := httpmock.NewMockTransport()
	controller.transport = mockTransport
	mockTransport.RegisterResponder(
		http.MethodGet,
		"http://grafana.default.svc:3000/api/users",
		httpmock.NewJsonResponderOrPanic(http.StatusOK, []grafana.User{}),
	)
	mockTransport.RegisterResponder(
		http.MethodPost,
		"http://grafana.default.svc:3000/api/admin/users",
		httpmock.NewStringResponder(http.StatusOK, ""),
	)

	user := &grafanav1alpha1.GrafanaUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "user1",
			Namespace: metav1.NamespaceDefault,
			Labels:    target.Spec.UserSelector.MatchLabels,
		},
		Spec: grafanav1alpha1.GrafanaUserSpec{
			Email: "user1@example.com",
		},
	}
	runner.RegisterFixture(user)
	runner.RegisterFixture(fixtures...)

	err = runner.Reconcile(controller, target)
	require.NoError(t, err)

	expect := target.DeepCopy()
	expect.Status.ObservedGeneration = 2
	runner.AssertAction(
		t, controllertest.Action{
			Verb:        controllertest.ActionUpdate,
			Subresource: "status",
			Object:      expect,
		},
	)
}

func grafanaFixture() (*grafanav1alpha1.Grafana, []runtime.Object) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "admin",
			Namespace: metav1.NamespaceDefault,
		},
		Data: map[string][]byte{
			"password": []byte("foobar"),
		},
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grafana",
			Namespace: metav1.NamespaceDefault,
		},
	}

	target := &grafanav1alpha1.Grafana{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test",
			Namespace:  metav1.NamespaceDefault,
			Generation: 2,
		},
		Spec: grafanav1alpha1.GrafanaSpec{
			UserSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			Service: &corev1.LocalObjectReference{
				Name: service.Name,
			},
			AdminPasswordSecret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secret.Name,
				},
				Key: "password",
			},
		},
	}
	return target, []runtime.Object{secret, service}
}
