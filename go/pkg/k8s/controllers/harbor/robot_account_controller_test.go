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

	harborv1alpha1 "go.f110.dev/mono/go/pkg/api/harbor/v1alpha1"
	"go.f110.dev/mono/go/pkg/harbor"
	"go.f110.dev/mono/go/pkg/http/mockutil"
	"go.f110.dev/mono/go/pkg/k8s/controllers/controllertest"
)

func TestRobotAccountController(t *testing.T) {
	runner, controller := newRobotAccountController(t)
	target, fixtures := newRobotAccountFixtures()
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

func newRobotAccountController(t *testing.T) (*controllertest.TestRunner, *RobotAccountController) {
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
	runner.RegisterFixture(secret, service)
	controller, err := NewRobotAccountController(
		context.Background(),
		runner.CoreClient,
		runner.Client,
		nil,
		runner.SharedInformerFactory,
		metav1.NamespaceDefault,
		service.Name,
		secret.Name,
		false,
	)
	require.NoError(t, err)

	return runner, controller
}

func newRobotAccountFixtures() (*harborv1alpha1.HarborRobotAccount, []runtime.Object) {
	project := &harborv1alpha1.HarborProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tool",
			Namespace: metav1.NamespaceDefault,
		},
		Status: harborv1alpha1.HarborProjectStatus{
			Ready:     true,
			ProjectId: 1,
		},
	}

	target := &harborv1alpha1.HarborRobotAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "robot1",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: harborv1alpha1.HarborRobotAccountSpec{
			SecretName:       "robot1-account",
			ProjectName:      project.Name,
			ProjectNamespace: project.Namespace,
		},
	}

	return target, []runtime.Object{project}
}
