package controllers

import (
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	miniocontrollerv1beta1 "github.com/minio/minio-operator/pkg/apis/miniocontroller/v1beta1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/api/miniov1alpha1"
	"go.f110.dev/mono/go/k8s/controllers/controllertest"
	"go.f110.dev/mono/go/k8s/k8sfactory"
)

func TestMinIOUserController(t *testing.T) {
	t.Run("MinIOInstance", func(t *testing.T) {
		runner := controllertest.NewGenericTestRunner[*miniov1alpha1.MinIOUser]()
		controller, mockTransport := newMinIOUserController(t, runner)
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
		instance, depObjs := minIOInstanceFixtureForMinIOUser()
		target := k8sfactory.MinIOUserFactory(nil,
			k8sfactory.Name("test"),
			k8sfactory.DefaultNamespace,
			k8sfactory.VaultPath("/secret", "/test"),
			k8sfactory.MinIOSelector(k8sfactory.MatchLabel(instance.ObjectMeta.Labels)),
		)
		runner.RegisterFixture(instance)
		runner.RegisterFixture(depObjs...)

		err := runner.Reconcile(controller.newReconciler(), target)
		require.NoError(t, err)

		runner.AssertCreateAction(t, k8sfactory.SecretFactory(nil,
			k8sfactory.Namef("%s-accesskey", target.Name),
			k8sfactory.Namespace(target.Namespace),
		))
		runner.AssertNoUnexpectedAction(t)
	})

	t.Run("MinIOCluster", func(t *testing.T) {
		runner := controllertest.NewGenericTestRunner[*miniov1alpha1.MinIOUser]()
		controller, mockTransport := newMinIOUserController(t, runner)
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
		cluster, depObjs := minIOClusterFixtureForMinIOUser()
		target := k8sfactory.MinIOUserFactory(nil,
			k8sfactory.Name("test"),
			k8sfactory.DefaultNamespace,
			k8sfactory.VaultPath("/secret", "/test"),
			k8sfactory.MinIOSelector(k8sfactory.MatchLabel(cluster.ObjectMeta.Labels)),
		)
		runner.RegisterFixture(cluster)
		runner.RegisterFixture(depObjs...)

		err := runner.Reconcile(controller.newReconciler(), target)
		require.NoError(t, err)

		runner.AssertCreateAction(t,
			k8sfactory.SecretFactory(nil,
				k8sfactory.Namef("%s-accesskey", target.Name),
				k8sfactory.Namespace(target.Namespace),
			),
		)
		runner.AssertNoUnexpectedAction(t)
	})
}

func minIOInstanceFixtureForMinIOUser() (*miniocontrollerv1beta1.MinIOInstance, []runtime.Object) {
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

	return instance, []runtime.Object{service, secret}
}

func minIOClusterFixtureForMinIOUser() (*miniov1alpha1.MinIOCluster, []runtime.Object) {
	secret := k8sfactory.SecretFactory(nil,
		k8sfactory.Name("test"),
		k8sfactory.DefaultNamespace,
		k8sfactory.Data("password", []byte("rootaccesskey")),
	)
	service := k8sfactory.ServiceFactory(nil,
		k8sfactory.Name("test"),
		k8sfactory.DefaultNamespace,
		k8sfactory.Port("", corev1.ProtocolTCP, 9000),
	)
	cluster := k8sfactory.MinIOClusterFactory(nil,
		k8sfactory.Name("test"),
		k8sfactory.DefaultNamespace,
		k8sfactory.Labels(map[string]string{"app": "minio"}),
	)

	return cluster, []runtime.Object{secret, service}
}
