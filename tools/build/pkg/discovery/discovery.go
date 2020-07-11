package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/xerrors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	batchv1informers "k8s.io/client-go/informers/batch/v1"
	"k8s.io/client-go/kubernetes"

	"go.f110.dev/mono/lib/logger"
	"go.f110.dev/mono/tools/build/pkg/coordinator"
	"go.f110.dev/mono/tools/build/pkg/database"
	"go.f110.dev/mono/tools/build/pkg/database/dao"
	"go.f110.dev/mono/tools/build/pkg/job"
	"go.f110.dev/mono/tools/build/pkg/watcher"
)

const (
	labelKeyRepoName     = "build.f110.dev/repo-name"
	labelKeyRepositoryId = "build.f110.dev/repository-id"
	discoveryBazelQuery  = "kind(job, //...)"
	jobType              = "bazelDiscovery"
)

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
	Target BazelTarget `json:"target"`
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
		}
	}
	return keyAndValue
}

type BazelAttr struct {
	Name                string `json:"name"`
	Type                string `json:"type"`
	IntValue            int    `json:"intValue"`
	StringValue         string `json:"stringValue"`
	BoolValue           bool   `json:"booleanValue"`
	ExplicitlySpecified bool   `json:"explicitlySpecified"`
	NoDep               bool   `json:"nodep"`
}

type Discover struct {
	Namespace string

	client      kubernetes.Interface
	jobInformer batchv1informers.JobInformer

	job *dao.Job
}

func NewDiscover(jobInformer batchv1informers.JobInformer, client kubernetes.Interface, namespace string, job *dao.Job) *Discover {
	d := &Discover{
		Namespace:   namespace,
		jobInformer: jobInformer,
		client:      client,
		job:         job,
	}
	watcher.Router.Add(jobType, d.syncJob)

	return d
}

func (d *Discover) FindOut(repository *database.SourceRepository) error {
	if err := d.buildJob(repository); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (d *Discover) syncJob(job *batchv1.Job) error {
	if !job.DeletionTimestamp.IsZero() {
		logger.Log.Debug("Job has been deleted", zap.String("job.name", job.Name))
		return nil
	}

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
		}
	}

	if success {
		pods, err := d.client.CoreV1().Pods(job.Namespace).List(metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(job.Spec.Selector)})
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		if len(pods.Items) != 1 {
			return xerrors.New("Target pod not found or found more than 1")
		}
		logReq := d.client.CoreV1().Pods(pods.Items[0].Namespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{})
		res := logReq.Do()
		rawLog, err := res.Raw()
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		jobs, err := Discovery(rawLog)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		currentJobs, err := d.job.ListBySourceRepositoryId(context.Background(), repoId)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		jobMap := make(map[string]*database.Job)
		for _, v := range currentJobs {
			// Temporary, all jobs will be deactivated.
			v.Active = false

			jobMap[v.Command+"/"+v.Target] = v
		}

		newJobs := make([]*database.Job, 0)
		for _, j := range jobs {
			if v, ok := jobMap[j.Command+"/"+j.Target]; ok {
				v.Active = true
			} else {
				newJobs = append(newJobs, &database.Job{
					Command:      j.Command,
					Target:       j.Target,
					RepositoryId: repoId,
					Active:       true,
				})
			}
		}

		for _, v := range currentJobs {
			if err := d.job.Update(context.Background(), v); err != nil {
				return xerrors.Errorf(": %w", err)
			}
		}
		for _, v := range newJobs {
			if _, err := d.job.Create(context.Background(), v); err != nil {
				return xerrors.Errorf(": %w", err)
			}
		}

		if err := d.teardownJob(job); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func (d *Discover) teardownJob(job *batchv1.Job) error {
	err := d.client.BatchV1().Jobs(job.Namespace).Delete(job.Name, &metav1.DeleteOptions{})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	pods, err := d.client.CoreV1().Pods(job.Namespace).List(metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(job.Spec.Selector)})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	for _, v := range pods.Items {
		if err := d.client.CoreV1().Pods(v.Namespace).Delete(v.Name, &metav1.DeleteOptions{}); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func (d *Discover) buildJob(repository *database.SourceRepository) error {
	j := d.jobTemplate(repository)
	_, err := d.client.BatchV1().Jobs(d.Namespace).Create(j)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (d Discover) jobTemplate(repository *database.SourceRepository) *batchv1.Job {
	var backoffLimit int32 = 0
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-discovery", repository.Name),
			Namespace: d.Namespace,
			Labels: map[string]string{
				labelKeyRepoName:     repository.Name,
				labelKeyRepositoryId: strconv.Itoa(int(repository.Id)),
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
							Name:  "pre-process",
							Image: coordinator.SidecarImage,
							Args:  []string{"--action=clone", "--work-dir=/work", fmt.Sprintf("--url=%s", repository.CloneUrl)},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "workdir", MountPath: "/work"},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "main",
							Image:           "l.gcr.io/google/bazel:3.2.0",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"sh", "-c", fmt.Sprintf("bazel cquery '%s' --output jsonproto 2> /dev/null", discoveryBazelQuery)},
							WorkingDir:      "/work",
							VolumeMounts: []corev1.VolumeMount{
								{Name: "workdir", MountPath: "/work"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "workdir",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
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
			}
		}

		if j.Target != "" {
			s := strings.SplitN(j.Target, ":", 2)
			j.Package, j.Target = s[0], s[1]
		}
		if j.Targets != "" && j.Target == "" {
			j.Target = j.Targets
		}
		jobs = append(jobs, j)
	}

	return jobs, nil
}
