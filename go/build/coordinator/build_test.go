package coordinator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.f110.dev/kubeproto/go/apis/batchv1"
	"go.f110.dev/kubeproto/go/apis/corev1"
	"go.f110.dev/kubeproto/go/apis/metav1"
	"go.f110.dev/kubeproto/go/k8sclient"
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
	coreInformer := k8sclient.NewCoreV1Informer(runner.CoreSharedInformerFactory.Cache(), runner.CoreClient.CoreV1, metav1.NamespaceDefault, 30*time.Second)
	batchInformer := k8sclient.NewBatchV1Informer(runner.CoreSharedInformerFactory.Cache(), runner.CoreClient.BatchV1, metav1.NamespaceDefault, 30*time.Second)
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
			BatchInformer:     batchInformer,
			CoreInformer:      coreInformer,
			Client:            &runner.CoreClient.Set,
			SecretStoreClient: fakesecretstoreclient.NewSimpleClientset(),
		},
		dao.Options{
			Repository: mockDAO.Repository,
			Task:       mockDAO.Task,
		},
		metav1.NamespaceDefault,
		nil,
		"foo",
		storage.S3Options{},
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

	t.Run("Update node and container while running", func(t *testing.T) {
		t.Cleanup(func() {
			mockDAO.Task.Reset()
			runner.Reset()
		})

		runningTask := &database.Task{Id: 1}
		runningTask.ResetMark()
		mockDAO.Task.RegisterSelect(1, runningTask)
		pod := k8sfactory.PodFactory(nil,
			k8sfactory.Name("running-pod"),
			k8sfactory.Namespace(metav1.NamespaceDefault),
			k8sfactory.Labels(map[string]string{labelKeyRepoId: "1", labelKeyTaskId: "1"}),
		)
		pod.Status = &corev1.PodStatus{
			HostIP: "10.0.0.1",
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "main", Image: "registry.f110.dev/build/bazel:latest", ImageID: "registry.f110.dev/build/bazel@sha256:abcdef"},
			},
		}
		node := &corev1.Node{}
		node.SetName("node-a")
		node.Status = &corev1.NodeStatus{Addresses: []corev1.NodeAddress{{Type: corev1.NodeAddressTypeInternalIP, Address: "10.0.0.1"}}}

		target := k8sfactory.JobFactory(target, k8sfactory.Name("running"))
		runner.RegisterFixture(target, pod, node)
		err = b.syncJob(target)
		require.NoError(t, err)

		called := mockDAO.Task.Called("Update")
		require.Len(t, called, 1)
		updated := called[0].Args["task"].(*database.Task)
		assertion.Nil(t, updated.FinishedAt)
		assertion.Equal(t, "node-a", updated.Node)
		assertion.Equal(t, "registry.f110.dev/build/bazel:latest@sha256:abcdef", updated.Container)
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

func TestRunningSameJob(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name     string
		taskList []*database.Task
		jobName  string
		want     int32 // task id of the running task, 0 means not running
	}{
		{
			name: "Same job is still running",
			taskList: []*database.Task{
				{Id: 1, JobName: "test", CreatedAt: now},
			},
			jobName: "test",
			want:    1,
		},
		{
			name: "Only a different job is running",
			taskList: []*database.Task{
				{Id: 1, JobName: "other", CreatedAt: now},
			},
			jobName: "test",
			want:    0,
		},
		{
			name: "Same job has already finished",
			taskList: []*database.Task{
				{Id: 1, JobName: "test", CreatedAt: now, FinishedAt: &now},
			},
			jobName: "test",
			want:    0,
		},
		{
			name: "Same job started but timed out",
			taskList: []*database.Task{
				{Id: 1, JobName: "test", CreatedAt: now.Add(-2 * jobTimeout)},
			},
			jobName: "test",
			want:    0,
		},
		{
			name: "Different job is running but the same job has finished",
			taskList: []*database.Task{
				{Id: 2, JobName: "other", CreatedAt: now},
				{Id: 1, JobName: "test", CreatedAt: now, FinishedAt: &now},
			},
			jobName: "test",
			want:    0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runningSameJob(tc.taskList, tc.jobName)
			if tc.want == 0 {
				assertion.Nil(t, got)
				return
			}
			assertion.NotNil(t, got)
			assertion.Equal(t, tc.want, got.Id)
		})
	}
}

func TestBazelBuilder_ForceStop(t *testing.T) {
	runner := controllertest.NewGenericTestRunner[*batchv1.Job]()
	coreInformer := k8sclient.NewCoreV1Informer(runner.CoreSharedInformerFactory.Cache(), runner.CoreClient.CoreV1, metav1.NamespaceDefault, 30*time.Second)
	batchInformer := k8sclient.NewBatchV1Informer(runner.CoreSharedInformerFactory.Cache(), runner.CoreClient.BatchV1, metav1.NamespaceDefault, 30*time.Second)
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
			BatchInformer:     batchInformer,
			CoreInformer:      coreInformer,
			Client:            &runner.CoreClient.Set,
			SecretStoreClient: fakesecretstoreclient.NewSimpleClientset(),
		},
		dao.Options{
			Repository: mockDAO.Repository,
			Task:       mockDAO.Task,
		},
		metav1.NamespaceDefault,
		nil,
		"foo",
		storage.S3Options{},
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

	updatedJob, err := runner.CoreClient.BatchV1.GetJob(t.Context(), metav1.NamespaceDefault, t.Name(), metav1.GetOptions{})
	require.NoError(t, err)
	assertion.Contains(t, updatedJob.GetLabels(), labelKeyForceStop)
}
