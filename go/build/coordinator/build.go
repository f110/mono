package coordinator

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v49/github"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	batchv1informers "k8s.io/client-go/informers/batch/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	secretstoreclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"

	"go.f110.dev/mono/go/build/cmd/sidecar"
	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/watcher"
	"go.f110.dev/mono/go/k8s/k8sfactory"
	"go.f110.dev/mono/go/k8s/k8smanifest"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
	"go.f110.dev/mono/go/varptr"
	"go.f110.dev/mono/go/vault"
)

const (
	sidecarImage        = "registry.f110.dev/build/sidecar"
	bazelImage          = "l.gcr.io/google/bazel"
	defaultBazelVersion = "3.1.0"

	defaultCPULimit    = "1000m"
	defaultMemoryLimit = "4096Mi"

	labelKeyRepoId = "build.f110.dev/repo-id"
	labelKeyJobId  = "build.f110.dev/job-id"
	labelKeyTaskId = "build.f110.dev/task-id"
	labelKeyCtrlBy = "build.f110.dev/control-by"

	jobTimeout = 1 * time.Hour
	jobType    = "bazelBuilder"
)

var (
	ErrOtherTaskIsRunning = xerrors.New("coordinator: Other task is running")
)

type KubernetesOptions struct {
	JobInformer        batchv1informers.JobInformer
	PodInformer        corev1informers.PodInformer
	Client             kubernetes.Interface
	SecretStoreClient  secretstoreclient.Interface
	RESTConfig         *rest.Config
	DefaultCPULimit    string
	DefaultMemoryLimit string
}

func NewKubernetesOptions(
	jInformer batchv1informers.JobInformer,
	podI corev1informers.PodInformer,
	c kubernetes.Interface,
	ssc secretstoreclient.Interface,
	cfg *rest.Config,
	cpuLimit, memoryLimit string,
) KubernetesOptions {
	return KubernetesOptions{
		JobInformer:        jInformer,
		PodInformer:        podI,
		Client:             c,
		SecretStoreClient:  ssc,
		RESTConfig:         cfg,
		DefaultCPULimit:    cpuLimit,
		DefaultMemoryLimit: memoryLimit,
	}
}

type BazelOptions struct {
	RemoteCache          string
	EnableRemoteAssetApi bool
	SidecarImage         string
	BazelImage           string
	UseBazelisk          bool
	DefaultVersion       string
	PullAlways           bool
	BazelMirrorURL       string
	GithubAppId          int64
	GithubInstallationId int64
	GithubAppSecretName  string
}

func NewBazelOptions(remoteCache string, enableRemoteAssetApi bool, sidecarImage, bazelImage string, useBazelisk bool, defaultVersion, bazelMirrorURL string, pullAlways bool, githubAppId, githubInstallationId int64, githubAppSecretName string) BazelOptions {
	return BazelOptions{
		RemoteCache:          remoteCache,
		EnableRemoteAssetApi: enableRemoteAssetApi,
		SidecarImage:         sidecarImage,
		BazelImage:           bazelImage,
		UseBazelisk:          useBazelisk,
		DefaultVersion:       defaultVersion,
		BazelMirrorURL:       bazelMirrorURL,
		PullAlways:           pullAlways,
		GithubAppId:          githubAppId,
		GithubInstallationId: githubInstallationId,
		GithubAppSecretName:  githubAppSecretName,
	}
}

type taskQueue struct {
	mu     sync.Mutex
	queues map[string][]*database.Task
}

func newTaskQueue() *taskQueue {
	return &taskQueue{queues: make(map[string][]*database.Task)}
}

func (tq *taskQueue) Enqueue(job *config.Job, task *database.Task) {
	tq.EnqueueById(job.Identification(), task)
}

func (tq *taskQueue) EnqueueById(id string, task *database.Task) {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	tq.queues[id] = append(tq.queues[id], task)
}

func (tq *taskQueue) Dequeue(job *config.Job) *database.Task {
	return tq.DequeueById(job.Identification())
}

func (tq *taskQueue) DequeueById(id string) *database.Task {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	q, ok := tq.queues[id]
	if !ok {
		return nil
	}

	switch len(q) {
	case 0:
		return nil
	case 1:
		tq.queues[id] = nil
		return q[0]
	default:
		tq.queues[id] = q[1:]
		return q[0]
	}
}

type BazelBuilder struct {
	Namespace    string
	dashboardUrl string

	client            kubernetes.Interface
	secretStoreClient secretstoreclient.Interface
	jobLister         batchv1listers.JobLister
	podLister         corev1listers.PodLister
	config            *rest.Config
	vaultClient       *vault.Client

	dao                    dao.Options
	githubClient           *github.Client
	minio                  *storage.MinIO
	vaultAddr              string
	remoteCache            string
	remoteAssetApi         bool
	sidecarImage           string
	bazelImage             string
	useBazelisk            bool
	defaultBazelVersion    string
	pullAlways             bool
	bazelMirrorURL         string
	defaultTaskCPULimit    resource.Quantity
	defaultTaskMemoryLimit resource.Quantity
	githubAppId            int64
	githubInstallationId   int64
	githubAppSecretName    string
	dev                    bool

	taskQueue               *taskQueue
	preProcessContainerName string
	buildContainerName      string
	reportContainerName     string
}

func NewBazelBuilder(
	dashboardUrl string,
	kOpt KubernetesOptions,
	daoOpt dao.Options,
	namespace string,
	ghClient *github.Client,
	bucket string,
	minIOOpt storage.MinIOOptions,
	bazelOpt BazelOptions,
	vaultClient *vault.Client,
	dev bool,
) (*BazelBuilder, error) {
	b := &BazelBuilder{
		Namespace:               namespace,
		dashboardUrl:            dashboardUrl,
		config:                  kOpt.RESTConfig,
		client:                  kOpt.Client,
		secretStoreClient:       kOpt.SecretStoreClient,
		dao:                     daoOpt,
		githubClient:            ghClient,
		minio:                   storage.NewMinIOStorage(bucket, minIOOpt),
		vaultAddr:               vaultClient.Addr(),
		vaultClient:             vaultClient,
		remoteCache:             bazelOpt.RemoteCache,
		remoteAssetApi:          bazelOpt.EnableRemoteAssetApi,
		sidecarImage:            bazelOpt.SidecarImage,
		bazelImage:              bazelOpt.BazelImage,
		useBazelisk:             bazelOpt.UseBazelisk,
		defaultBazelVersion:     bazelOpt.DefaultVersion,
		pullAlways:              bazelOpt.PullAlways,
		bazelMirrorURL:          bazelOpt.BazelMirrorURL,
		githubAppId:             bazelOpt.GithubAppId,
		githubInstallationId:    bazelOpt.GithubInstallationId,
		githubAppSecretName:     bazelOpt.GithubAppSecretName,
		dev:                     dev,
		taskQueue:               newTaskQueue(),
		preProcessContainerName: "pre-process",
		buildContainerName:      "main",
		reportContainerName:     "report",
	}
	if kOpt.JobInformer != nil {
		b.jobLister = kOpt.JobInformer.Lister()
	}
	if kOpt.PodInformer != nil {
		b.podLister = kOpt.PodInformer.Lister()
	}
	if b.sidecarImage == "" {
		b.sidecarImage = sidecarImage
	}
	if b.defaultBazelVersion == "" {
		b.defaultBazelVersion = defaultBazelVersion
	}
	if b.bazelImage == "" {
		b.bazelImage = bazelImage
	}
	if kOpt.DefaultCPULimit != "" {
		q, err := resource.ParseQuantity(kOpt.DefaultCPULimit)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		b.defaultTaskCPULimit = q
	} else {
		b.defaultTaskCPULimit = resource.MustParse(defaultCPULimit)
	}
	if kOpt.DefaultMemoryLimit != "" {
		q, err := resource.ParseQuantity(kOpt.DefaultMemoryLimit)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		b.defaultTaskMemoryLimit = q
	} else {
		b.defaultTaskMemoryLimit = resource.MustParse(defaultMemoryLimit)
	}
	if !b.IsStub() {
		watcher.Router.Add(jobType, b.syncJob)
	}

	pendingTasks, err := b.dao.Task.ListPending(context.Background())
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	for _, v := range pendingTasks {
		b.taskQueue.EnqueueById(v.JobName, v)
	}

	return b, nil
}

func (b *BazelBuilder) Build(ctx context.Context, repo *database.SourceRepository, job *config.Job, revision, bazelVersion, command string, targets, platforms []string, via string, isMainBranch bool) ([]*database.Task, error) {
	var tasks []*database.Task
	defer func() {
		for _, task := range tasks {
			if task.IsChanged() {
				if err := b.dao.Task.Update(ctx, task); err != nil {
					logger.Log.Warn("Failed update task", zap.Error(err))
				}
			}
		}
	}()

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(job); err != nil {
		return nil, err
	}
	jobConfiguration := buf.String()
	t := strings.Join(targets, "\n")
	for _, platform := range platforms {
		task, err := b.dao.Task.Create(ctx, &database.Task{
			RepositoryId:     repo.Id,
			JobName:          job.Name,
			JobConfiguration: jobConfiguration,
			Revision:         revision,
			IsTrunk:          isMainBranch,
			BazelVersion:     bazelVersion,
			Command:          command,
			Targets:          t,
			Platform:         platform,
			Via:              via,
			ConfigName:       job.ConfigName,
		})
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		tasks = append(tasks, task)

		if err := b.buildJob(ctx, repo, job, task); err != nil {
			if errors.Is(err, ErrOtherTaskIsRunning) {
				logger.Log.Info("Enqueue the task", zap.Int32("task.id", task.Id))
				b.taskQueue.Enqueue(job, task)
				return tasks, nil
			}

			return nil, xerrors.WithStack(err)
		}

		if job.GitHubStatus {
			if err := b.updateGithubStatus(ctx, repo, job, task, "pending"); err != nil {
				logger.Log.Warn("Failure update the status of github", zap.Error(err), zap.Int32("task.id", task.Id))
			}
		}
	}

	return tasks, nil
}

func (b *BazelBuilder) IsStub() bool {
	return b.client == nil || b.jobLister == nil || b.podLister == nil
}

// syncJob is the reconcile function.
// If BazelBuilder is running stub mode, syncJob is never triggered.
func (b *BazelBuilder) syncJob(job *batchv1.Job) error {
	if !job.DeletionTimestamp.IsZero() {
		logger.Log.Debug("Job has been deleted", zap.String("job.name", job.Name))
		return nil
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	repoId, err := strconv.Atoi(job.Labels[labelKeyRepoId])
	if err != nil {
		return err
	}
	repo, err := b.dao.Repository.Select(ctx, int32(repoId))
	if err != nil {
		return err
	}

	taskId := job.Labels[labelKeyTaskId]
	task, err := b.getTask(taskId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Log.Info("Not found task", zap.String("task.id", taskId))
			if err := b.teardownJob(ctx, job); err != nil {
				return xerrors.WithStack(err)
			}
			return nil
		}
		return xerrors.WithStack(err)
	}
	jobConfiguration := &config.Job{}
	if err := json.Unmarshal([]byte(task.JobConfiguration), jobConfiguration); err != nil {
		return nil
	}

	if task.FinishedAt != nil {
		logger.Log.Debug("task is already finished", zap.String("job.name", job.Name), zap.Int32("task_id", task.Id))
		if job.DeletionTimestamp.IsZero() {
			if err := b.teardownJob(ctx, job); err != nil {
				return xerrors.WithStack(err)
			}
		}
		return nil
	}

	// Timed out
	if job.CreationTimestamp.Add(jobTimeout).Before(time.Now()) {
		logger.Log.Info("Job is timed out", zap.String("job.name", job.Name), zap.Int32("task_id", task.Id))
		now := time.Now()
		task.FinishedAt = &now
		if err := b.dao.Task.Update(context.Background(), task); err != nil {
			return xerrors.WithStack(err)
		}
		return nil
	}

	if len(job.Status.Conditions) == 0 {
		logger.Log.Debug("Skip job due to the job doesn't have Conditions")
		return nil
	}

	now := time.Now()
	for _, v := range job.Status.Conditions {
		switch v.Type {
		case batchv1.JobComplete:
			if task.FinishedAt == nil {
				if err := b.postProcess(ctx, job, repo, jobConfiguration, task, true); err != nil {
					return xerrors.WithStack(err)
				}
			}
			task.Success = true
			task.FinishedAt = &now
			logger.Log.Info("Job was finished successfully", zap.String("job.name", job.Name), zap.Int32("task_id", task.Id))
		case batchv1.JobFailed:
			if task.FinishedAt == nil {
				if err := b.postProcess(ctx, job, repo, jobConfiguration, task, false); err != nil {
					return xerrors.WithStack(err)
				}
			}
			task.FinishedAt = &now
			logger.Log.Info("Job was failed", zap.String("job.name", job.Name), zap.Int32("task_id", task.Id))
		}
	}
	if task.FinishedAt != nil {
		if err := b.teardownJob(ctx, job); err != nil {
			return xerrors.WithStack(err)
		}
	}

	if err := b.dao.Task.Update(context.Background(), task); err != nil {
		return xerrors.WithStack(err)
	}

	if followTask := b.taskQueue.DequeueById(task.JobName); followTask != nil {
		logger.Log.Info("Dequeue the task", zap.Int32("task.id", followTask.Id))
		if err := b.buildJob(ctx, repo, jobConfiguration, followTask); err != nil {
			logger.Log.Warn("Failed starting follow task. You have to start a task manually", zap.Error(err), zap.String("job.name", task.JobName), zap.Int32("task.id", task.Id))
			return nil
		}
		if err := b.dao.Task.Update(context.Background(), followTask); err != nil {
			logger.Log.Warn("Failed update the task", zap.Error(err), zap.Int32("task.id", followTask.Id))
		}
	}

	return nil
}

func (b *BazelBuilder) teardownJob(ctx context.Context, job *batchv1.Job) error {
	if err := b.client.BatchV1().Jobs(job.Namespace).Delete(ctx, job.Name, metav1.DeleteOptions{}); err != nil {
		return xerrors.WithStack(err)
	}
	pods, err := b.client.CoreV1().Pods(job.Namespace).List(ctx, metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(job.Spec.Selector)})
	if err != nil {
		return xerrors.WithStack(err)
	}
	for _, v := range pods.Items {
		if err := b.client.CoreV1().Pods(v.Namespace).Delete(ctx, v.Name, metav1.DeleteOptions{}); err != nil {
			return xerrors.WithStack(err)
		}
	}

	for _, v := range job.Spec.Template.Spec.Volumes {
		if v.CSI == nil {
			continue
		}
		if v.CSI.Driver != "secrets-store.csi.k8s.io" {
			continue
		}
		err = b.secretStoreClient.SecretsstoreV1().SecretProviderClasses(job.Namespace).Delete(ctx, v.CSI.VolumeAttributes["secretProviderClass"], metav1.DeleteOptions{})
		if err != nil {
			return xerrors.WithStack(err)
		}
	}

	for _, v := range job.Spec.Template.Spec.Containers[0].EnvFrom {
		if v.SecretRef == nil {
			continue
		}
		if v.SecretRef.Name != job.Name {
			continue
		}
		err = b.client.CoreV1().Secrets(job.Namespace).Delete(ctx, job.Name, metav1.DeleteOptions{})
		if err != nil {
			return xerrors.WithStack(err)
		}
	}

	return nil
}

func (b *BazelBuilder) getTask(taskId string) (*database.Task, error) {
	id, err := strconv.Atoi(taskId)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	task, err := b.dao.Task.Select(context.Background(), int32(id))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return task, nil
}

func (b *BazelBuilder) buildJob(ctx context.Context, repo *database.SourceRepository, job *config.Job, task *database.Task) error {
	if job.Exclusive && b.isRunningJob(job) {
		return xerrors.WithStack(ErrOtherTaskIsRunning)
	}

	builtObjects, err := b.buildJobTemplate(repo, job, task, task.Platform)
	if err != nil {
		return err
	}
	for _, v := range builtObjects {
		switch obj := v.(type) {
		case *batchv1.Job:
			if b.IsStub() {
				m, _ := k8smanifest.Marshal(obj)
				logger.Log.Info("Create Job", zap.String("manifest", string(m)))
			} else {
				_, err := b.client.BatchV1().Jobs(b.Namespace).Create(ctx, obj, metav1.CreateOptions{})
				if err != nil {
					return xerrors.WithStack(err)
				}
			}
		case *corev1.Secret:
			if b.IsStub() {
				m, _ := k8smanifest.Marshal(obj)
				logger.Log.Info("Create Secret", zap.String("manifest", string(m)))
			} else {
				_, err := b.client.CoreV1().Secrets(b.Namespace).Create(ctx, obj, metav1.CreateOptions{})
				if err != nil {
					return xerrors.WithStack(err)
				}
			}
		case *corev1.ServiceAccount:
			if b.IsStub() {
				m, _ := k8smanifest.Marshal(obj)
				logger.Log.Info("Create ServiceAccount", zap.String("manifest", string(m)))
			} else {
				_, err := b.client.CoreV1().ServiceAccounts(b.Namespace).Get(ctx, obj.Name, metav1.GetOptions{})
				if kerrors.IsNotFound(err) {
					if _, err := b.client.CoreV1().ServiceAccounts(b.Namespace).Create(ctx, obj, metav1.CreateOptions{}); err != nil {
						return xerrors.WithStack(err)
					}
				}
			}
		case *secretsstorev1.SecretProviderClass:
			if b.IsStub() {
				m, _ := k8smanifest.Marshal(obj)
				logger.Log.Info("Create SecretProviderClass", zap.String("manifest", string(m)))
			} else {
				_, err := b.secretStoreClient.SecretsstoreV1().SecretProviderClasses(b.Namespace).Create(ctx, obj, metav1.CreateOptions{})
				if err != nil {
					return xerrors.WithStack(err)
				}
			}
		}
	}
	now := time.Now()
	task.StartAt = &now

	var buf bytes.Buffer
	for _, v := range builtObjects {
		if _, ok := v.(*batchv1.Job); !ok {
			continue
		}

		if err := k8smanifest.NewEncoder(&buf).Encode(v); err != nil {
			return err
		}
		break
	}
	task.Manifest = buf.String()

	return nil
}

func (b *BazelBuilder) isRunningJob(job *config.Job) bool {
	if b.IsStub() {
		return false
	}

	jobs, err := b.jobLister.List(labels.Everything())
	if err != nil {
		logger.Log.Warn("Could not get a job's list from kube-apiserver.", zap.Error(err))
		// Can not detect a status of job.
		// In this situation, we assume that the job is running.
		return true
	}

	for _, v := range jobs {
		jobId, ok := v.Labels[labelKeyJobId]
		if !ok {
			continue
		}
		if job.Identification() == jobId {
			if v.Status.CompletionTime.IsZero() && !v.Status.StartTime.IsZero() {
				return true
			}
		}
	}

	return false
}

func (b *BazelBuilder) postProcess(ctx context.Context, job *batchv1.Job, repo *database.SourceRepository, jobConfiguration *config.Job, task *database.Task, success bool) error {
	podList, err := b.client.CoreV1().Pods(b.Namespace).List(ctx, metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(job.Spec.Selector)})
	if err != nil {
		return xerrors.WithStack(err)
	}
	if len(podList.Items) != 1 {
		return xerrors.New("Target pods not found or found more than 1")
	}
	if len(podList.Items[0].Status.ContainerStatuses) > 0 {
		image := podList.Items[0].Status.ContainerStatuses[0].Image
		if i := strings.Index(podList.Items[0].Status.ContainerStatuses[0].ImageID, "@"); i > 0 {
			image += podList.Items[0].Status.ContainerStatuses[0].ImageID[i:]
		}
		task.Container = image
	}
	buildPod := podList.Items[0]

	buf := new(bytes.Buffer)
	logReq := b.client.CoreV1().Pods(b.Namespace).GetLogs(buildPod.Name, &corev1.PodLogOptions{Container: b.preProcessContainerName})
	rawLog, err := logReq.DoRaw(ctx)
	if err != nil {
		return xerrors.WithStack(err)
	}
	buf.WriteString("----- pre-process -----\n")
	buf.Write(rawLog)

	logReq = b.client.CoreV1().Pods(b.Namespace).GetLogs(buildPod.Name, &corev1.PodLogOptions{Container: b.buildContainerName})
	rawLog, err = logReq.DoRaw(ctx)
	if err != nil {
		return xerrors.WithStack(err)
	}
	buf.WriteString("\n")
	buf.WriteString("----- main -----\n")
	buf.Write(rawLog)

	var testReport []byte
	if jobConfiguration.Command == "test" && task.IsTrunk {
		logReq = b.client.CoreV1().Pods(b.Namespace).GetLogs(buildPod.Name, &corev1.PodLogOptions{Container: b.reportContainerName})
		rawLog, err = logReq.DoRaw(ctx)
		if err != nil {
			return xerrors.WithStack(err)
		}
		testReport = rawLog
	}

	if err := b.minio.Put(context.Background(), job.Name, buf.Bytes()); err != nil {
		return xerrors.WithStack(err)
	}
	task.LogFile = job.Name

	s, err := metav1.LabelSelectorAsSelector(job.Spec.Selector)
	if err != nil {
		return xerrors.WithStack(err)
	}
	pods, err := b.podLister.Pods(b.Namespace).List(s)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if len(pods) > 0 {
		nodeList, err := b.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return xerrors.WithStack(err)
		}
	NodeList:
		for _, v := range nodeList.Items {
			for _, a := range v.Status.Addresses {
				if a.Type == corev1.NodeInternalIP &&
					a.Address == pods[0].Status.HostIP {
					task.Node = v.Name
					break NodeList
				}
			}
		}
	}

	if len(testReport) > 0 {
		if err := b.updateTestReport(ctx, testReport, repo); err != nil {
			logger.Log.Warn("Failed to parse the report json", logger.Error(err))
		}
	}

	if jobConfiguration.GitHubStatus {
		state := "success"
		if !success {
			state = "failure"
		}
		if err := b.updateGithubStatus(context.Background(), repo, jobConfiguration, task, state); err != nil {
			return xerrors.WithStack(err)
		}
	}

	return nil
}

func (b *BazelBuilder) updateTestReport(ctx context.Context, reportJSON []byte, repo *database.SourceRepository) error {
	var report sidecar.TestReport
	if err := json.Unmarshal(reportJSON, &report); err != nil {
		return xerrors.WithStack(err)
	}

	for _, s := range report.Tests {
		if s.Status == sidecar.TestStatusFailed {
			continue
		}

		_, err := b.dao.TestReport.Create(ctx, &database.TestReport{
			RepositoryId: repo.Id,
			Label:        s.Label,
			Duration:     s.Duration,
			StartAt:      s.StartAt,
		})
		if err != nil {
			return xerrors.WithStack(err)
		}
	}

	return nil
}

func (b *BazelBuilder) updateGithubStatus(ctx context.Context, repo *database.SourceRepository, job *config.Job, task *database.Task, state string) error {
	if task.Revision == "" {
		return nil
	}

	u, err := url.Parse(repo.Url)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if u.Hostname() != "github.com" {
		logger.Log.Warn("Expect to update a status of github. but repository url is not github.com", zap.String("url", repo.Url))
		return nil
	}
	// u.Path is /owner/repo if URL is github.com.
	s := strings.Split(u.Path, "/")
	owner, repoName := s[1], s[2]

	targetUrl := ""
	if state == "success" || state == "failure" {
		targetUrl = fmt.Sprintf("%s/task/%d", b.dashboardUrl, task.Id)
	}

	_, _, err = b.githubClient.Repositories.CreateStatus(
		ctx,
		owner,
		repoName,
		task.Revision,
		&github.RepoStatus{
			State:     github.String(state),
			Context:   github.String(fmt.Sprintf("%s %s", task.Command, job.Name)),
			TargetURL: github.String(targetUrl),
		},
	)
	if err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (b *BazelBuilder) buildJobTemplate(repo *database.SourceRepository, job *config.Job, task *database.Task, platform string) ([]runtime.Object, error) {
	var builtObjects []runtime.Object
	repoIdString := fmt.Sprintf("%d", repo.Id)
	taskIdString := strconv.Itoa(int(task.Id))

	sa := k8sfactory.ServiceAccountFactory(nil,
		k8sfactory.Namef("build-%s", job.RepositoryName),
		k8sfactory.Namespace(b.Namespace),
	)
	builtObjects = append(builtObjects, sa)
	workDir := k8sfactory.NewEmptyDirVolumeSource("workdir", "/work")
	var commDir *k8sfactory.VolumeSource
	if job.Command == "test" && task.IsTrunk {
		commDir = k8sfactory.NewEmptyDirVolumeSource("comm", "/comm")
	}
	buildPod := k8sfactory.PodFactory(nil,
		k8sfactory.Labels(map[string]string{
			labelKeyTaskId: taskIdString,
			labelKeyCtrlBy: "bazel-build",
		}),
		k8sfactory.RestartPolicy(corev1.RestartPolicyNever),
		k8sfactory.Volume(workDir),
		k8sfactory.Volume(commDir),
		k8sfactory.ServiceAccount(fmt.Sprintf("build-%s", job.RepositoryName)),
	)
	preProcessContainer := k8sfactory.ContainerFactory(nil,
		k8sfactory.Name(b.preProcessContainerName),
		k8sfactory.Image(b.sidecarImage, nil),
		k8sfactory.PullPolicy(corev1.PullIfNotPresent),
		k8sfactory.Volume(workDir),
	)
	mainContainer := k8sfactory.ContainerFactory(nil,
		k8sfactory.Name(b.buildContainerName),
		k8sfactory.PullPolicy(corev1.PullIfNotPresent),
		k8sfactory.WorkDir(workDir.Mount.MountPath),
		k8sfactory.Volume(workDir),
		k8sfactory.Volume(commDir),
	)
	var reportContainer *corev1.Container
	if job.Command == "test" && task.IsTrunk {
		reportContainer = k8sfactory.ContainerFactory(nil,
			k8sfactory.Name(b.reportContainerName),
			k8sfactory.Image(b.sidecarImage, nil),
			k8sfactory.PullPolicy(corev1.PullIfNotPresent),
			k8sfactory.Volume(commDir),
			k8sfactory.Args("report", fmt.Sprintf("--event-binary-file=%s/bep", commDir.Mount.MountPath), "--startup-timeout=10m"),
		)
	}
	imageTag := b.defaultBazelVersion
	if b.useBazelisk {
		imageTag = "bazelisk"
	} else if task.BazelVersion != "" {
		imageTag = task.BazelVersion
	}
	mainContainer = k8sfactory.ContainerFactory(mainContainer, k8sfactory.Image(fmt.Sprintf("%s:%s", b.bazelImage, imageTag), nil))
	if b.useBazelisk && b.bazelMirrorURL != "" {
		mainContainer = k8sfactory.ContainerFactory(mainContainer, k8sfactory.EnvVar("BAZELISK_FORMAT_URL", b.bazelMirrorURL+"/bazel-%v-%o-%m%e"))
	}
	if b.pullAlways {
		mainContainer = k8sfactory.ContainerFactory(mainContainer, k8sfactory.PullPolicy(corev1.PullAlways))
		preProcessContainer = k8sfactory.ContainerFactory(preProcessContainer, k8sfactory.PullPolicy(corev1.PullAlways))
	}

	preProcessArgs := []string{"clone", "--work-dir=work", fmt.Sprintf("--url=%s", repo.CloneUrl)}
	if task.Revision != "" {
		preProcessArgs = append(preProcessArgs, "--commit="+task.Revision)
	}
	if repo.Private {
		preProcessArgs = append(preProcessArgs,
			fmt.Sprintf("--github-app-id=%d", b.githubAppId),
			fmt.Sprintf("--github-installation-id=%d", b.githubInstallationId),
			"--private-key-file=/etc/github/privatekey.pem",
		)
		githubSecretVolume := k8sfactory.NewSecretVolumeSource(
			"github-secret",
			"/etc/github",
			k8sfactory.SecretFactory(nil, k8sfactory.Name(b.githubAppSecretName)),
		)
		preProcessContainer = k8sfactory.ContainerFactory(preProcessContainer, k8sfactory.Volume(githubSecretVolume))
		buildPod = k8sfactory.PodFactory(buildPod, k8sfactory.Volume(githubSecretVolume))
	}
	preProcessContainer = k8sfactory.ContainerFactory(preProcessContainer, k8sfactory.Args(preProcessArgs...))

	cpuLimit := b.defaultTaskCPULimit
	if job.CPULimit != "" {
		q, err := resource.ParseQuantity(job.CPULimit)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		cpuLimit = q
	}
	memoryLimit := b.defaultTaskMemoryLimit
	if job.MemoryLimit != "" {
		q, err := resource.ParseQuantity(job.MemoryLimit)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		memoryLimit = q
	}
	mainContainer = k8sfactory.ContainerFactory(mainContainer, k8sfactory.ResourceLimit(cpuLimit, memoryLimit))

	args := []string{task.Command}
	if b.remoteCache != "" {
		args = append(args, fmt.Sprintf("--remote_cache=%s", b.remoteCache))
		if b.remoteAssetApi {
			args = append(args, fmt.Sprintf("--experimental_remote_downloader=%s", b.remoteCache))
		}
	}
	if task.ConfigName != "" {
		args = append(args, fmt.Sprintf("--config=%s", task.ConfigName))
	}
	var platformName string
	if platform != "" {
		args = append(args, "--platforms="+platform)
		if strings.Contains(platform, ":") {
			s := strings.Split(platform, ":")
			platformName = "-" + strings.Replace(s[1], "_", "-", -1)
		}
	}
	switch job.Command {
	case "test":
		if task.IsTrunk {
			args = append(args,
				fmt.Sprintf("--build_event_binary_file=%s/bep", commDir.Mount.MountPath),
				"--cache_test_results=no",
			)
		}
		args = append(args, "--")
		targets := strings.Split(task.Targets, "\n")
		args = append(args, targets...)
	case "run":
		args = append(args, job.Targets[0])
		if job.Args != nil {
			args = append(args, "--")
			args = append(args, job.Args...)
		}
	}
	mainContainer = k8sfactory.ContainerFactory(mainContainer, k8sfactory.Args(args...))

	if len(job.Secrets) > 0 && b.vaultClient == nil {
		logger.Log.Warn("Secret injection is not supported", zap.String("repo", repo.Name), zap.String("job", job.Name))
	} else {
		secretProviderClasses := make(map[string]*secretsstorev1.SecretProviderClass)
		for _, secret := range job.Secrets {
			if secret.MountPath != "" {
				name := fmt.Sprintf("%s-%d%s-%s", job.RepositoryName, task.Id, platformName, strings.Replace(secret.MountPath[1:], "/", "-", -1))
				if c, ok := secretProviderClasses[secret.MountPath]; !ok {
					secretProviderClasses[secret.MountPath] = &secretsstorev1.SecretProviderClass{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: b.Namespace,
							Labels: map[string]string{
								labelKeyJobId:  job.Identification(),
								labelKeyCtrlBy: "bazel-build",
							},
						},
						Spec: secretsstorev1.SecretProviderClassSpec{
							Provider: "vault",
							Parameters: map[string]string{
								"roleName":     fmt.Sprintf("build-%s", job.RepositoryName),
								"vaultAddress": b.vaultAddr,
								"objects": fmt.Sprintf(
									"- objectName: %q\n  secretPath: %q\n  secretKey: %q\n",
									secret.VaultKey,
									fmt.Sprintf("%s/data/%s", secret.VaultMount, secret.VaultPath),
									secret.VaultKey,
								),
							},
						},
					}
				} else {
					c.Spec.Parameters["objects"] += fmt.Sprintf(
						"- objectName: %q\n  secretPath: %q\n  secretKey: %q\n",
						secret.VaultKey,
						fmt.Sprintf("%s/data/%s", secret.VaultMount, secret.VaultPath),
						secret.VaultKey,
					)
				}
			}
		}
		if len(secretProviderClasses) > 0 {
			for mountPath, class := range secretProviderClasses {
				envVolume := k8sfactory.NewSecretStoreVolumeSource(class.Name, mountPath)
				mainContainer = k8sfactory.ContainerFactory(mainContainer, k8sfactory.Volume(envVolume))
				buildPod = k8sfactory.PodFactory(buildPod, k8sfactory.Volume(envVolume))
				builtObjects = append(builtObjects, class)
			}
		}
	}

	var envSecret *corev1.Secret
	for k, v := range job.Env {
		switch val := v.(type) {
		case string:
			mainContainer = k8sfactory.ContainerFactory(mainContainer, k8sfactory.EnvVar(k, val))
		default:
			logger.Log.Warn("Not supported value type", zap.String("key", k))
		}
	}
	if envSecret != nil {
		builtObjects = append(builtObjects, envSecret)
	}

	builtObjects = append(builtObjects, k8sfactory.JobFactory(nil,
		k8sfactory.Namef("%s-%d%s", job.RepositoryName, task.Id, platformName),
		k8sfactory.Namespace(b.Namespace),
		k8sfactory.Labels(map[string]string{
			labelKeyRepoId:    repoIdString,
			labelKeyJobId:     job.Identification(),
			labelKeyTaskId:    taskIdString,
			labelKeyCtrlBy:    "bazel-build",
			watcher.TypeLabel: jobType,
		}),
		k8sfactory.BackoffLimit(0),
		k8sfactory.PodFailurePolicy(batchv1.PodFailurePolicyRule{
			Action: batchv1.PodFailurePolicyActionFailJob,
			OnExitCodes: &batchv1.PodFailurePolicyOnExitCodesRequirement{
				ContainerName: varptr.Ptr(b.buildContainerName),
				Operator:      batchv1.PodFailurePolicyOnExitCodesOpNotIn,
				Values:        []int32{0},
			},
		}),
		k8sfactory.Pod(
			k8sfactory.PodFactory(buildPod,
				k8sfactory.InitContainer(preProcessContainer),
				k8sfactory.Container(mainContainer),
				k8sfactory.Container(reportContainer),
			),
		),
	))
	return builtObjects, nil
}
