package buildctl

import (
	"context"
	"fmt"
	"path"
	"strings"

	"go.f110.dev/kubeproto/go/apis/batchv1"
	"go.f110.dev/kubeproto/go/apis/corev1"
	"go.f110.dev/xerrors"
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/build/api"
	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/coordinator"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/k8s/k8smanifest"
)

func manifestCommand(endpoint *string) *cli.Command {
	var jobName, cloneURL, revision, dir, platform string
	var private bool
	manifest := &cli.Command{
		Use:   "manifest",
		Short: "Generate a standalone Kubernetes Job manifest for a job defined in .build",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			apiClient, err := newClient(endpoint)
			if err != nil {
				return err
			}
			info, err := apiClient.GetServerInfo(ctx, &api.RequestGetServerInfo{})
			if err != nil {
				return xerrors.WithStack(err)
			}

			jobs, err := config.ReadJobsFromBuildDir(config.NewLocalProvider(dir))
			if err != nil {
				return err
			}
			var job *config.JobV2
			for _, j := range jobs {
				if j.Name == jobName {
					job = j
					break
				}
			}
			if job == nil {
				return xerrors.Definef("job %q is not found in %s", jobName, dir).WithStack()
			}

			objs, err := buildManifests(info.GetConfig(), job, cloneURL, revision, platform, private)
			if err != nil {
				return err
			}
			for i, obj := range objs {
				if i != 0 {
					fmt.Println("---")
				}
				buf, err := k8smanifest.Marshal(obj)
				if err != nil {
					return err
				}
				fmt.Print(string(buf))
			}
			return nil
		},
	}
	manifest.Flags().String("name", "Job name defined in .build").Var(&jobName).Required()
	manifest.Flags().String("url", "Clone URL of the repository").Var(&cloneURL).Required()
	manifest.Flags().String("revision", "Commit or branch to checkout").Var(&revision)
	manifest.Flags().String("config", "Repository root that contains the .build directory").Shorthand("c").Default(".").Var(&dir)
	manifest.Flags().String("platform", "Build only for this platform. If empty, a manifest is generated for every platform of the job.").Var(&platform)
	manifest.Flags().Bool("private", "Treat the repository as private (adds GitHub App credentials to the pre-process container)").Var(&private)
	return manifest
}

// buildManifests builds the Job manifests for job by delegating to the
// coordinator's JobBuilder, so the output matches what the builder actually
// runs (pre-process / main containers, bazel args, GitHub App credentials for
// private repositories, Vault secrets, ...). One object set is produced per
// platform.
func buildManifests(cfg *api.ServerConfig, job *config.JobV2, cloneURL, revision, platform string, private bool) ([]runtime.Object, error) {
	platforms := job.Platforms
	if platform != "" {
		found := false
		for _, p := range job.Platforms {
			if p == platform {
				found = true
				break
			}
		}
		if !found {
			return nil, xerrors.Definef("platform %q is not defined in job %q", platform, job.Name).WithStack()
		}
		platforms = []string{platform}
	}
	if len(platforms) == 0 {
		return nil, xerrors.Definef("job %q has no platforms", job.Name).WithStack()
	}

	// JobBuilder.Job requires RepositoryName, but ReadJobsFromBuildDir does not
	// set it for a local provider. Derive it from the clone URL.
	repoName := repositoryNameFromURL(cloneURL)
	job.RepositoryName = repoName

	jb := coordinator.NewJobBuilder(cfg.GetNamespace(), cfg.GetBazelImage(), cfg.GetSidecarImage(), nil)
	jb.DefaultBazelVersion(cfg.GetDefaultBazelVersion())
	if cfg.GetUseBazelisk() {
		jb.UseBazelisk()
	}
	if cfg.GetBazelMirrorUrl() != "" {
		jb.BazelBinaryMirror(cfg.GetBazelMirrorUrl())
	}
	if cfg.GetCentralRegistryMirrorUrl() != "" {
		jb.CentralRegistryMirror(cfg.GetCentralRegistryMirrorUrl())
	}
	if cfg.GetRemoteCache() != "" {
		jb.EnableRemoteCache(cfg.GetRemoteCache())
	}
	if cfg.GetRemoteAssetApi() {
		jb.EnableRemoteAssetAPI()
	}
	if cfg.GetVaultAddr() != "" {
		jb.Vault(cfg.GetVaultAddr())
	}
	if cfg.GetPullAlways() {
		jb.PullAlways()
	}
	jb.GitHubApp(cfg.GetGithubAppId(), cfg.GetGithubInstallationId(), cfg.GetGithubAppSecretName())

	repo := &database.SourceRepository{Name: repoName, CloneUrl: cloneURL, Private: private}
	task := &database.Task{
		Revision:   revision,
		Command:    job.Command,
		ConfigName: job.ConfigName,
		Targets:    strings.Join(job.Targets, "\n"),
		IsTrunk:    false,
	}

	var objs []runtime.Object
	for _, p := range platforms {
		built, err := jb.Clone().
			Repo(repo).
			Job(job).
			Task(task).
			Platform(p).
			Build()
		if err != nil {
			return nil, err
		}
		for _, o := range built {
			// The ServiceAccount is shared infrastructure expected to already
			// exist in the cluster; a standalone manifest must not recreate it.
			if _, ok := o.(*corev1.ServiceAccount); ok {
				continue
			}
			if j, ok := o.(*batchv1.Job); ok {
				adjustForCLI(j, repoName+"-"+strings.ReplaceAll(job.Name, "_", "-"))
			}
			objs = append(objs, o)
		}
	}
	return objs, nil
}

// adjustForCLI strips coordinator-only runtime metadata (control labels and the
// reconciler finalizer) and renames the Job to "<repository>-<job>" so the
// generated manifest is convenient to apply and inspect by hand.
func adjustForCLI(j *batchv1.Job, name string) {
	j.Name = name
	j.Labels = nil
	j.Finalizers = nil
	if j.Spec != nil && j.Spec.Template.ObjectMeta != nil {
		j.Spec.Template.ObjectMeta.Labels = nil
	}
}

// repositoryNameFromURL extracts the repository name from a clone URL, e.g.
// "https://github.com/f110/mono.git" -> "mono".
func repositoryNameFromURL(u string) string {
	return path.Base(strings.TrimSuffix(u, ".git"))
}
