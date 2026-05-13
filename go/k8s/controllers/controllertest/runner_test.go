package controllertest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.f110.dev/kubeproto/go/apis/appsv1"
	"go.f110.dev/kubeproto/go/apis/metav1"
	"go.f110.dev/kubeproto/go/k8sclient"
	"k8s.io/apimachinery/pkg/labels"

	"go.f110.dev/mono/go/api/grafanav1alpha1"
	"go.f110.dev/mono/go/k8s/client"
)

func TestResourceName(t *testing.T) {
	assert.Equal(t, "grafanas", resourceName(&grafanav1alpha1.Grafana{}))
	assert.Equal(t, "deployments", resourceName(&appsv1.Deployment{}))
}

func TestRegisterFixture(t *testing.T) {
	r := NewTestRunner()
	r.RegisterFixture(&grafanav1alpha1.Grafana{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foobar",
			Namespace: metav1.NamespaceDefault,
		},
	})
	r.RegisterFixture(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "baz",
			Namespace: metav1.NamespaceDefault,
		},
	})

	// Fetch from client via object tracker
	grafana, err := r.Client.GrafanaV1alpha1.GetGrafana(context.Background(), metav1.NamespaceDefault, "foobar", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "foobar", grafana.Name)

	// Fetch from lister
	informers := client.NewGrafanaV1alpha1Informer(r.Factory.Cache(), r.Client.GrafanaV1alpha1, metav1.NamespaceDefault, 0)
	grafana, err = informers.GrafanaLister().Get(metav1.NamespaceDefault, "foobar")
	require.NoError(t, err)
	assert.Equal(t, "foobar", grafana.Name)
	fromList, err := informers.GrafanaLister().List(metav1.NamespaceDefault, labels.Everything())
	require.NoError(t, err)
	assert.Len(t, fromList, 1)

	deploy, err := r.CoreClient.AppsV1.GetDeployment(t.Context(), metav1.NamespaceDefault, "baz", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "baz", deploy.Name)

	appsInformers := k8sclient.NewAppsV1Informer(r.CoreSharedInformerFactory.Cache(), r.CoreClient.AppsV1, metav1.NamespaceDefault, 0)
	deploy, err = appsInformers.DeploymentLister().Get(metav1.NamespaceDefault, "baz")
	require.NoError(t, err)
	assert.Equal(t, "baz", deploy.Name)
}
