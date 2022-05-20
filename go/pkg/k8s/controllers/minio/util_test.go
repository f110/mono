package minio

import (
	"net/http"
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/jarcoal/httpmock"
	miniocontrollerv1beta1 "github.com/minio/minio-operator/pkg/apis/miniocontroller/v1beta1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"go.f110.dev/mono/go/pkg/k8s/controllers/controllertest"
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

func newUserController(t *testing.T, runner *controllertest.TestRunner) (*UserController, *httpmock.MockTransport) {
	tr := httpmock.NewMockTransport()
	vaultClient, err := api.NewClient(&api.Config{
		HttpClient: &http.Client{
			Transport: tr,
		},
	})
	require.NoError(t, err)

	controller, err := NewUserController(
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
