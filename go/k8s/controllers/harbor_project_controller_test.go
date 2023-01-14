package controllers

import (
	"context"
	"net/http"
	"regexp"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/api/harborv1alpha1"
	"go.f110.dev/mono/go/harbor"
	"go.f110.dev/mono/go/k8s/controllers/controllertest"
	"go.f110.dev/mono/go/k8s/k8sfactory"
)

func TestHarborProjectController(t *testing.T) {
	runner, controller := newHarborProjectController(t)
	target, fixtures := newHarborProjectFixture()
	runner.RegisterFixture(fixtures...)

	mockTransport := httpmock.NewMockTransport()
	controller.transport = mockTransport
	mockTransport.RegisterRegexpResponder(
		http.MethodHead,
		regexp.MustCompile(`.+/api/v2.0/projects.+`),
		httpmock.NewStringResponder(http.StatusNotFound, ""),
	)
	mockTransport.RegisterRegexpResponder(
		http.MethodPost,
		regexp.MustCompile(`.+/api/v2.0/projects$`),
		httpmock.NewStringResponder(http.StatusCreated, ""),
	)
	mockTransport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(`.+/api/v2.0/projects$`),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, []harbor.Project{
			{Id: 1, Name: target.Name},
		}),
	)

	err := runner.Reconcile(controller, target)
	require.NoError(t, err)

	expect := target.DeepCopy()
	expect.Status.Ready = true
	expect.Status.ProjectId = 1
	expect.Status.Registry = "test-registry.f110.dev"
	runner.AssertAction(t, controllertest.Action{
		Verb:        controllertest.ActionUpdate,
		Subresource: "status",
		Object:      expect,
	})
	runner.AssertNoUnexpectedAction(t)
}

func newHarborProjectController(t *testing.T) (*controllertest.TestRunner, *HarborProjectController) {
	runner := controllertest.NewTestRunner()
	secret := k8sfactory.SecretFactory(nil,
		k8sfactory.Name("admin"),
		k8sfactory.DefaultNamespace,
		k8sfactory.Data("HARBOR_ADMIN_PASSWORD", []byte("password")),
	)
	service := k8sfactory.ServiceFactory(nil,
		k8sfactory.Name("test"),
		k8sfactory.DefaultNamespace,
	)
	configMap := k8sfactory.ConfigMapFactory(nil,
		k8sfactory.Name("config"),
		k8sfactory.DefaultNamespace,
		k8sfactory.Data("EXT_ENDPOINT", []byte("http://test-registry.f110.dev")),
	)
	runner.RegisterFixture(secret, service, configMap)
	controller, err := NewHarborProjectController(
		context.Background(),
		runner.CoreClient,
		&runner.Client.Set,
		nil,
		runner.Factory,
		metav1.NamespaceDefault,
		service.Name,
		secret.Name,
		configMap.Name,
		false,
	)
	require.NoError(t, err)

	return runner, controller
}

func newHarborProjectFixture() (*harborv1alpha1.HarborProject, []runtime.Object) {
	target := k8sfactory.HarborProjectFactory(nil,
		k8sfactory.Name("test1"),
		k8sfactory.DefaultNamespace,
	)

	return target, []runtime.Object{}
}
