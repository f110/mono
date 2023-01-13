package controllers

import (
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	miniocontrollerv1beta1 "github.com/minio/minio-operator/pkg/apis/miniocontroller/v1beta1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"go.f110.dev/mono/go/k8s/controllers/controllertest"
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

func newBucketController(t *testing.T, runner *controllertest.TestRunner) *BucketController {
	controller, err := NewBucketController(runner.CoreClient, &runner.Client.Set, nil, runner.CoreSharedInformerFactory, runner.Factory, false)
	require.NoError(t, err)

	return controller
}

func newUserController(t *testing.T, runner *controllertest.TestRunner) (*MinIOUserController, *httpmock.MockTransport) {
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
