package discovery

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"github.com/google/go-github/v32/github"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	batchv1informers "k8s.io/client-go/informers/batch/v1"
	"k8s.io/client-go/kubernetes"

	"go.f110.dev/mono/go/pkg/build/coordinator"
	"go.f110.dev/mono/go/pkg/build/database"
	"go.f110.dev/mono/go/pkg/build/database/dao"
	"go.f110.dev/mono/go/pkg/build/job"
	"go.f110.dev/mono/go/pkg/build/watcher"
	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/storage"
)

const (
	labelKeyRepoName     = "build.f110.dev/repo-name"
	labelKeyRepositoryId = "build.f110.dev/repository-id"
	labelKeyRevision     = "build.f110.dev/revision"
	labelKeyBazelVersion = "build.f110.dev/bazel-version"
	jobType              = "bazelDiscovery"
	defaultBazelVersion  = "3.5.0"
)

const discoveryJobScript = `{{ .Bazel }} cquery 'kind(job, //...)' --output jsonproto > /tmp/out.log 2> /tmp/err.log
status=$?
if [ $status -eq "0" ]; then
	cat /tmp/out.log
else
	cat /tmp/err.log
fi
exit $status`

type discoveryJobVar struct {
	Bazel string
}

type BuildFile struct {
	Path    string
	Package string

	symbol map[string]funcSymbol
	calls  []funcCall
}

type funcSymbol struct {
	module string
	from   string
	to     string
}

type funcCall struct {
	name string
	args map[string]string
}

type BazelProto struct {
	Results []*BazelResult `json:"results"`
}

type BazelResult struct {
	Target        BazelTarget        `json:"target"`
	Configuration BazelConfiguration `json:"configuration"`
}

type BazelConfiguration struct {
	Checksum string `json:"checksum"`
}

type BazelTarget struct {
	Type string    `json:"type"`
	Rule BazelRule `json:"rule"`
}

type BazelRule struct {
	Name      string      `json:"name"`
	RuleClass string      `json:"ruleClass"`
	Location  string      `json:"location"`
	Attribute []BazelAttr `json:"attribute"`
}

func (b BazelRule) Attrs() map[string]interface{} {
	keyAndValue := make(map[string]interface{})
	for _, a := range b.Attribute {
		switch a.Type {
		case "STRING", "LABEL":
			keyAndValue[a.Name] = a.StringValue
		case "BOOLEAN":
			keyAndValue[a.Name] = a.BoolValue
		case "STRING_LIST":
			keyAndValue[a.Name] = a.StringListValue
		}
	}
	return keyAndValue
}

type BazelAttr struct {
	Name                string   `json:"name"`
	Type                string   `json:"type"`
	IntValue            int      `json:"intValue"`
	StringValue         string   `json:"stringValue"`
	BoolValue           bool     `json:"booleanValue"`
	StringListValue     []string `json:"stringListValue"`
	ExplicitlySpecified bool     `json:"explicitlySpecified"`
	NoDep               bool     `json:"nodep"`
}

type Discover struct {
	Namespace string

	githubClient *github.Client
	client       kubernetes.Interface
	jobInformer  batchv1informers.JobInformer
	minio        *storage.MinIO

	builder              *coordinator.BazelBuilder
	bazelImage           string
	sidecarImage         string
	ctlImage             string
	builderApi           string
	dao                  dao.Options
	githubAppId          int64
	githubInstallationId int64
	githubAppSecretName  string

	debug bool
}

func NewDiscover(
	jobInformer batchv1informers.JobInformer,
	client kubernetes.Interface,
	namespace string,
	daoOpt dao.Options,
	builder *coordinator.BazelBuilder,
	bazelImage string,
	sidecarImage string,
	ctlImage string,
	builderApi string,
	ghClient *github.Client,
	bucket string,
	minioOpt storage.MinIOOptions,
	appId int64,
	installationId int64,
	appSecretName string,
	debug bool,
) *Discover {
	d := &Discover{
		Namespace:            namespace,
		jobInformer:          jobInformer,
		client:               client,
		dao:                  daoOpt,
		builder:              builder,
		bazelImage:           bazelImage,
		sidecarImage:         sidecarImage,
		minio:                storage.NewMinIOStorage(bucket, minioOpt),
		ctlImage:             ctlImage,
		builderApi:           builderApi,
		githubClient:         ghClient,
		githubAppId:          appId,
		githubInstallationId: installationId,
		githubAppSecretName:  appSecretName,
		debug:                debug,
	}
	watcher.Router.Add(jobType, d.syncJob)
	if d.bazelImage == "" {
		d.bazelImage = "l.gcr.io/google/bazel"
	}

	return d
}

// FindOut will create the Job for discover. If revision is not empty, trigger tasks after discovery job finished.
func (d *Discover) FindOut(repository *database.SourceRepository, revision string) error {
	u, err := url.Parse(repository.Url)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	s := strings.Split(u.Path, "/")
	owner, repo := s[1], s[2]
	treeSHA := revision
	if revision == "" {
		repoInfo, _, err := d.githubClient.Repositories.Get(context.TODO(), owner, repo)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		ref, _, err := d.githubClient.Git.GetRef(context.TODO(), owner, repo, fmt.Sprintf("heads/%s", repoInfo.GetDefaultBranch()))
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		treeSHA = ref.Object.GetSHA()
	}
	tree, _, err := d.githubClient.Git.GetTree(context.TODO(), owner, repo, treeSHA, false)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	bazelVersion := defaultBazelVersion
	for _, v := range tree.Entries {
		if v.GetPath() == ".bazelversion" {
			blob, _, err := d.githubClient.Git.GetBlob(context.TODO(), owner, repo, v.GetSHA())
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			buf, err := base64.StdEncoding.DecodeString(blob.GetContent())
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			bazelVersion = strings.TrimRight(string(buf), "\n")
			break
		}
	}

	if err := d.buildJob(context.TODO(), repository, revision, bazelVersion); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	logger.Log.Info("Start discovery job", zap.String("repo", repository.Name), zap.String("revision", revision), zap.String("bazel_version", bazelVersion))

	return nil
}

func (d *Discover) syncJob(job *batchv1.Job) error {
	if !job.DeletionTimestamp.IsZero() {
		logger.Log.Debug("Job has been deleted", zap.String("job.name", job.Name))
		return nil
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	var repoId int32
	if v, ok := job.Labels[labelKeyRepositoryId]; !ok {
		return nil
	} else {
		i, err := strconv.Atoi(v)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		repoId = int32(i)
	}

	success := false
	for _, v := range job.Status.Conditions {
		switch v.Type {
		case batchv1.JobComplete:
			success = true
			if err := d.minio.Delete(ctx, job.Name); err != nil {
				return xerrors.Errorf(": %w", err)
			}
		case batchv1.JobFailed:
			pods, err := d.client.CoreV1().Pods(job.Namespace).List(
				ctx,
				metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(job.Spec.Selector)},
			)
			if err == nil && len(pods.Items) == 1 {
				pod := pods.Items[0]
				logReq := d.client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: "main"})
				rawLog, err := logReq.DoRaw(ctx)
				if err != nil {
					return xerrors.Errorf(": %w", err)
				}
				if err := d.minio.Put(ctx, job.Name, rawLog); err != nil {
					return xerrors.Errorf(": %w", err)
				}
			}

			if d.debug {
				logger.Log.Info("Skip delete job due to enabled debugging mode", zap.String("job.name", job.Name))
				return nil
			}

			if err := d.teardownJob(ctx, job); err != nil {
				return xerrors.Errorf(": %w", err)
			}
			jobs, err := d.dao.Job.ListBySourceRepositoryId(context.Background(), repoId)
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			for _, v := range jobs {
				v.Sync = false
				if err := d.dao.Job.Update(context.Background(), v); err != nil {
					logger.Log.Warn("Failed update job", zap.Error(err))
				}
			}
			return nil
		}
	}
	if !success {
		return nil
	}

	pods, err := d.client.CoreV1().Pods(job.Namespace).List(ctx, metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(job.Spec.Selector)})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if len(pods.Items) != 1 {
		return xerrors.New("Target pod not found or found more than 1")
	}
	logReq := d.client.CoreV1().Pods(pods.Items[0].Namespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{})
	res := logReq.Do(ctx)
	rawLog, err := res.Raw()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	jobs, err := Discovery(rawLog)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	currentJobs, err := d.dao.Job.ListBySourceRepositoryId(context.Background(), repoId)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	jobMap := make(map[string]*database.Job)
	for _, v := range currentJobs {
		// Temporary, all jobs will be deactivated.
		v.Active = false

		jobMap[v.Name] = v
	}

	bazelVersion := ""
	if v, ok := job.Labels[labelKeyBazelVersion]; ok {
		bazelVersion = v
	}

	repo, err := d.dao.Repository.Select(ctx, repoId)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	newJobs := make([]*database.Job, 0)
	for _, j := range jobs {
		if v, ok := jobMap[j.Name]; ok {
			v.ImportFrom(j)
			v.BazelVersion = bazelVersion
		} else {
			n := &database.Job{}
			n.ImportFrom(j)
			n.BazelVersion = bazelVersion
			n.RepositoryId = repoId
			n.Active = true
			n.Repository = repo
			newJobs = append(newJobs, n)
		}
	}

	for _, v := range currentJobs {
		if err := d.dao.Job.Update(context.Background(), v); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}
	for _, v := range newJobs {
		if created, err := d.dao.Job.Create(context.Background(), v); err != nil {
			return xerrors.Errorf(": %w", err)
		} else {
			v.Id = created.Id
		}
	}

	if err := d.syncCronJob(ctx, append(currentJobs, newJobs...)); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	if err := d.teardownJob(ctx, job); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	if rev, ok := job.ObjectMeta.Labels[labelKeyRevision]; ok && rev != "" {
		// Trigger after task.
		if err := d.triggerTask(ctx, job, rev); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func (d *Discover) triggerTask(ctx context.Context, job *batchv1.Job, revision string) error {
	repoId, ok := job.ObjectMeta.Labels[labelKeyRepositoryId]
	if !ok {
		return nil
	}
	id, err := strconv.Atoi(repoId)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	repo, err := d.dao.Repository.Select(ctx, int32(id))
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	jobs, err := d.dao.Job.ListBySourceRepositoryId(ctx, repo.Id)
	if err != nil {
		logger.Log.Warn("Could not get jobs", zap.Error(err))
		return xerrors.Errorf(": %w", err)
	}
	for _, v := range jobs {
		// Trigger the job when Command is build or test only.
		// In other words, If command is run, we are not trigger the job via PushEvent.
		switch v.Command {
		case "build", "test":
		default:
			continue
		}

		if _, err := d.builder.Build(ctx, v, revision, v.Command, v.Targets, v.Platforms, "push"); err != nil {
			logger.Log.Warn("Failed start job", zap.Error(err), zap.Int32("job.id", v.Id))
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func (d *Discover) teardownJob(ctx context.Context, job *batchv1.Job) error {
	err := d.client.BatchV1().Jobs(job.Namespace).Delete(ctx, job.Name, metav1.DeleteOptions{})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	pods, err := d.client.CoreV1().Pods(job.Namespace).List(ctx, metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(job.Spec.Selector)})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	for _, v := range pods.Items {
		if err := d.client.CoreV1().Pods(v.Namespace).Delete(ctx, v.Name, metav1.DeleteOptions{}); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func (d *Discover) syncCronJob(ctx context.Context, jobs []*database.Job) error {
	cronJobs, err := d.client.BatchV1beta1().CronJobs(d.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	cronJobMap := make(map[string]*batchv1beta1.CronJob)
	deleteCronJobs := make([]*batchv1beta1.CronJob, 0)
	for _, v := range cronJobs.Items {
		cronJobMap[v.Name] = &v

		found := false
		for _, j := range jobs {
			if fmt.Sprintf("%s-%d", j.Repository.Name, j.Id) == v.Name {
				found = true
				break
			}
		}
		if found {
			deleteCronJobs = append(deleteCronJobs, &v)
		}
	}

	newCronJobs := make([]*batchv1beta1.CronJob, 0)
	updateCronJobs := make([]*batchv1beta1.CronJob, 0)
	for _, v := range jobs {
		if v.Schedule == "" {
			if cj, ok := cronJobMap[fmt.Sprintf("%s-%d", v.Repository.Name, v.Id)]; ok {
				deleteCronJobs = append(deleteCronJobs, cj)
			}
			continue
		}

		if cj, ok := cronJobMap[fmt.Sprintf("%s-%d", v.Repository.Name, v.Id)]; !ok {
			backoffLimit := int32(1)
			jobHistory := int32(1)
			newCronJobs = append(newCronJobs, &batchv1beta1.CronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%d", v.Repository.Name, v.Id),
					Namespace: d.Namespace,
					Labels: map[string]string{
						labelKeyRepoName:     v.Repository.Name,
						labelKeyRepositoryId: strconv.Itoa(int(v.RepositoryId)),
						labelKeyBazelVersion: v.BazelVersion,
					},
				},
				Spec: batchv1beta1.CronJobSpec{
					Schedule:                   v.Schedule,
					SuccessfulJobsHistoryLimit: &jobHistory,
					FailedJobsHistoryLimit:     &jobHistory,
					JobTemplate: batchv1beta1.JobTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								labelKeyRepoName:     v.Repository.Name,
								labelKeyRepositoryId: strconv.Itoa(int(v.RepositoryId)),
								labelKeyBazelVersion: v.BazelVersion,
							},
						},
						Spec: batchv1.JobSpec{
							BackoffLimit: &backoffLimit,
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									RestartPolicy: corev1.RestartPolicyNever,
									Containers: []corev1.Container{
										{
											Name:            "trigger",
											Image:           d.ctlImage,
											ImagePullPolicy: corev1.PullAlways,
											Args: []string{
												"job",
												"trigger",
												fmt.Sprintf("--job-id=%d", v.Id),
												fmt.Sprintf("--endpoint=%s", d.builderApi),
												"--via=cron",
											},
										},
									},
								},
							},
						},
					},
				},
			})
		} else {
			if cj.Spec.Schedule != v.Schedule {
				newCJ := cj.DeepCopy()
				newCJ.Spec.Schedule = v.Schedule
				updateCronJobs = append(updateCronJobs, newCJ)
			}
		}
	}

	for _, v := range deleteCronJobs {
		logger.Log.Debug("Delete CronJob", zap.String("name", v.Name))
		err := d.client.BatchV1beta1().CronJobs(d.Namespace).Delete(ctx, v.Name, metav1.DeleteOptions{})
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}
	for _, v := range newCronJobs {
		logger.Log.Debug("Create CronJob", zap.String("name", v.Name))
		_, err := d.client.BatchV1beta1().CronJobs(d.Namespace).Create(ctx, v, metav1.CreateOptions{})
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}
	for _, v := range updateCronJobs {
		logger.Log.Debug("Update CronJob", zap.String("name", v.Name))
		_, err := d.client.BatchV1beta1().CronJobs(d.Namespace).Update(ctx, v, metav1.UpdateOptions{})
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func (d *Discover) buildJob(ctx context.Context, repository *database.SourceRepository, revision, bazelVersion string) error {
	j := d.newDiscoveryJob(repository, revision, bazelVersion)
	_, err := d.client.BatchV1().Jobs(d.Namespace).Create(ctx, j, metav1.CreateOptions{})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (d *Discover) newDiscoveryJob(repository *database.SourceRepository, revision, bazelVersion string) *batchv1.Job {
	volumes := []corev1.Volume{
		{
			Name: "workdir",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	sidecarVolumeMounts := []corev1.VolumeMount{
		{Name: "workdir", MountPath: "/work"},
	}
	if repository.Private {
		volumes = append(volumes, corev1.Volume{
			Name: "github-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: d.githubAppSecretName,
				},
			},
		})
		sidecarVolumeMounts = append(sidecarVolumeMounts, corev1.VolumeMount{
			Name: "github-secret", MountPath: "/etc/github", ReadOnly: true,
		})
	}
	preProcessArgs := []string{"--action=clone", "--work-dir=/work", fmt.Sprintf("--url=%s", repository.CloneUrl)}
	if repository.Private {
		preProcessArgs = append(preProcessArgs,
			fmt.Sprintf("--github-app-id=%d", d.githubAppId),
			fmt.Sprintf("--github-installation-id=%d", d.githubInstallationId),
			"--private-key-file=/etc/github/privatekey.pem",
		)
	}

	discoveryScriptTemplate := template.Must(template.New("").Parse(discoveryJobScript))
	discoveryScript := new(bytes.Buffer)
	err := discoveryScriptTemplate.Execute(discoveryScript, discoveryJobVar{
		Bazel: "bazel",
	})
	if err != nil {
		return nil
	}

	var backoffLimit int32 = 0
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-discovery", repository.Name),
			Namespace: d.Namespace,
			Labels: map[string]string{
				labelKeyRepoName:     repository.Name,
				labelKeyRepositoryId: strconv.Itoa(int(repository.Id)),
				labelKeyRevision:     revision,
				labelKeyBazelVersion: bazelVersion,
				watcher.TypeLabel:    jobType,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						labelKeyRepoName: repository.Name,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						{
							Name:         "pre-process",
							Image:        d.sidecarImage,
							Args:         preProcessArgs,
							VolumeMounts: sidecarVolumeMounts,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "main",
							Image:           fmt.Sprintf("%s:%s", d.bazelImage, bazelVersion),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"sh", "-c", discoveryScript.String()},
							WorkingDir:      "/work",
							VolumeMounts: []corev1.VolumeMount{
								{Name: "workdir", MountPath: "/work"},
							},
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
}

func Discovery(b []byte) ([]*job.Job, error) {
	res := &BazelProto{}
	if err := json.Unmarshal(b, res); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	seen := make(map[string]struct{})
	jobs := make([]*job.Job, 0)
	for _, v := range res.Results {
		j := &job.Job{}
		keyAndValue := v.Target.Rule.Attrs()

		refTypeOfJob := reflect.TypeOf(j).Elem()
		refValueOfJob := reflect.ValueOf(j).Elem()
		for i := 0; i < refTypeOfJob.NumField(); i++ {
			f := refTypeOfJob.Field(i)
			attrName := f.Tag.Get("attr")

			value := refValueOfJob.Field(i)
			switch f.Type.Kind() {
			case reflect.String:
				v := keyAndValue[attrName]
				if v != nil {
					value.SetString(v.(string))
				}
			case reflect.Bool:
				v := keyAndValue[attrName]
				if v != nil {
					value.SetBool(v.(bool))
				}
			case reflect.Slice:
				v := keyAndValue[attrName]
				if v != nil {
					val := reflect.ValueOf(v)
					value.Set(val)
				}
			}
		}

		if j.Target != "" {
			s := strings.SplitN(j.Target, ":", 2)
			j.Package, j.Target = s[0], s[1]
		}
		if _, ok := seen[j.Name]; ok {
			continue
		}
		jobs = append(jobs, j)

		// Mark
		seen[j.Name] = struct{}{}
	}

	return jobs, nil
}
