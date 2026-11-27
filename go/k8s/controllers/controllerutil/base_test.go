package controllerutil

import (
	"context"
	"testing"
	"time"

	"go.f110.dev/kubeproto/go/apis/metav1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"go.f110.dev/mono/go/api/natsv1alpha1"
	"go.f110.dev/mono/go/k8s/client/testingclient"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestGenericControllerBase(t *testing.T) {
	logger.Init()

	t.Run("AddFinalizer", func(t *testing.T) {
		apiClient := testingclient.NewSet()
		target := &natsv1alpha1.JetStream{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: metav1.NamespaceDefault, ResourceVersion: "1", Finalizers: []string{"other"}},
			Spec:       natsv1alpha1.JetStreamSpec{MaxAge: &metav1.Duration{Duration: time.Hour}},
		}
		assertion.MustNoError(t, apiClient.Tracker().Add(target))
		b := NewGenericControllerBase[*natsv1alpha1.JetStream](
			"test",
			nil,
			fake.NewClientset(),
			nil,
			nil,
			[]string{"test"},
			func(_, _ string) (*natsv1alpha1.JetStream, error) { return target, nil },
			apiClient.NatsV1alpha1.PatchJetStream,
		)

		err := b.process(context.Background(), "default/test")
		assertion.MustNoError(t, err)

		actions := apiClient.Actions()
		assertion.MustLen(t, actions, 1)
		action := actions[0].(k8stesting.PatchAction)
		assertion.Equal(t, action.GetPatchType(), types.MergePatchType)
		assertion.Equal(t, string(action.GetPatch()), `{"metadata":{"finalizers":["other","test"],"resourceVersion":"1"}}`)
		updated, err := apiClient.NatsV1alpha1.GetJetStream(context.Background(), metav1.NamespaceDefault, "test", metav1.GetOptions{})
		assertion.MustNoError(t, err)
		assertion.Equal(t, updated.Finalizers, []string{"other", "test"})
		assertion.Equal(t, updated.Spec, target.Spec)
	})
}
