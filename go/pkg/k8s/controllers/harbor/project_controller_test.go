package harbor

import (
	"context"
	"net/http"
	"regexp"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/api/harborv1alpha1"
	"go.f110.dev/mono/go/pkg/harbor"
	"go.f110.dev/mono/go/pkg/k8s/controllers/controllertest"
)

func TestProjectController(t *testing.T) {
	runner, controller := newProjectController(t)
	target, fixtures := newProjectFixture()
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

func newProjectController(t *testing.T) (*controllertest.TestRunner, *ProjectController) {
	runner := controllertest.NewTestRunner()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "admin",
			Namespace: metav1.NamespaceDefault,
		},
		Data: map[string][]byte{
			"HARBOR_ADMIN_PASSWORD": []byte("password"),
		},
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: metav1.NamespaceDefault,
		},
	}
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: metav1.NamespaceDefault,
		},
		Data: map[string]string{
			"EXT_ENDPOINT": "http://test-registry.f110.dev",
		},
	}
	runner.RegisterFixture(secret, service, configMap)
	controller, err := NewProjectController(
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

func newProjectFixture() (*harborv1alpha1.HarborProject, []runtime.Object) {
	target := &harborv1alpha1.HarborProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test1",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: harborv1alpha1.HarborProjectSpec{},
	}

	return target, []runtime.Object{}
}
