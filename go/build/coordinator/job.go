package coordinator

import (
	"fmt"
	"strconv"
	"strings"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/watcher"
	"go.f110.dev/mono/go/k8s/k8sfactory"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/varptr"
)

type JobBuilder struct {
	repo                 *database.SourceRepository
	task                 *database.Task
	job                  *config.Job
	platform             string
	bazelImage           string
	sidecarImage         string
	defaultBazelVersion  string
	bazelMirrorURL       string
	namespace            string
	useBazelisk          bool
	pullAlways           bool
	githubAppId          int64
	githubInstallationId int64
	githubAppSecretName  string
	defaultCPULimit      resource.Quantity
	defaultMemoryLimit   resource.Quantity
	remoteCache          string
	remoteAssetAPI       bool
	vaultAddr            string
	excludeNodes         []string

	PreProcessContainerName string
	BuildContainerName      string
	ReportContainerName     string

	reportContainer *corev1.Container
	workDirVolume   *k8sfactory.VolumeSource
	// commDirVolume is a empty dir volume for sharing Build Event protocol file between main and report container.
	commDirVolume *k8sfactory.VolumeSource

	sa                       *corev1.ServiceAccount
	mainContainer            *corev1.Container
	preProcessContainer      *corev1.Container
	credentialSetupContainer *corev1.Container
	buildPod                 *corev1.Pod
	secretProviderClasses    []runtime.Object
}

func NewJobBuilder(ns, bazelImage, sidecar string, excludeNodes []string) *JobBuilder {
	workDir := k8sfactory.NewEmptyDirVolumeSource("workdir", "/work")
	return &JobBuilder{
		namespace:               ns,
		bazelImage:              bazelImage,
		sidecarImage:            sidecar,
		PreProcessContainerName: "pre-process",
		BuildContainerName:      "main",
		ReportContainerName:     "report",
		excludeNodes:            excludeNodes,

		workDirVolume: workDir,
		mainContainer: k8sfactory.ContainerFactory(nil,
			k8sfactory.Name("main"),
			k8sfactory.PullPolicy(corev1.PullIfNotPresent),
			k8sfactory.WorkDir(workDir.Mount.MountPath),
			k8sfactory.Volume(workDir),
			k8sfactory.EnvVar("WORKSPACE", workDir.Mount.MountPath),
		),
		preProcessContainer: k8sfactory.ContainerFactory(nil,
			k8sfactory.Name("pre-process"),
			k8sfactory.Image(sidecar, nil),
			k8sfactory.PullPolicy(corev1.PullIfNotPresent),
			k8sfactory.Volume(workDir),
		),
		buildPod: k8sfactory.PodFactory(nil,
			k8sfactory.Labels(map[string]string{
				labelKeyCtrlBy: "bazel-build",
			}),
			k8sfactory.RestartPolicy(corev1.RestartPolicyNever),
			k8sfactory.Volume(workDir),
			k8sfactory.AntiNodeAffinity("kubernetes.io/hostname", excludeNodes),
		),
	}
}

func (j *JobBuilder) BazelBinaryMirror(u string) {
	j.bazelMirrorURL = u
}

func (j *JobBuilder) DefaultBazelVersion(ver string) {
	j.defaultBazelVersion = ver
}

func (j *JobBuilder) UseBazelisk() {
	j.useBazelisk = true
	if j.bazelMirrorURL != "" {
		j.mainContainer = k8sfactory.ContainerFactory(j.mainContainer, k8sfactory.EnvVar("BAZELISK_FORMAT_URL", j.bazelMirrorURL+"/bazel-%v-%o-%m%e"))
	}
}

func (j *JobBuilder) PullAlways() {
	j.pullAlways = true
	j.mainContainer = k8sfactory.ContainerFactory(j.mainContainer, k8sfactory.PullPolicy(corev1.PullAlways))
	j.preProcessContainer = k8sfactory.ContainerFactory(j.preProcessContainer, k8sfactory.PullPolicy(corev1.PullAlways))
}

func (j *JobBuilder) GitHubApp(appId, installationId int64, secretName string) {
	j.githubAppId = appId
	j.githubInstallationId = installationId
	j.githubAppSecretName = secretName
}

func (j *JobBuilder) DefaultLimit(cpu, memory resource.Quantity) {
	j.defaultCPULimit = cpu
	j.defaultMemoryLimit = memory
}

func (j *JobBuilder) EnableRemoteCache(addr string) {
	j.remoteCache = addr
}

func (j *JobBuilder) EnableRemoteAssetAPI() {
	j.remoteAssetAPI = true
}

func (j *JobBuilder) Vault(addr string) {
	j.vaultAddr = addr
}

func (j *JobBuilder) Clone() *JobBuilder {
	return &JobBuilder{
		namespace:               j.namespace,
		bazelImage:              j.bazelImage,
		sidecarImage:            j.sidecarImage,
		defaultBazelVersion:     j.defaultBazelVersion,
		bazelMirrorURL:          j.bazelMirrorURL,
		PreProcessContainerName: j.PreProcessContainerName,
		BuildContainerName:      j.BuildContainerName,
		ReportContainerName:     j.ReportContainerName,
		useBazelisk:             j.useBazelisk,
		pullAlways:              j.pullAlways,
		githubAppId:             j.githubAppId,
		githubInstallationId:    j.githubInstallationId,
		githubAppSecretName:     j.githubAppSecretName,
		defaultCPULimit:         j.defaultCPULimit,
		defaultMemoryLimit:      j.defaultMemoryLimit,
		remoteCache:             j.remoteCache,
		remoteAssetAPI:          j.remoteAssetAPI,
		vaultAddr:               j.vaultAddr,

		workDirVolume:       j.workDirVolume,
		mainContainer:       j.mainContainer,
		preProcessContainer: j.preProcessContainer,
		buildPod:            j.buildPod,
	}
}

func (j *JobBuilder) Job(job *config.Job) *JobBuilder {
	if j.job != nil {
		return j
	}
	if job.RepositoryName == "" {
		logger.Log.Warn("RepositoryName is required", zap.String("name", job.Name))
		return j
	}
	j.job = job

	j.sa = k8sfactory.ServiceAccountFactory(nil,
		k8sfactory.Namef("build-%s", j.job.RepositoryName),
		k8sfactory.Namespace(j.namespace),
	)
	j.makeReportContainer()
	j.injectSecret()

	if job.Container != "" {
		j.mainContainer = k8sfactory.ContainerFactory(j.mainContainer, k8sfactory.Image(job.Container, nil))
	}
	cpuLimit := j.defaultCPULimit
	if j.job.CPULimit != "" {
		q, err := resource.ParseQuantity(j.job.CPULimit)
		if err != nil {
			logger.Log.Info("Invalid limit syntax", zap.String("job", j.job.Name), logger.Error(err))
		} else {
			cpuLimit = q
		}
	}
	memoryLimit := j.defaultMemoryLimit
	if j.job.MemoryLimit != "" {
		q, err := resource.ParseQuantity(j.job.MemoryLimit)
		if err != nil {
			logger.Log.Info("Invalid limit syntax", zap.String("job", j.job.Name), logger.Error(err))
		} else {
			memoryLimit = q
		}
	}
	j.mainContainer = k8sfactory.ContainerFactory(j.mainContainer, k8sfactory.ResourceLimit(cpuLimit, memoryLimit))

	for k, v := range j.job.Env {
		switch val := v.(type) {
		case string:
			j.mainContainer = k8sfactory.ContainerFactory(j.mainContainer, k8sfactory.EnvVar(k, val))
		default:
			logger.Log.Warn("Not supported value type", zap.String("key", k))
		}
	}

	j.buildPod = k8sfactory.PodFactory(j.buildPod, k8sfactory.ServiceAccount(fmt.Sprintf("build-%s", j.job.RepositoryName)))
	return j
}

func (j *JobBuilder) Repo(repo *database.SourceRepository) *JobBuilder {
	if j.repo != nil {
		return j
	}
	j.repo = repo

	if j.repo.Private {
		githubSecretVolume := k8sfactory.NewSecretVolumeSource(
			"github-secret",
			"/etc/github",
			k8sfactory.SecretFactory(nil, k8sfactory.Name(j.githubAppSecretName)),
		)
		j.preProcessContainer = k8sfactory.ContainerFactory(j.preProcessContainer, k8sfactory.Volume(githubSecretVolume))
		j.buildPod = k8sfactory.PodFactory(j.buildPod, k8sfactory.Volume(githubSecretVolume))
	}
	return j
}

func (j *JobBuilder) Task(task *database.Task) *JobBuilder {
	if j.task != nil {
		return j
	}
	if j.job == nil {
		logger.Log.Error("Job is not set")
		return j
	}
	j.task = task
	j.makeReportContainer()
	j.injectSecret()

	if j.mainContainer.Image == "" {
		imageTag := j.defaultBazelVersion
		if j.useBazelisk {
			imageTag = "bazelisk"
		} else if j.task.BazelVersion != "" {
			imageTag = j.task.BazelVersion
		}
		j.mainContainer = k8sfactory.ContainerFactory(j.mainContainer, k8sfactory.Image(fmt.Sprintf("%s:%s", j.bazelImage, imageTag), nil))
	}

	j.buildPod = k8sfactory.PodFactory(j.buildPod, k8sfactory.Labels(map[string]string{labelKeyTaskId: strconv.Itoa(int(j.task.Id))}))
	return j
}

func (j *JobBuilder) Platform(p string) *JobBuilder {
	j.platform = p
	j.injectSecret()
	return j
}

func (j *JobBuilder) Build() ([]runtime.Object, error) {
	if j.job == nil {
		return nil, xerrors.Define("job is not set").WithStack()
	}
	if j.task == nil {
		return nil, xerrors.Define("task is not set").WithStack()
	}
	if j.repo == nil {
		return nil, xerrors.Define("repo is not set").WithStack()
	}
	if j.platform == "" {
		return nil, xerrors.Define("platform is not set").WithStack()
	}

	builtObjects := []runtime.Object{j.sa}

	preProcessArgs := []string{"clone", fmt.Sprintf("--work-dir=%s", j.workDirVolume.Mount.MountPath), fmt.Sprintf("--url=%s", j.repo.CloneUrl)}
	if j.task.Revision != "" {
		preProcessArgs = append(preProcessArgs, "--commit="+j.task.Revision)
	}
	if j.repo.Private {
		preProcessArgs = append(preProcessArgs,
			fmt.Sprintf("--github-app-id=%d", j.githubAppId),
			fmt.Sprintf("--github-installation-id=%d", j.githubInstallationId),
			"--private-key-file=/etc/github/privatekey.pem",
		)
	}
	preProcessContainer := k8sfactory.ContainerFactory(j.preProcessContainer, k8sfactory.Args(preProcessArgs...))

	args := []string{j.task.Command}
	if j.remoteCache != "" {
		args = append(args, fmt.Sprintf("--remote_cache=%s", j.remoteCache))
		if j.remoteAssetAPI {
			args = append(args, fmt.Sprintf("--experimental_remote_downloader=%s", j.remoteCache))
		}
	}
	if j.task.ConfigName != "" {
		args = append(args, fmt.Sprintf("--config=%s", j.task.ConfigName))
	}
	var platformName string
	if j.platform != "" {
		args = append(args, "--platforms="+j.platform)
		if strings.Contains(j.platform, ":") {
			s := strings.Split(j.platform, ":")
			platformName = "-" + strings.Replace(s[1], "_", "-", -1)
		}
	}
	if j.commDirVolume != nil {
		args = append(args,
			fmt.Sprintf("--build_event_binary_file=%s/bep", j.commDirVolume.Mount.MountPath),
			"--cache_test_results=no",
		)
	}
	if j.job.Command == "test" && !j.task.IsTrunk {
		args = append(args, "--remote_upload_local_results=false")
	}
	switch j.job.Command {
	case "test":
		args = append(args, "--")
		targets := strings.Split(j.task.Targets, "\n")
		args = append(args, targets...)
	case "run":
		args = append(args, j.job.Targets[0])
		if j.job.Args != nil {
			args = append(args, "--")
			args = append(args, j.job.Args...)
		}
	}
	mainContainer := k8sfactory.ContainerFactory(j.mainContainer, k8sfactory.Args(args...))

	builtObjects = append(builtObjects, j.secretProviderClasses...)
	builtObjects = append(builtObjects, k8sfactory.JobFactory(nil,
		k8sfactory.Namef("%s-%d%s", j.job.RepositoryName, j.task.Id, platformName),
		k8sfactory.Namespace(j.namespace),
		k8sfactory.Labels(map[string]string{
			labelKeyRepoId:    fmt.Sprintf("%d", j.repo.Id),
			labelKeyJobId:     j.job.Identification(),
			labelKeyTaskId:    strconv.Itoa(int(j.task.Id)),
			labelKeyCtrlBy:    "bazel-build",
			watcher.TypeLabel: jobType,
		}),
		k8sfactory.Finalizer(bazelBuilderControllerFinalizerName),
		k8sfactory.BackoffLimit(0),
		k8sfactory.PodFailurePolicy(batchv1.PodFailurePolicyRule{
			Action: batchv1.PodFailurePolicyActionFailJob,
			OnExitCodes: &batchv1.PodFailurePolicyOnExitCodesRequirement{
				ContainerName: varptr.Ptr(j.BuildContainerName),
				Operator:      batchv1.PodFailurePolicyOnExitCodesOpNotIn,
				Values:        []int32{0},
			},
		}),
		k8sfactory.Pod(
			k8sfactory.PodFactory(j.buildPod,
				k8sfactory.InitContainer(preProcessContainer),
				k8sfactory.InitContainer(j.credentialSetupContainer),
				k8sfactory.Container(mainContainer),
				k8sfactory.Container(j.reportContainer),
				k8sfactory.SortVolume(),
			),
		),
	))
	return builtObjects, nil
}

func (j *JobBuilder) makeReportContainer() {
	if j.job == nil || j.job.Command != "test" {
		return
	}
	if j.task == nil || !j.task.IsTrunk {
		return
	}

	j.commDirVolume = k8sfactory.NewEmptyDirVolumeSource("comm", "/comm")
	j.reportContainer = k8sfactory.ContainerFactory(nil,
		k8sfactory.Name(j.ReportContainerName),
		k8sfactory.Image(j.sidecarImage, nil),
		k8sfactory.PullPolicy(corev1.PullIfNotPresent),
		k8sfactory.Volume(j.commDirVolume),
		k8sfactory.Args("report", fmt.Sprintf("--event-binary-file=%s/bep", j.commDirVolume.Mount.MountPath), "--startup-timeout=10m"),
	)
	j.mainContainer = k8sfactory.ContainerFactory(j.mainContainer, k8sfactory.Volume(j.commDirVolume))
	j.buildPod = k8sfactory.PodFactory(j.buildPod, k8sfactory.Volume(j.commDirVolume))
}

func (j *JobBuilder) injectSecret() {
	if j.job == nil || j.task == nil || j.platform == "" {
		return
	}

	if len(j.job.Secrets) > 0 && j.vaultAddr == "" {
		logger.Log.Warn("Secret injection is not supported", zap.String("repo", j.repo.Name), zap.String("job", j.job.Name))
		return
	}

	var platformName string
	if j.platform != "" {
		if strings.Contains(j.platform, ":") {
			s := strings.Split(j.platform, ":")
			platformName = "-" + strings.Replace(s[1], "_", "-", -1)
		}
	}
	secretProviderClasses := make(map[string]*secretsstorev1.SecretProviderClass)
	registrySecretProviderClass := make(map[string]*secretsstorev1.SecretProviderClass)
	for _, s := range j.job.Secrets {
		switch secret := s.(type) {
		case *config.Secret:
			if secret.MountPath != "" {
				name := fmt.Sprintf("%s-%d%s-%s", j.job.RepositoryName, j.task.Id, platformName, strings.Replace(secret.MountPath[1:], "/", "-", -1))
				if c, ok := secretProviderClasses[secret.MountPath]; !ok {
					secretProviderClasses[secret.MountPath] = &secretsstorev1.SecretProviderClass{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: j.namespace,
							Labels: map[string]string{
								labelKeyJobId:  j.job.Identification(),
								labelKeyCtrlBy: "bazel-build",
							},
						},
						Spec: secretsstorev1.SecretProviderClassSpec{
							Provider: "vault",
							Parameters: map[string]string{
								"roleName":     fmt.Sprintf("build-%s", j.job.RepositoryName),
								"vaultAddress": j.vaultAddr,
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
		case *config.RegistrySecret:
			if secret.Host == "" {
				continue
			}

			if _, ok := registrySecretProviderClass[secret.Host]; !ok {
				name := fmt.Sprintf("%s-%d%s-%s", j.job.RepositoryName, j.task.Id, platformName, strings.Replace(secret.Host, ".", "-", -1))
				registrySecretProviderClass[secret.Host] = &secretsstorev1.SecretProviderClass{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: j.namespace,
						Labels: map[string]string{
							labelKeyJobId:  j.job.Identification(),
							labelKeyCtrlBy: "bazel-build",
						},
					},
					Spec: secretsstorev1.SecretProviderClassSpec{
						Provider: "vault",
						Parameters: map[string]string{
							"roleName":     fmt.Sprintf("build-%s", j.job.RepositoryName),
							"vaultAddress": j.vaultAddr,
							"objects": fmt.Sprintf(
								"- objectName: %q\n  secretPath: %q\n  secretKey: %q\n",
								secret.VaultKey,
								fmt.Sprintf("%s/data/%s", secret.VaultMount, secret.VaultPath),
								secret.VaultKey,
							),
						},
					},
				}
			}
		}
	}
	if len(secretProviderClasses) > 0 {
		for mountPath, class := range secretProviderClasses {
			envVolume := k8sfactory.NewSecretStoreVolumeSource(class.Name, mountPath)
			j.mainContainer = k8sfactory.ContainerFactory(j.mainContainer, k8sfactory.Volume(envVolume))
			j.buildPod = k8sfactory.PodFactory(j.buildPod, k8sfactory.Volume(envVolume))
			j.secretProviderClasses = append(j.secretProviderClasses, class)
		}
	}
	if len(registrySecretProviderClass) > 0 {
		credVol := k8sfactory.NewEmptyDirVolumeSource("containerregistry", "/root/.docker")
		j.credentialSetupContainer = k8sfactory.ContainerFactory(nil,
			k8sfactory.Name("credential"),
			k8sfactory.Image(j.sidecarImage, nil),
			k8sfactory.Args("credential", "container-registry", fmt.Sprintf("--out=%s/config.json", credVol.Mount.MountPath), "--dir=/etc/registry"),
			k8sfactory.Volume(credVol),
		)
		j.mainContainer = k8sfactory.ContainerFactory(j.mainContainer, k8sfactory.Volume(credVol))
		j.buildPod = k8sfactory.PodFactory(j.buildPod, k8sfactory.Volume(credVol))

		for host, class := range registrySecretProviderClass {
			envVolume := k8sfactory.NewSecretStoreVolumeSource(class.Name, "/etc/registry/"+host)
			j.credentialSetupContainer = k8sfactory.ContainerFactory(j.credentialSetupContainer, k8sfactory.Volume(envVolume))
			j.buildPod = k8sfactory.PodFactory(j.buildPod, k8sfactory.Volume(envVolume))
			j.secretProviderClasses = append(j.secretProviderClasses, class)
		}
	}
}
