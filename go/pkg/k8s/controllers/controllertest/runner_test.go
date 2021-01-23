package controllertest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	grafanav1alpha1 "go.f110.dev/mono/go/pkg/api/grafana/v1alpha1"
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
	grafana, err := r.Client.GrafanaV1alpha1().Grafanas(metav1.NamespaceDefault).Get(context.Background(), "foobar", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "foobar", grafana.Name)

	// Fetch from lister
	grafana, err = r.SharedInformerFactory.Grafana().V1alpha1().Grafanas().Lister().Grafanas(metav1.NamespaceDefault).Get("foobar")
	require.NoError(t, err)
	assert.Equal(t, "foobar", grafana.Name)
	fromList, err := r.SharedInformerFactory.Grafana().V1alpha1().Grafanas().Lister().Grafanas(metav1.NamespaceDefault).List(labels.Everything())
	require.NoError(t, err)
	assert.Len(t, fromList, 1)

	deploy, err := r.CoreClient.AppsV1().Deployments(metav1.NamespaceDefault).Get(context.Background(), "baz", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "baz", deploy.Name)

	deploy, err = r.CoreSharedInformerFactory.Apps().V1().Deployments().Lister().Deployments(metav1.NamespaceDefault).Get("baz")
	require.NoError(t, err)
	assert.Equal(t, "baz", deploy.Name)
}
