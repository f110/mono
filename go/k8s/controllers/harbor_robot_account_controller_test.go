package controllers

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
	"go.f110.dev/mono/go/harbor"
	"go.f110.dev/mono/go/http/mockutil"
	"go.f110.dev/mono/go/k8s/controllers/controllertest"
	"go.f110.dev/mono/go/k8s/k8sfactory"
)

func TestHarborRobotAccountController(t *testing.T) {
	runner, controller := newHarborRobotAccountController(t)
	target, fixtures := newHarborRobotAccountFixtures()
	runner.RegisterFixture(fixtures...)

	mockTransport := httpmock.NewMockTransport()
	controller.transport = mockTransport
	mockTransport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(".+/projects/1/robots$"),
		mockutil.NewMultipleResponder(
			httpmock.NewJsonResponderOrPanic(http.StatusOK, []harbor.RobotAccount{}),
			httpmock.NewJsonResponderOrPanic(http.StatusOK, []harbor.RobotAccount{
				{Name: "$" + target.Name, Id: 10},
			}),
		),
	)
	mockTransport.RegisterRegexpResponder(
		http.MethodPost,
		regexp.MustCompile(".+/projects/1/robots$"),
		httpmock.NewJsonResponderOrPanic(http.StatusCreated, harbor.RobotAccount{}),
	)

	err := runner.Reconcile(controller, target)
	require.NoError(t, err)

	expect := target.DeepCopy()
	expect.Status.Ready = true
	expect.Status.RobotId = 10
	runner.AssertAction(t, controllertest.Action{
		Verb:        controllertest.ActionUpdate,
		Subresource: "status",
		Object:      expect,
	})
	runner.AssertAction(t, controllertest.Action{
		Verb: controllertest.ActionCreate,
		Object: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      target.Spec.SecretName,
				Namespace: target.Namespace,
			},
		},
	})
	runner.AssertNoUnexpectedAction(t)
}

func newHarborRobotAccountController(t *testing.T) (*controllertest.TestRunner, *HarborRobotAccountController) {
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
	runner.RegisterFixture(secret, service)
	controller, err := NewHarborRobotAccountController(
		context.Background(),
		runner.CoreClient,
		&runner.Client.Set,
		nil,
		runner.Factory,
		metav1.NamespaceDefault,
		service.Name,
		secret.Name,
		false,
	)
	require.NoError(t, err)

	return runner, controller
}

func newHarborRobotAccountFixtures() (*harborv1alpha1.HarborRobotAccount, []runtime.Object) {
	project := k8sfactory.HarborProjectFactory(nil,
		k8sfactory.Name("tool"),
		k8sfactory.DefaultNamespace,
		k8sfactory.ReadyProject(1),
	)

	target := k8sfactory.HarborRobotAccountFactory(nil,
		k8sfactory.Name("robot1"),
		k8sfactory.DefaultNamespace,
		k8sfactory.ProjectReference(project),
		k8sfactory.DockerSecret(k8sfactory.SecretFactory(nil, k8sfactory.Name("robot1-account"))),
	)
	return target, []runtime.Object{project}
}
