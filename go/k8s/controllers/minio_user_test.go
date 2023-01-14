package controllers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/api/miniov1alpha1"
	"go.f110.dev/mono/go/k8s/controllers/controllertest"
	"go.f110.dev/mono/go/k8s/k8sfactory"
)

func TestMinIOUserController(t *testing.T) {
	t.Run("CreateSecret", func(t *testing.T) {
		runner := newRunner()
		controller, mockTransport := newUserController(t, runner)
		mockTransport.RegisterResponder(
			http.MethodPut,
			"/minio/admin/v3/add-user",
			httpmock.NewStringResponder(
				http.StatusOK,
				"",
			),
		)
		mockTransport.RegisterResponder(
			http.MethodPost,
			"/v1/secret/data/test",
			httpmock.NewStringResponder(
				http.StatusOK,
				"",
			),
		)
		target, fixtures := minIOUserFixture()
		runner.RegisterFixture(fixtures...)

		err := runner.Reconcile(controller, target)
		require.NoError(t, err)

		runner.AssertAction(t, controllertest.Action{
			Verb: controllertest.ActionCreate,
			Object: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-accesskey", target.Name),
					Namespace: target.Namespace,
				},
			},
		})
		runner.AssertNoUnexpectedAction(t)
	})
}

func minIOUserFixture() (*miniov1alpha1.MinIOUser, []runtime.Object) {
	secret := k8sfactory.SecretFactory(nil,
		k8sfactory.Name("root-accesskey"),
		k8sfactory.DefaultNamespace,
		k8sfactory.Data("accesskey", []byte("rootaccesskey")),
		k8sfactory.Data("secretkey", []byte("rootsecretkey")),
	)
	instance := k8sfactory.MinIOInstanceFactory(nil,
		k8sfactory.Name("test"),
		k8sfactory.DefaultNamespace,
		k8sfactory.Labels(map[string]string{"app": "minio"}),
		k8sfactory.MinIOCredential(k8sfactory.LocalObjectReference(secret)),
	)
	service := k8sfactory.ServiceFactory(nil,
		k8sfactory.Namef("%s-hl-svc", instance.Name),
		k8sfactory.DefaultNamespace,
		k8sfactory.Port("", corev1.ProtocolTCP, 9000),
	)
	user := k8sfactory.MinIOUserFactory(nil,
		k8sfactory.Name("test"),
		k8sfactory.DefaultNamespace,
		k8sfactory.VaultPath("/secret", "/test"),
		k8sfactory.MinIOSelector(k8sfactory.MatchLabel(instance.ObjectMeta.Labels)),
	)

	return user, []runtime.Object{instance, service, secret}
}
