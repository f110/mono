package coordinator

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v32/github"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	batchv1informers "k8s.io/client-go/informers/batch/v1"
	"k8s.io/client-go/kubernetes"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
	"k8s.io/client-go/rest"

	"go.f110.dev/mono/lib/logger"
	"go.f110.dev/mono/tools/build/pkg/database"
	"go.f110.dev/mono/tools/build/pkg/database/dao"
	"go.f110.dev/mono/tools/build/pkg/storage"
	"go.f110.dev/mono/tools/build/pkg/watcher"
)

const (
	sidecarImage        = "registry.f110.dev/build/sidecar"
	bazelImage          = "l.gcr.io/google/bazel"
	defaultBazelVersion = "3.1.0"

	defaultCPULimit    = "1000m"
	defaultMemoryLimit = "4096Mi"

	labelKeyJobId  = "build.f110.dev/job-id"
	labelKeyTaskId = "build.f110.dev/task-id"
	labelKeyCtrlBy = "build.f110.dev/control-by"

	jobTimeout = 1 * time.Hour
	jobType    = "bazelBuilder"
)

var (
	ErrOtherTaskIsRunning = xerrors.New("coordinator: Other task is running")
)

type GithubAppOptions struct {
	AppId          int64
	InstallationId int64
	PrivateKeyFile string
}

func NewGithubAppOptions(appId, installationId int64, privateKeyFile string) GithubAppOptions {
	return GithubAppOptions{AppId: appId, InstallationId: installationId, PrivateKeyFile: privateKeyFile}
}

type KubernetesOptions struct {
	JobInformer        batchv1informers.JobInformer
	Client             kubernetes.Interface
	RESTConfig         *rest.Config
	DefaultCPULimit    string
	DefaultMemoryLimit string
}

func NewKubernetesOptions(jInformer batchv1informers.JobInformer, c kubernetes.Interface, cfg *rest.Config, cpuLimit, memoryLimit string) KubernetesOptions {
	return KubernetesOptions{JobInformer: jInformer, Client: c, RESTConfig: cfg, DefaultCPULimit: cpuLimit, DefaultMemoryLimit: memoryLimit}
}

type BazelOptions struct {
	RemoteCache          string
	EnableRemoteAssetApi bool
	SidecarImage         string
	BazelImage           string
	DefaultVersion       string
}

func NewBazelOptions(remoteCache string, enableRemoteAssetApi bool, sidecarImage, bazelImage, defaultVersion string) BazelOptions {
	return BazelOptions{
		RemoteCache:          remoteCache,
		EnableRemoteAssetApi: enableRemoteAssetApi,
		SidecarImage:         sidecarImage,
		BazelImage:           bazelImage,
		DefaultVersion:       defaultVersion,
	}
}

type taskQueue struct {
	mu     sync.Mutex
	queues map[int32][]*database.Task
}

func newTaskQueue() *taskQueue {
	return &taskQueue{queues: make(map[int32][]*database.Task)}
}

func (tq *taskQueue) Enqueue(job *database.Job, task *database.Task) {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	tq.queues[job.Id] = append(tq.queues[job.Id], task)
}

func (tq *taskQueue) Dequeue(job *database.Job) *database.Task {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	q, ok := tq.queues[job.Id]
	if !ok {
		return nil
	}

	switch len(q) {
	case 0:
		return nil
	case 1:
		tq.queues[job.Id] = nil
		return q[0]
	default:
		tq.queues[job.Id] = q[1:]
		return q[0]
	}
}

type BazelBuilder struct {
	Namespace    string
	dashboardUrl string

	client    kubernetes.Interface
	jobLister batchv1listers.JobLister
	config    *rest.Config

	dao                    dao.Options
	githubClient           *github.Client
	minio                  *storage.MinIO
	remoteCache            string
	remoteAssetApi         bool
	sidecarImage           string
	bazelImage             string
	defaultBazelVersion    string
	defaultTaskCPULimit    resource.Quantity
	defaultTaskMemoryLimit resource.Quantity
	dev                    bool

	taskQueue *taskQueue
}

func NewBazelBuilder(
	dashboardUrl string,
	kOpt KubernetesOptions,
	daoOpt dao.Options,
	namespace string,
	appOpt GithubAppOptions,
	minIOOpt storage.MinIOOptions,
	bazelOpt BazelOptions,
	dev bool,
) (*BazelBuilder, error) {
	t, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appOpt.AppId, appOpt.InstallationId, appOpt.PrivateKeyFile)
	if err != nil {
		return nil, xerrors.Errorf(": %v", err)
	}

	b := &BazelBuilder{
		Namespace:           namespace,
		dashboardUrl:        dashboardUrl,
		config:              kOpt.RESTConfig,
		client:              kOpt.Client,
		jobLister:           kOpt.JobInformer.Lister(),
		dao:                 daoOpt,
		githubClient:        github.NewClient(&http.Client{Transport: t}),
		minio:               storage.NewMinIOStorage(kOpt.Client, kOpt.RESTConfig, minIOOpt, dev),
		remoteCache:         bazelOpt.RemoteCache,
		remoteAssetApi:      bazelOpt.EnableRemoteAssetApi,
		sidecarImage:        bazelOpt.SidecarImage,
		bazelImage:          bazelOpt.BazelImage,
		defaultBazelVersion: bazelOpt.DefaultVersion,
		dev:                 dev,
		taskQueue:           newTaskQueue(),
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
			return nil, xerrors.Errorf(": %w", err)
		}
		b.defaultTaskCPULimit = q
	} else {
		b.defaultTaskCPULimit = resource.MustParse(defaultCPULimit)
	}
	if kOpt.DefaultMemoryLimit != "" {
		q, err := resource.ParseQuantity(kOpt.DefaultMemoryLimit)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		b.defaultTaskMemoryLimit = q
	} else {
		b.defaultTaskMemoryLimit = resource.MustParse(defaultMemoryLimit)
	}
	watcher.Router.Add(jobType, b.syncJob)

	pendingTasks, err := b.dao.Task.ListPending(context.Background())
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	for _, v := range pendingTasks {
		b.taskQueue.Enqueue(v.Job, v)
	}

	return b, nil
}

func (b *BazelBuilder) Build(ctx context.Context, job *database.Job, revision, command, target, via string) (*database.Task, error) {
	task, err := b.dao.Task.Create(ctx, &database.Task{JobId: job.Id, Revision: revision, Command: command, Target: target, Via: via})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	defer func() {
		if task.IsChanged() {
			if err := b.dao.Task.Update(ctx, task); err != nil {
				logger.Log.Warn("Failed update task", zap.Error(err))
			}
		}
	}()

	if err := b.buildJob(job, task); err != nil {
		if errors.Is(err, ErrOtherTaskIsRunning) {
			logger.Log.Info("Enqueue the task", zap.Int32("task.id", task.Id))
			b.taskQueue.Enqueue(job, task)
			return task, nil
		}

		return nil, xerrors.Errorf(": %w", err)
	}

	if job.GithubStatus {
		if err := b.updateGithubStatus(ctx, job, task, "pending"); err != nil {
			logger.Log.Warn("Failure update the status of github", zap.Error(err), zap.Int32("task.id", task.Id))
		}
	}

	return task, nil
}

func (b *BazelBuilder) syncJob(job *batchv1.Job) error {
	if !job.DeletionTimestamp.IsZero() {
		logger.Log.Debug("Job has been deleted", zap.String("job.name", job.Name))
		return nil
	}

	taskId := job.Labels[labelKeyTaskId]
	task, err := b.getTask(taskId)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Log.Info("Not found task", zap.String("task.id", taskId))
			return nil
		}
		return xerrors.Errorf(": %w", err)
	}

	if task.FinishedAt != nil {
		logger.Log.Debug("task is always finished", zap.String("job.name", job.Name), zap.Int32("task_id", task.Id))
		return nil
	}

	// Timed out
	if job.CreationTimestamp.Add(jobTimeout).Before(time.Now()) {
		logger.Log.Info("Job is timed out", zap.String("job.name", job.Name), zap.Int32("task_id", task.Id))
		now := time.Now()
		task.FinishedAt = &now
		if err := b.dao.Task.Update(context.Background(), task); err != nil {
			return xerrors.Errorf(": %w", err)
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
				if err := b.postProcess(job, task, true); err != nil {
					return xerrors.Errorf(": %w", err)
				}
			}
			task.Success = true
			task.FinishedAt = &now
			logger.Log.Info("Job is finished successfully", zap.String("job.name", job.Name), zap.Int32("task_id", task.Id))
		case batchv1.JobFailed:
			if task.FinishedAt == nil {
				if err := b.postProcess(job, task, false); err != nil {
					return xerrors.Errorf(": %w", err)
				}
			}
			task.FinishedAt = &now
			logger.Log.Info("Job is failed", zap.String("job.name", job.Name), zap.Int32("task_id", task.Id))
		}
	}
	if task.FinishedAt != nil {
		if err := b.teardownJob(job); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	if err := b.dao.Task.Update(context.Background(), task); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	if followTask := b.taskQueue.Dequeue(task.Job); followTask != nil {
		logger.Log.Info("Dequeue the task", zap.Int32("task.id", followTask.Id))
		if err := b.buildJob(task.Job, followTask); err != nil {
			logger.Log.Warn("Failed starting follow task. You have to start a task manually", zap.Error(err), zap.Int32("job.id", task.JobId), zap.Int32("task.id", task.Id))
			return nil
		}
		if err := b.dao.Task.Update(context.Background(), followTask); err != nil {
			logger.Log.Warn("Failed update the task", zap.Error(err), zap.Int32("task.id", followTask.Id))
		}
	}

	return nil
}

func (b *BazelBuilder) teardownJob(job *batchv1.Job) error {
	if err := b.client.BatchV1().Jobs(job.Namespace).Delete(job.Name, &metav1.DeleteOptions{}); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	pods, err := b.client.CoreV1().Pods(job.Namespace).List(metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(job.Spec.Selector)})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	for _, v := range pods.Items {
		if err := b.client.CoreV1().Pods(v.Namespace).Delete(v.Name, &metav1.DeleteOptions{}); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func (b *BazelBuilder) getTask(taskId string) (*database.Task, error) {
	id, err := strconv.Atoi(taskId)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	task, err := b.dao.Task.Select(context.Background(), int32(id))
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	return task, nil
}

func (b *BazelBuilder) buildJob(job *database.Job, task *database.Task) error {
	if job.Synchronized {
		if b.isRunningJob(job) {
			return xerrors.Errorf(": %w", ErrOtherTaskIsRunning)
		}
	}

	buildTemplate := b.buildJobTemplate(job, task)
	_, err := b.client.BatchV1().Jobs(b.Namespace).Create(buildTemplate)
	if err != nil {
		return xerrors.Errorf(": %v", err)
	}
	now := time.Now()
	task.StartAt = &now

	return nil
}

func (b *BazelBuilder) isRunningJob(job *database.Job) bool {
	jobs, err := b.jobLister.List(labels.Everything())
	if err != nil {
		logger.Log.Warn("Could not get a job's list from kube-apiserver.", zap.Error(err))
		// Can not detect a status of job.
		// In this situation, we assume that the job is running.
		return true
	}

	for _, v := range jobs {
		jobIdString, ok := v.Labels[labelKeyJobId]
		if !ok {
			continue
		}
		jobId, err := strconv.Atoi(jobIdString)
		if err != nil {
			continue
		}
		if job.Id == int32(jobId) {
			if v.Status.CompletionTime.IsZero() && !v.Status.StartTime.IsZero() {
				return true
			}
		}
	}

	return false
}

func (b *BazelBuilder) postProcess(job *batchv1.Job, task *database.Task, success bool) error {
	j, err := b.dao.Job.Select(context.Background(), task.JobId)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	pods, err := b.client.CoreV1().Pods(b.Namespace).List(metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(job.Spec.Selector)})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if len(pods.Items) != 1 {
		return xerrors.New("Target pods not found or found more than 1")
	}

	buf := new(bytes.Buffer)
	logReq := b.client.CoreV1().Pods(b.Namespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{Container: "pre-process"})
	rawLog, err := logReq.DoRaw()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	buf.WriteString("----- pre-process -----\n")
	buf.Write(rawLog)

	logReq = b.client.CoreV1().Pods(b.Namespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{})
	rawLog, err = logReq.DoRaw()
	buf.WriteString("\n")
	buf.WriteString("----- main -----\n")
	buf.Write(rawLog)

	if err := b.minio.Put(context.Background(), job.Name, buf); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	task.LogFile = job.Name

	if j.GithubStatus {
		state := "success"
		if !success {
			state = "failure"
		}
		if err := b.updateGithubStatus(context.Background(), j, task, state); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func (b *BazelBuilder) updateGithubStatus(ctx context.Context, job *database.Job, task *database.Task, state string) error {
	if task.Revision == "" {
		return nil
	}

	u, err := url.Parse(job.Repository.Url)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if u.Hostname() != "github.com" {
		logger.Log.Warn("Expect update a status of github. but repository url is not github.com", zap.String("url", job.Repository.Url))
		return nil
	}
	// u.Path is /owner/repo if URL is github.com.
	s := strings.Split(u.Path, "/")
	owner, repo := s[1], s[2]

	targetUrl := ""
	if state == "success" || state == "failure" {
		targetUrl = fmt.Sprintf("%s/logs/%s", b.dashboardUrl, task.LogFile)
	}

	_, _, err = b.githubClient.Repositories.CreateStatus(
		ctx,
		owner,
		repo,
		task.Revision,
		&github.RepoStatus{
			State:     github.String(state),
			Context:   github.String(fmt.Sprintf("%s %s", job.Command, job.Target)),
			TargetURL: github.String(targetUrl),
		},
	)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (b *BazelBuilder) buildJobTemplate(job *database.Job, task *database.Task) *batchv1.Job {
	mainImage := fmt.Sprintf("%s:%s", b.bazelImage, b.defaultBazelVersion)
	taskIdString := strconv.Itoa(int(task.Id))
	jobIdString := strconv.Itoa(int(job.Id))

	volumes := []corev1.Volume{
		{
			Name: "workdir",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{Name: "workdir", MountPath: "/work"},
	}

	preProcessArgs := []string{"--action=clone", "--work-dir=work", fmt.Sprintf("--url=%s", job.Repository.CloneUrl)}
	if task.Revision != "" {
		preProcessArgs = append(preProcessArgs, "--commit="+task.Revision)
	}

	cpuLimit := b.defaultTaskCPULimit
	if job.CpuLimit != "" {
		q, err := resource.ParseQuantity(job.CpuLimit)
		if err != nil {
			return nil
		}
		cpuLimit = q
	}
	memoryLimit := b.defaultTaskMemoryLimit
	if job.MemoryLimit != "" {
		q, err := resource.ParseQuantity(job.MemoryLimit)
		if err != nil {
			return nil
		}
		memoryLimit = q
	}

	args := []string{task.Command}
	if b.remoteCache != "" {
		args = append(args, fmt.Sprintf("--remote_cache=%s", b.remoteCache))
		if b.remoteAssetApi {
			args = append(args, fmt.Sprintf("--experimental_remote_downloader=%s", b.remoteCache))
		}
	}
	args = append(args, task.Target)
	var backoffLimit int32 = 0
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d", job.Repository.Name, task.Id),
			Namespace: b.Namespace,
			Labels: map[string]string{
				labelKeyJobId:     jobIdString,
				labelKeyTaskId:    taskIdString,
				labelKeyCtrlBy:    "bazel-build",
				watcher.TypeLabel: jobType,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						labelKeyTaskId: taskIdString,
						labelKeyCtrlBy: "bazel-build",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						{
							Name:  "pre-process",
							Image: b.sidecarImage,
							Args:  preProcessArgs,
							VolumeMounts: []corev1.VolumeMount{
								{Name: "workdir", MountPath: "/work"},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "main",
							Image:           mainImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            args,
							WorkingDir:      "/work",
							VolumeMounts:    volumeMounts,
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    cpuLimit,
									corev1.ResourceMemory: memoryLimit,
								},
							},
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
}
