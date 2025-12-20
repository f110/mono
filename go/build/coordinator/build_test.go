package coordinator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakesecretstoreclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"

	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/database/dao/daotest"
	"go.f110.dev/mono/go/k8s/controllers/controllertest"
	"go.f110.dev/mono/go/k8s/k8sfactory"
	"go.f110.dev/mono/go/storage"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestBazelBuilder_SyncJob(t *testing.T) {
	runner := controllertest.NewGenericTestRunner[*batchv1.Job]()
	podInformer := runner.CoreSharedInformerFactory.Core().V1().Pods()
	jobInformer := runner.CoreSharedInformerFactory.Batch().V1().Jobs()
	mockDAO := struct {
		Repository *daotest.SourceRepository
		Task       *daotest.Task
	}{
		Repository: daotest.NewSourceRepository(),
		Task:       daotest.NewTask(),
	}
	mockDAO.Task.RegisterListPending([]*database.Task{}, nil)
	b, err := NewBazelBuilder(
		"",
		KubernetesOptions{
			JobInformer:       jobInformer,
			PodInformer:       podInformer,
			Client:            runner.CoreClient,
			SecretStoreClient: fakesecretstoreclient.NewSimpleClientset(),
		},
		dao.Options{
			Repository: mockDAO.Repository,
			Task:       mockDAO.Task,
		},
		metav1.NamespaceDefault,
		nil,
		"foo",
		storage.MinIOOptions{},
		BazelOptions{},
		nil,
		nil,
		false,
	)
	require.NoError(t, err)

	mockDAO.Repository.RegisterSelect(1, &database.SourceRepository{Id: 1, Url: "https://github.com/f110/mono"})
	mockDAO.Task.RegisterSelect(1, &database.Task{Id: 1})
	target := k8sfactory.JobFactory(nil,
		k8sfactory.Namespace(metav1.NamespaceDefault),
		k8sfactory.Name(t.Name()),
		k8sfactory.CreatedAt(time.Now().Add(-1*time.Minute)),
		k8sfactory.Labels(map[string]string{labelKeyRepoId: "1", labelKeyTaskId: "1"}),
		k8sfactory.Finalizer(bazelBuilderControllerFinalizerName),
		k8sfactory.MatchLabelSelector(map[string]string{labelKeyRepoId: "1", labelKeyTaskId: "1"}),
	)

	t.Run("Finished normally", func(t *testing.T) {
		t.Cleanup(func() {
			mockDAO.Task.Reset()
			runner.Reset()
		})

		target := k8sfactory.JobFactory(target,
			k8sfactory.Name(t.Name()),
			k8sfactory.JobComplete,
			k8sfactory.Pod(
				k8sfactory.PodFactory(nil,
					k8sfactory.Container(k8sfactory.ContainerFactory(nil)),
				),
			),
		)
		runner.RegisterFixture(target)
		err = b.syncJob(target)
		require.NoError(t, err)

		called := mockDAO.Task.Called("Update")
		require.Len(t, called, 1)
		updated := called[0].Args["task"].(*database.Task)
		assertion.NotNil(t, updated.FinishedAt)
		assertion.True(t, updated.Success)
		runner.AssertUpdateAction(t, "", k8sfactory.JobFactory(target, k8sfactory.RemoveFinalizer(bazelBuilderControllerFinalizerName)))
		runner.AssertNoUnexpectedAction(t)
	})

	t.Run("Timed out", func(t *testing.T) {
		t.Cleanup(func() {
			mockDAO.Task.Reset()
			runner.Reset()
		})

		mockDAO.Task.RegisterSelect(2, &database.Task{Id: 2})
		target := k8sfactory.JobFactory(target,
			k8sfactory.Name("timed-out"),
			k8sfactory.CreatedAt(time.Now().Add(-2*time.Hour)),
			k8sfactory.Label(labelKeyTaskId, "2"),
			k8sfactory.Pod(
				k8sfactory.PodFactory(nil,
					k8sfactory.Container(k8sfactory.ContainerFactory(nil)),
				),
			),
		)
		runner.RegisterFixture(target)
		err = b.syncJob(target)
		require.NoError(t, err)

		called := mockDAO.Task.Called("Update")
		require.Len(t, called, 1)
		updated := called[0].Args["task"].(*database.Task)
		assertion.NotNil(t, updated.FinishedAt)
		runner.AssertUpdateAction(t, "", k8sfactory.JobFactory(target, k8sfactory.RemoveFinalizer(bazelBuilderControllerFinalizerName)))
		runner.AssertNoUnexpectedAction(t)
	})

	t.Run("Force stopped", func(t *testing.T) {
		t.Cleanup(func() {
			mockDAO.Task.Reset()
			runner.Reset()
		})

		mockDAO.Task.RegisterSelect(4, &database.Task{Id: 4, JobObjectName: "force-stopped"})
		target := k8sfactory.JobFactory(target,
			k8sfactory.Name("force-stopped"),
			k8sfactory.CreatedAt(time.Now().Add(-2*time.Hour)),
			k8sfactory.Label(labelKeyTaskId, "4"),
			k8sfactory.Label(labelKeyForceStop, "true"),
			k8sfactory.Pod(
				k8sfactory.PodFactory(nil,
					k8sfactory.Container(k8sfactory.ContainerFactory(nil)),
				),
			),
		)
		runner.RegisterFixture(target)
		err = b.syncJob(target)
		require.NoError(t, err)

		called := mockDAO.Task.Called("Update")
		require.Len(t, called, 1)
		updated := called[0].Args["task"].(*database.Task)
		assertion.NotNil(t, updated.FinishedAt)
		runner.AssertUpdateAction(t, "", k8sfactory.JobFactory(target, k8sfactory.RemoveFinalizer(bazelBuilderControllerFinalizerName)))
		runner.AssertNoUnexpectedAction(t)
	})

	t.Run("Delete Job", func(t *testing.T) {
		t.Cleanup(func() {
			mockDAO.Task.Reset()
			runner.Reset()
		})

		mockDAO.Task.RegisterSelect(3, &database.Task{Id: 3})
		target := k8sfactory.JobFactory(target,
			k8sfactory.Name(t.Name()),
			k8sfactory.Delete,
			k8sfactory.Label(labelKeyTaskId, "3"),
		)
		runner.RegisterFixture(target)
		err = b.syncJob(target)
		require.NoError(t, err)

		called := mockDAO.Task.Called("Update")
		require.Len(t, called, 1)
		updated := called[0].Args["task"].(*database.Task)
		assertion.NotNil(t, updated.FinishedAt)

		runner.AssertUpdateAction(t, "", k8sfactory.JobFactory(target, k8sfactory.RemoveFinalizer(t.Name())))
		runner.AssertNoUnexpectedAction(t)
	})
}

func TestBazelBuilder_ForceStop(t *testing.T) {
	runner := controllertest.NewGenericTestRunner[*batchv1.Job]()
	podInformer := runner.CoreSharedInformerFactory.Core().V1().Pods()
	jobInformer := runner.CoreSharedInformerFactory.Batch().V1().Jobs()
	mockDAO := struct {
		Repository *daotest.SourceRepository
		Task       *daotest.Task
	}{
		Repository: daotest.NewSourceRepository(),
		Task:       daotest.NewTask(),
	}
	mockDAO.Task.RegisterListPending([]*database.Task{}, nil)
	b, err := NewBazelBuilder(
		"",
		KubernetesOptions{
			JobInformer:       jobInformer,
			PodInformer:       podInformer,
			Client:            runner.CoreClient,
			SecretStoreClient: fakesecretstoreclient.NewSimpleClientset(),
		},
		dao.Options{
			Repository: mockDAO.Repository,
			Task:       mockDAO.Task,
		},
		metav1.NamespaceDefault,
		nil,
		"foo",
		storage.MinIOOptions{},
		BazelOptions{},
		nil,
		nil,
		false,
	)
	require.NoError(t, err)

	mockDAO.Repository.RegisterSelect(1, &database.SourceRepository{Id: 1, Url: "https://github.com/f110/mono"})
	mockDAO.Task.RegisterSelect(1, &database.Task{Id: 1, JobObjectName: t.Name()})
	target := k8sfactory.JobFactory(nil,
		k8sfactory.Namespace(metav1.NamespaceDefault),
		k8sfactory.Name(t.Name()),
		k8sfactory.CreatedAt(time.Now().Add(-1*time.Minute)),
		k8sfactory.Labels(map[string]string{labelKeyRepoId: "1", labelKeyTaskId: "1"}),
		k8sfactory.Finalizer(bazelBuilderControllerFinalizerName),
		k8sfactory.MatchLabelSelector(map[string]string{labelKeyRepoId: "1", labelKeyTaskId: "1"}),
	)
	runner.RegisterFixture(target)

	err = b.ForceStop(t.Context(), 1)
	require.NoError(t, err)

	updatedJob, err := runner.CoreClient.BatchV1().Jobs(metav1.NamespaceDefault).Get(t.Context(), t.Name(), metav1.GetOptions{})
	require.NoError(t, err)
	assertion.Contains(t, updatedJob.GetLabels(), labelKeyForceStop)
}
