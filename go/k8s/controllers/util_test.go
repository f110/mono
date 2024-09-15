package controllers

import (
	"context"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	miniocontrollerv1beta1 "github.com/minio/minio-operator/pkg/apis/miniocontroller/v1beta1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"go.f110.dev/mono/go/api/miniov1alpha1"
	"go.f110.dev/mono/go/k8s/controllers/controllertest"
	"go.f110.dev/mono/go/k8s/k8sfactory"
	"go.f110.dev/mono/go/vault"
)

func newRunner() *controllertest.TestRunner {
	runner := controllertest.NewTestRunner()
	runner.CoreClient.Resources = append(
		runner.CoreClient.Resources, &metav1.APIResourceList{
			GroupVersion: miniocontrollerv1beta1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Kind: "MinIOInstance"},
			},
		},
	)

	return runner
}

func newMinIOBucketController(t *testing.T, runner *controllertest.TestRunner) *MinIOBucketController {
	controller, err := NewMinIOBucketController(runner.CoreClient, &runner.Client.Set, nil, runner.CoreSharedInformerFactory, runner.Factory, false)
	require.NoError(t, err)

	return controller
}

func newMinIOUserController(t *testing.T, runner *controllertest.GenericTestRunner[*miniov1alpha1.MinIOUser]) (*MinIOUserController, *httpmock.MockTransport) {
	tr := httpmock.NewMockTransport()
	vaultClient, err := vault.NewClient("http://localhost:8300", "", vault.HttpClient(&http.Client{Transport: tr}))
	require.NoError(t, err)

	controller, err := NewMinIOUserController(
		runner.CoreClient,
		&runner.Client.Set,
		nil,
		runner.CoreSharedInformerFactory,
		runner.Factory,
		vaultClient,
		false,
	)
	require.NoError(t, err)

	controller.transport = tr
	return controller, tr
}

func newConsulBackupController(t *testing.T) (*controllertest.TestRunner, *ConsulBackupController) {
	runner := controllertest.NewTestRunner()
	controller, err := NewConsulBackupController(
		runner.CoreSharedInformerFactory,
		runner.Factory,
		runner.CoreClient,
		&runner.Client.Set,
		nil,
		false,
	)
	require.NoError(t, err)

	return runner, controller
}

func newGrafanaUserController(t *testing.T) (*controllertest.TestRunner, *GrafanaUserController) {
	runner := controllertest.NewTestRunner()
	controller, err := NewGrafanaUserController(
		runner.CoreSharedInformerFactory,
		runner.Factory,
		runner.CoreClient,
		&runner.Client.Set,
	)
	require.NoError(t, err)

	return runner, controller
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
