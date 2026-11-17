package coordinator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.f110.dev/kubeproto/go/apis/batchv1"
	"go.f110.dev/kubeproto/go/apis/corev1"
	"go.f110.dev/kubeproto/go/apis/metav1"
	"go.f110.dev/kubeproto/go/k8sclient"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/apimachinery/pkg/runtime"
	fakesecretstoreclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/database/dao/daotest"
	"go.f110.dev/mono/go/git"
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
			k8sfactory.MatchLabelSelector(map[string]string{labelKeyRepoId: "1", labelKeyTaskId: "2"}),
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
		finalizerRemoved := k8sfactory.JobFactory(target, k8sfactory.RemoveFinalizer(bazelBuilderControllerFinalizerName))
		runner.AssertUpdateAction(t, "", finalizerRemoved)
		runner.AssertDeleteAction(t, finalizerRemoved)
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
			k8sfactory.MatchLabelSelector(map[string]string{labelKeyRepoId: "1", labelKeyTaskId: "4"}),
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
		finalizerRemoved := k8sfactory.JobFactory(target, k8sfactory.RemoveFinalizer(bazelBuilderControllerFinalizerName))
		runner.AssertUpdateAction(t, "", finalizerRemoved)
		runner.AssertDeleteAction(t, finalizerRemoved)
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

func TestNewBazelBuilder_CloneFromGitDataService(t *testing.T) {
	newBuilder := func(opt BazelOptions) *BazelBuilder {
		mockTask := daotest.NewTask()
		b, err := NewBazelBuilder(
			"",
			KubernetesOptions{},
			dao.Options{Task: mockTask},
			metav1.NamespaceDefault,
			nil,
			"foo",
			storage.S3Options{},
			opt,
			nil,
			nil,
			true, // dev mode to skip watcher registration
		)
		require.NoError(t, err)
		return b
	}

	t.Run("Clone from GitHub by default even if git-data-service-url is set", func(t *testing.T) {
		b := newBuilder(BazelOptions{GitDataServiceURL: "git-data.default.svc:9020"})
		assertion.Equal(t, "", b.jobBuilder.gitDataServiceURL)
	})

	t.Run("Clone from git-data-service when the flag is set", func(t *testing.T) {
		b := newBuilder(BazelOptions{GitDataServiceURL: "git-data.default.svc:9020", CloneFromGitDataService: true})
		assertion.Equal(t, "git-data.default.svc:9020", b.jobBuilder.gitDataServiceURL)
	})
}

type fakeGitDataClient struct {
	git.GitDataClient
	commits map[string]*git.Commit
}

func (f *fakeGitDataClient) GetCommit(_ context.Context, in *git.RequestGetCommit, _ ...grpc.CallOption) (*git.ResponseGetCommit, error) {
	c, ok := f.commits[in.Repo+"@"+in.Sha]
	if !ok {
		return nil, status.Error(codes.NotFound, "commit not found")
	}
	return &git.ResponseGetCommit{Commit: c}, nil
}

func TestBazelBuilder_BuildJobTemplate(t *testing.T) {
	job, repo, task, _, _ := testJobBuilderFixtures()
	repo.Name = "f110/example"

	newBuilder := func(client git.GitDataClient) *BazelBuilder {
		mockTask := daotest.NewTask()
		b, err := NewBazelBuilder("", KubernetesOptions{}, dao.Options{Task: mockTask}, metav1.NamespaceDefault, nil, "foo", storage.S3Options{}, BazelOptions{}, nil, nil, true)
		require.NoError(t, err)
		b.GitDataClient(client)
		return b
	}
	mainContainerEnv := func(t *testing.T, objs []runtime.Object) map[string]string {
		for _, obj := range objs {
			j, ok := obj.(*batchv1.Job)
			if !ok {
				continue
			}
			env := make(map[string]string)
			for _, c := range j.Spec.Template.Spec.Containers {
				if c.Name != "main" {
					continue
				}
				for _, e := range c.Env {
					env[e.Name] = e.Value
				}
			}
			return env
		}
		t.Fatal("Job is not built")
		return nil
	}

	t.Run("Pass the commit to the build", func(t *testing.T) {
		b := newBuilder(&fakeGitDataClient{commits: map[string]*git.Commit{
			"f110/example@" + task.Revision: {
				Sha:       task.Revision,
				Committer: &git.Signature{When: timestamppb.New(time.Date(2026, 11, 13, 14, 3, 20, 0, time.UTC))},
			},
		}})
		objs, err := b.buildJobTemplate(context.Background(), repo, job, task, "@rules_go//go/toolchain:linux_amd64")
		require.NoError(t, err)
		env := mainContainerEnv(t, objs)
		assertion.Equal(t, env["BUILD_COMMIT_SHA"], task.Revision)
		assertion.Equal(t, env["BUILD_COMMIT_TIME"], "2026-11-13T14:03:20Z")
	})

	t.Run("Build without the commit if it is not found", func(t *testing.T) {
		b := newBuilder(&fakeGitDataClient{})
		objs, err := b.buildJobTemplate(context.Background(), repo, job, task, "@rules_go//go/toolchain:linux_amd64")
		require.NoError(t, err)
		env := mainContainerEnv(t, objs)
		assertion.NotContains(t, env, "BUILD_COMMIT_SHA")
		assertion.NotContains(t, env, "BUILD_COMMIT_TIME")
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
	// Create the job only in kube-apiserver. The informer is not running on the builder which is not the leader.
	_, err = runner.CoreClient.BatchV1.CreateJob(t.Context(), target, metav1.CreateOptions{})
	require.NoError(t, err)

	err = b.ForceStop(t.Context(), 1)
	require.NoError(t, err)

	updatedJob, err := runner.CoreClient.BatchV1.GetJob(t.Context(), metav1.NamespaceDefault, t.Name(), metav1.GetOptions{})
	require.NoError(t, err)
	assertion.Contains(t, updatedJob.GetLabels(), labelKeyForceStop)
}

func TestBazelBuilder_Build(t *testing.T) {
	runner := controllertest.NewGenericTestRunner[*batchv1.Job]()
	coreInformer := k8sclient.NewCoreV1Informer(runner.CoreSharedInformerFactory.Cache(), runner.CoreClient.CoreV1, metav1.NamespaceDefault, 30*time.Second)
	batchInformer := k8sclient.NewBatchV1Informer(runner.CoreSharedInformerFactory.Cache(), runner.CoreClient.BatchV1, metav1.NamespaceDefault, 30*time.Second)
	mockTask := daotest.NewTask()
	b, err := NewBazelBuilder(
		"",
		KubernetesOptions{
			BatchInformer:     batchInformer,
			CoreInformer:      coreInformer,
			Client:            &runner.CoreClient.Set,
			SecretStoreClient: fakesecretstoreclient.NewSimpleClientset(),
		},
		dao.Options{Task: mockTask},
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

	repo := &database.SourceRepository{Id: 1, Url: "https://github.com/f110/mono"}
	job := &config.JobV2{Name: "test_all", Command: "test", Targets: []string{"//..."}, RepositoryOwner: "f110", RepositoryName: "mono"}
	mockTask.RegisterListByRevision(repo.Id, "e192aef54cb0d31afd7cae64b079be2a12a56a74", []*database.Task{}, nil)

	tasks, err := b.Build(t.Context(), repo, job, "e192aef54cb0d31afd7cae64b079be2a12a56a74", "", "test", job.Targets, []string{"linux_amd64"}, "api", false)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assertion.Nil(t, tasks[0].StartAt)

	// Build creates only the task. The task is started by the leader.
	jobs, err := runner.CoreClient.BatchV1.ListJob(t.Context(), metav1.NamespaceDefault, metav1.ListOptions{})
	require.NoError(t, err)
	assertion.Len(t, jobs.Items, 0)
}

func TestBazelBuilder_StartPendingTasks(t *testing.T) {
	newPendingTask := func(t *testing.T, job *config.JobV2) *database.Task {
		jobConfiguration, err := config.MarshalJob(job)
		require.NoError(t, err)
		task := &database.Task{
			Id:                     1,
			RepositoryId:           1,
			JobName:                job.Name,
			ParsedJobConfiguration: jobConfiguration,
			Revision:               "e192aef54cb0d31afd7cae64b079be2a12a56a74",
			Command:                job.Command,
			Targets:                strings.Join(job.Targets, "\n"),
			Platform:               "linux_amd64",
		}
		task.ResetMark()
		return task
	}
	listJobs := func(t *testing.T, runner *controllertest.GenericTestRunner[*batchv1.Job]) []batchv1.Job {
		jobs, err := runner.CoreClient.BatchV1.ListJob(t.Context(), metav1.NamespaceDefault, metav1.ListOptions{})
		require.NoError(t, err)
		return jobs.Items
	}

	const (
		started = iota
		waiting
		failed
	)
	cases := []struct {
		name                string
		exclusive           bool
		brokenConfiguration bool
		running             []*database.Task
		want                int
	}{
		{
			name:      "Start the pending task",
			exclusive: true,
			running:   []*database.Task{},
			want:      started,
		},
		{
			name:      "Wait for the running task of the exclusive job",
			exclusive: true,
			running:   []*database.Task{{Id: 2, RepositoryId: 1, JobName: "test_all"}},
			want:      waiting,
		},
		{
			name:      "Start the task of the job which is not exclusive even if the other task is running",
			exclusive: false,
			running:   []*database.Task{{Id: 2, RepositoryId: 1, JobName: "test_all"}},
			want:      started,
		},
		{
			name:                "Finish the task which has the broken job configuration",
			brokenConfiguration: true,
			running:             []*database.Task{},
			want:                failed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := controllertest.NewGenericTestRunner[*batchv1.Job]()
			coreInformer := k8sclient.NewCoreV1Informer(runner.CoreSharedInformerFactory.Cache(), runner.CoreClient.CoreV1, metav1.NamespaceDefault, 30*time.Second)
			batchInformer := k8sclient.NewBatchV1Informer(runner.CoreSharedInformerFactory.Cache(), runner.CoreClient.BatchV1, metav1.NamespaceDefault, 30*time.Second)
			mockRepository := daotest.NewSourceRepository()
			mockTask := daotest.NewTask()
			b, err := NewBazelBuilder(
				"",
				KubernetesOptions{
					BatchInformer:     batchInformer,
					CoreInformer:      coreInformer,
					Client:            &runner.CoreClient.Set,
					SecretStoreClient: fakesecretstoreclient.NewSimpleClientset(),
				},
				dao.Options{Repository: mockRepository, Task: mockTask},
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

			job := &config.JobV2{Name: "test_all", Command: "test", Targets: []string{"//..."}, Exclusive: tc.exclusive}
			pendingTask := newPendingTask(t, job)
			if tc.brokenConfiguration {
				pendingTask.ParsedJobConfiguration = []byte("{")
			}
			mockRepository.RegisterSelect(1, &database.SourceRepository{Id: 1, Name: "f110/mono", Url: "https://github.com/f110/mono"})
			mockTask.RegisterListPending([]*database.Task{pendingTask}, nil)
			mockTask.RegisterListRunning(1, job.Name, tc.running, nil)

			err = b.startPendingTasks(t.Context())
			require.NoError(t, err)

			called := mockTask.Called("Update")
			switch tc.want {
			case started:
				require.Len(t, called, 1)
				updated := called[0].Args["task"].(*database.Task)
				assertion.NotNil(t, updated.StartAt)
				assertion.Nil(t, updated.FinishedAt)
				assertion.Len(t, listJobs(t, runner), 1)
			case waiting:
				assertion.Len(t, called, 0)
				assertion.Len(t, listJobs(t, runner), 0)
			case failed:
				require.Len(t, called, 1)
				updated := called[0].Args["task"].(*database.Task)
				assertion.NotNil(t, updated.FinishedAt)
				assertion.False(t, updated.Success)
				assertion.Len(t, listJobs(t, runner), 0)
			}
		})
	}
}
