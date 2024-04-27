package controllers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/api/miniov1alpha1"
	"go.f110.dev/mono/go/k8s/controllers/controllertest"
	"go.f110.dev/mono/go/k8s/k8sfactory"
)

func TestMinIOClusterController_Reconcile(t *testing.T) {
	runner := controllertest.NewGenericTestRunner[*miniov1alpha1.MinIOCluster]()
	controller := NewMinIOClusterController(runner.CoreClient, &runner.Client.Set, nil, runner.CoreSharedInformerFactory, runner.Factory, false)

	target := minioClusterFixture()
	err := runner.Reconcile(controller.newReconciler(), target)
	require.NoError(t, err)

	runner.AssertCreateAction(t, k8sfactory.PersistentVolumeClaimFactory(nil,
		k8sfactory.Namef("%s-data-1", target.Name)))
	runner.AssertCreateAction(t, k8sfactory.PodFactory(nil,
		k8sfactory.Namef("%s-1", target.Name)))
	runner.AssertCreateAction(t, k8sfactory.ServiceFactory(nil,
		k8sfactory.Name(target.Name)))
	runner.AssertCreateAction(t, k8sfactory.SecretFactory(nil,
		k8sfactory.Name(target.Name)))
	runner.AssertNoUnexpectedAction(t)
}

func minioClusterFixture() *miniov1alpha1.MinIOCluster {
	return k8sfactory.MinIOClusterFactory(nil,
		k8sfactory.Name("test"),
		k8sfactory.Nodes(1),
		k8sfactory.TotalSize(10),
	)
}
