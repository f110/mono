package coordinator

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v32/github"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	SidecarImage        = "registry.f110.dev/build/sidecar"
	bazelImage          = "l.gcr.io/google/bazel"
	defaultBazelVersion = "3.2.0"

	labelKeyTaskId = "build.f110.dev/task-id"
	labelKeyCtrlBy = "build.f110.dev/control-by"

	jobTimeout = 1 * time.Hour
	jobType    = "bazelBuilder"
)

type GithubAppOptions struct {
	AppId          int64
	InstallationId int64
	PrivateKeyFile string
}

func NewGithubAppOptions(appId, installationId int64, privateKeyFile string) GithubAppOptions {
	return GithubAppOptions{AppId: appId, InstallationId: installationId, PrivateKeyFile: privateKeyFile}
}

type BazelBuilder struct {
	Namespace    string
	dashboardUrl string

	client    kubernetes.Interface
	jobLister batchv1listers.JobLister
	config    *rest.Config

	dao          dao.Options
	githubClient *github.Client
	minio        *storage.MinIO
	workingDir   string
	dev          bool
}

func NewBazelBuilder(
	dashboardUrl string,
	jobInformer batchv1informers.JobInformer,
	client kubernetes.Interface,
	config *rest.Config,
	daoOpt dao.Options,
	namespace string,
	appOpt GithubAppOptions,
	minIOOpt storage.MinIOOptions,
	dev bool,
) (*BazelBuilder, error) {
	t, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appOpt.AppId, appOpt.InstallationId, appOpt.PrivateKeyFile)
	if err != nil {
		return nil, xerrors.Errorf(": %v", err)
	}

	b := &BazelBuilder{
		Namespace:    namespace,
		dashboardUrl: dashboardUrl,
		config:       config,
		client:       client,
		jobLister:    jobInformer.Lister(),
		dao:          daoOpt,
		githubClient: github.NewClient(&http.Client{Transport: t}),
		minio:        storage.NewMinIOStorage(client, config, minIOOpt, dev),
		dev:          dev,
	}
	watcher.Router.Add(jobType, b.syncJob)

	return b, nil
}

func (b *BazelBuilder) Build(ctx context.Context, job *database.Job, revision, via string) (*database.Task, error) {
	task, err := b.dao.Task.Create(ctx, &database.Task{JobId: job.Id, Revision: revision, Via: via})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	if err := b.buildJob(job, task); err != nil {
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

	task, err := b.dao.Task.SelectById(context.Background(), int32(id))
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	return task, nil
}

func (b *BazelBuilder) buildJob(job *database.Job, task *database.Task) error {
	buildTemplate := b.buildJobTemplate(job, task)
	_, err := b.client.BatchV1().Jobs(b.Namespace).Create(buildTemplate)
	if err != nil {
		return xerrors.Errorf(": %v", err)
	}

	return nil
}

func (b *BazelBuilder) postProcess(job *batchv1.Job, task *database.Task, success bool) error {
	j, err := b.dao.Job.SelectById(context.Background(), task.JobId)
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
	logReq := b.client.CoreV1().Pods(b.Namespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{})
	res := logReq.Do()
	rawLog, err := res.Raw()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	if err := b.minio.Put(context.Background(), job.Name, rawLog); err != nil {
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
	mainImage := fmt.Sprintf("%s:%s", bazelImage, defaultBazelVersion)
	taskIdString := strconv.Itoa(int(task.Id))

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

	var backoffLimit int32 = 0
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d", job.Repository.Name, task.Id),
			Namespace: b.Namespace,
			Labels: map[string]string{
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
							Image: SidecarImage,
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
							Args:            []string{job.Command, job.Target},
							WorkingDir:      "/work",
							VolumeMounts:    volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
}
