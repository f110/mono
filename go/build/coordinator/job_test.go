package coordinator

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/watcher"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/k8s/k8sfactory"
	"go.f110.dev/mono/go/k8s/k8smanifest"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/varptr"
)

func TestMain(m *testing.M) {
	logger.Log = zap.NewNop()
	m.Run()
}

func TestJobBuilder_Clone(t *testing.T) {
	b := NewJobBuilder("default", "bazel", "sidecar")
	assert.False(t, b.useBazelisk)
	n := b.Clone()
	b.UseBazelisk()
	assert.True(t, b.useBazelisk)
	assert.False(t, n.useBazelisk)
}

func TestJobBuilder(t *testing.T) {
	job, repo, task, saObject, jobObject := testJobBuilderFixtures()

	cases := []struct {
		Platform       string
		Mutation       func(*config.Job, *database.SourceRepository, *database.Task) (*config.Job, *database.SourceRepository, *database.Task)
		Error          string
		ExpectObjects  []runtime.Object
		ObjectMutation map[runtime.Object][]k8sfactory.Trait
	}{
		{
			Mutation: func(_ *config.Job, r *database.SourceRepository, ta *database.Task) (*config.Job, *database.SourceRepository, *database.Task) {
				return nil, r, ta
			},
			Platform: "@io_bazel_rules_go//go/toolchain:linux_amd64",
			Error:    "job is not set",
		},
		{
			Mutation: func(j *config.Job, r *database.SourceRepository, ta *database.Task) (*config.Job, *database.SourceRepository, *database.Task) {
				j.RepositoryName = ""
				return j, r, ta
			},
			Platform: "@io_bazel_rules_go//go/toolchain:linux_amd64",
			Error:    "job is not set",
		},
		{
			Mutation: func(j *config.Job, r *database.SourceRepository, _ *database.Task) (*config.Job, *database.SourceRepository, *database.Task) {
				return j, r, nil
			},
			Platform: "@io_bazel_rules_go//go/toolchain:linux_amd64",
			Error:    "task is not set",
		},
		{
			Mutation: func(j *config.Job, _ *database.SourceRepository, ta *database.Task) (*config.Job, *database.SourceRepository, *database.Task) {
				return j, nil, ta
			},
			Platform: "@io_bazel_rules_go//go/toolchain:linux_amd64",
			Error:    "repo is not set",
		},
		{
			Mutation: func(j *config.Job, r *database.SourceRepository, ta *database.Task) (*config.Job, *database.SourceRepository, *database.Task) {
				return j, r, ta
			},
			Error: "platform is not set",
		},
		{
			Mutation: func(j *config.Job, r *database.SourceRepository, ta *database.Task) (*config.Job, *database.SourceRepository, *database.Task) {
				return j, r, ta
			},
			Platform:      "@io_bazel_rules_go//go/toolchain:linux_amd64",
			ExpectObjects: []runtime.Object{saObject, jobObject},
		},
		{
			Mutation: func(j *config.Job, r *database.SourceRepository, ta *database.Task) (*config.Job, *database.SourceRepository, *database.Task) {
				j.Command = "run"
				ta.Command = "run"
				return j, r, ta
			},
			Platform:      "@io_bazel_rules_go//go/toolchain:linux_amd64",
			ExpectObjects: []runtime.Object{saObject, jobObject},
			ObjectMutation: map[runtime.Object][]k8sfactory.Trait{
				jobObject: {
					RemoveContainer("report"),
					RemoveVolume("comm"),
					k8sfactory.OnContainer("main", k8sfactory.Args("run", "--remote_cache=127.0.0.1:4567", "--experimental_remote_downloader=127.0.0.1:4567", "--platforms=@io_bazel_rules_go//go/toolchain:linux_amd64", "//...")),
				},
			},
		},
		{
			Mutation: func(j *config.Job, r *database.SourceRepository, ta *database.Task) (*config.Job, *database.SourceRepository, *database.Task) {
				ta.IsTrunk = false
				return j, r, ta
			},
			Platform:      "@io_bazel_rules_go//go/toolchain:linux_amd64",
			ExpectObjects: []runtime.Object{saObject, jobObject},
			ObjectMutation: map[runtime.Object][]k8sfactory.Trait{
				jobObject: {
					RemoveContainer("report"),
					RemoveVolume("comm"),
					k8sfactory.OnContainer("main", RemoveArgs("--build_event_binary_file=/comm/bep", "--cache_test_results=no"), AddArgsBefore("--", "--remote_upload_local_results=false")),
				},
			},
		},
		{
			Mutation: func(j *config.Job, r *database.SourceRepository, ta *database.Task) (*config.Job, *database.SourceRepository, *database.Task) {
				j.CPULimit = "2000m"
				j.MemoryLimit = "2048Mi"
				return j, r, ta
			},
			Platform:      "@io_bazel_rules_go//go/toolchain:linux_amd64",
			ExpectObjects: []runtime.Object{saObject, jobObject},
			ObjectMutation: map[runtime.Object][]k8sfactory.Trait{
				jobObject: {k8sfactory.OnContainer("main", k8sfactory.ResourceLimit(resource.MustParse("2000m"), resource.MustParse("2048Mi")))},
			},
		},
		{
			Mutation: func(j *config.Job, r *database.SourceRepository, ta *database.Task) (*config.Job, *database.SourceRepository, *database.Task) {
				j.Env = map[string]any{"FOOBAR": "baz"}
				return j, r, ta
			},
			Platform:      "@io_bazel_rules_go//go/toolchain:linux_amd64",
			ExpectObjects: []runtime.Object{saObject, jobObject},
			ObjectMutation: map[runtime.Object][]k8sfactory.Trait{
				jobObject: {k8sfactory.OnContainer("main", k8sfactory.EnvVar("FOOBAR", "baz"))},
			},
		},
		{
			Mutation: func(j *config.Job, r *database.SourceRepository, ta *database.Task) (*config.Job, *database.SourceRepository, *database.Task) {
				r.Private = true
				return j, r, ta
			},
			Platform:      "@io_bazel_rules_go//go/toolchain:linux_amd64",
			ExpectObjects: []runtime.Object{saObject, jobObject},
			ObjectMutation: map[runtime.Object][]k8sfactory.Trait{
				jobObject: {
					AddSecretVolume("pre-process", "github-secret", "githubapp-secret", "/etc/github"),
					k8sfactory.SortVolume(),
					k8sfactory.OnContainer("pre-process", AddArgs("--github-app-id=2", "--github-installation-id=20", "--private-key-file=/etc/github/privatekey.pem")),
				},
			},
		},
		{
			Mutation: func(j *config.Job, r *database.SourceRepository, ta *database.Task) (*config.Job, *database.SourceRepository, *database.Task) {
				ta.ConfigName = "e2e"
				return j, r, ta
			},
			Platform:      "@io_bazel_rules_go//go/toolchain:linux_amd64",
			ExpectObjects: []runtime.Object{saObject, jobObject},
			ObjectMutation: map[runtime.Object][]k8sfactory.Trait{
				jobObject: {
					k8sfactory.OnContainer("main",
						k8sfactory.Args("test", "--remote_cache=127.0.0.1:4567", "--experimental_remote_downloader=127.0.0.1:4567", "--config=e2e", "--platforms=@io_bazel_rules_go//go/toolchain:linux_amd64",
							"--build_event_binary_file=/comm/bep", "--cache_test_results=no",
							"--", job.Targets[0],
						),
					),
				},
			},
		},
		{
			Mutation: func(j *config.Job, r *database.SourceRepository, ta *database.Task) (*config.Job, *database.SourceRepository, *database.Task) {
				j.Secrets = []starlark.Value{&config.Secret{MountPath: "/etc/job/secret", VaultMount: "secrets", VaultPath: "login", VaultKey: "password"}}
				return j, r, ta
			},
			Platform: "@io_bazel_rules_go//go/toolchain:linux_amd64",
			ExpectObjects: []runtime.Object{
				saObject,
				jobObject,
				k8sfactory.NewSecretProviderClassFactory(nil,
					k8sfactory.Namef("%s-%d-linux-amd64-etc-job-secret", job.RepositoryName, task.Id), k8sfactory.Namespace(metav1.NamespaceDefault),
					k8sfactory.Labels(map[string]string{labelKeyCtrlBy: "bazel-build", labelKeyJobId: job.Identification()}),
					k8sfactory.Parameters(map[string]string{"objects": "- objectName: \"password\"\n  secretPath: \"secrets/data/login\"\n  secretKey: \"password\"\n", "roleName": "build-" + job.RepositoryName, "vaultAddress": "https://127.0.0.1:7000"}),
					k8sfactory.Provider("vault"),
				),
			},
			ObjectMutation: map[runtime.Object][]k8sfactory.Trait{
				jobObject: {
					k8sfactory.OnContainer("main", k8sfactory.Volume(&k8sfactory.VolumeSource{Mount: corev1.VolumeMount{Name: "example-100-linux-amd64-etc-job-secret", ReadOnly: true, MountPath: "/etc/job/secret"}})),
					AddCSIVolume(fmt.Sprintf("%s-%d-linux-amd64-etc-job-secret", job.RepositoryName, task.Id), &corev1.CSIVolumeSource{Driver: "secrets-store.csi.k8s.io", ReadOnly: varptr.Ptr(true), VolumeAttributes: map[string]string{"secretProviderClass": fmt.Sprintf("%s-%d-linux-amd64-etc-job-secret", job.RepositoryName, task.Id)}}),
					k8sfactory.SortVolume(),
				},
			},
		},
		{
			Mutation: func(j *config.Job, r *database.SourceRepository, ta *database.Task) (*config.Job, *database.SourceRepository, *database.Task) {
				j.Secrets = []starlark.Value{&config.RegistrySecret{Host: "registry.example.com", VaultMount: "secrets", VaultPath: "registry", VaultKey: "foobar"}}
				return j, r, ta
			},
			Platform: "@io_bazel_rules_go//go/toolchain:linux_amd64",
			ExpectObjects: []runtime.Object{
				saObject,
				jobObject,
				k8sfactory.NewSecretProviderClassFactory(nil,
					k8sfactory.Namef("%s-%d-linux-amd64-registry-example-com", job.RepositoryName, task.Id), k8sfactory.Namespace(metav1.NamespaceDefault),
					k8sfactory.Labels(map[string]string{labelKeyCtrlBy: "bazel-build", labelKeyJobId: job.Identification()}),
					k8sfactory.Parameters(map[string]string{"objects": "- objectName: \"foobar\"\n  secretPath: \"secrets/data/registry\"\n  secretKey: \"foobar\"\n", "roleName": "build-" + job.RepositoryName, "vaultAddress": "https://127.0.0.1:7000"}),
					k8sfactory.Provider("vault"),
				),
			},
			ObjectMutation: map[runtime.Object][]k8sfactory.Trait{
				jobObject: {
					k8sfactory.InitContainer(k8sfactory.ContainerFactory(nil,
						k8sfactory.Name("credential"),
						k8sfactory.Image("registry/sidecar", nil),
						k8sfactory.Args("credential", "container-registry", "--out=/root/.docker/config.json", "--dir=/etc/registry"),
						k8sfactory.Volume(&k8sfactory.VolumeSource{Mount: corev1.VolumeMount{Name: "containerregistry", MountPath: "/root/.docker"}}),
						k8sfactory.Volume(&k8sfactory.VolumeSource{Mount: corev1.VolumeMount{Name: fmt.Sprintf("%s-%d-linux-amd64-registry-example-com", job.RepositoryName, task.Id), ReadOnly: true, MountPath: "/etc/registry/registry.example.com"}}),
					)),
					k8sfactory.OnContainer("main", k8sfactory.Volume(&k8sfactory.VolumeSource{Mount: corev1.VolumeMount{Name: "containerregistry", MountPath: "/root/.docker"}})),
					AddCSIVolume(fmt.Sprintf("%s-%d-linux-amd64-registry-example-com", job.RepositoryName, task.Id), &corev1.CSIVolumeSource{Driver: "secrets-store.csi.k8s.io", ReadOnly: varptr.Ptr(true), VolumeAttributes: map[string]string{"secretProviderClass": fmt.Sprintf("%s-%d-linux-amd64-registry-example-com", job.RepositoryName, task.Id)}}),
					AddEmptyVolume("containerregistry"),
					k8sfactory.SortVolume(),
				},
			},
		},
		{
			Mutation: func(j *config.Job, r *database.SourceRepository, ta *database.Task) (*config.Job, *database.SourceRepository, *database.Task) {
				j.Container = "example.com/bazel:bazelisk"
				return j, r, ta
			},
			Platform:      "@io_bazel_rules_go//go/toolchain:linux_amd64",
			ExpectObjects: []runtime.Object{saObject, jobObject},
			ObjectMutation: map[runtime.Object][]k8sfactory.Trait{
				jobObject: {k8sfactory.OnContainer("main", k8sfactory.Image("example.com/bazel:bazelisk", nil))},
			},
		},
		{
			Mutation: func(j *config.Job, r *database.SourceRepository, ta *database.Task) (*config.Job, *database.SourceRepository, *database.Task) {
				j.Command = "run"
				ta.Command = "run"
				j.Args = []string{"--verbose"}
				return j, r, ta
			},
			Platform:      "@io_bazel_rules_go//go/toolchain:linux_amd64",
			ExpectObjects: []runtime.Object{saObject, jobObject},
			ObjectMutation: map[runtime.Object][]k8sfactory.Trait{
				jobObject: {
					RemoveContainer("report"),
					RemoveVolume("comm"),
					k8sfactory.OnContainer("main",
						k8sfactory.Args("run", "--remote_cache=127.0.0.1:4567", "--experimental_remote_downloader=127.0.0.1:4567", "--platforms=@io_bazel_rules_go//go/toolchain:linux_amd64", "//...", "--", "--verbose"),
					),
				},
			},
		},
	}

	require.NoError(t, logger.Init())
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			b := NewJobBuilder(metav1.NamespaceDefault, "registry/bazel", "registry/sidecar")
			b.BazelBinaryMirror("https://bazelmirror")
			b.DefaultBazelVersion("6.0.0")
			b.UseBazelisk()
			b.PullAlways()
			b.GitHubApp(2, 20, "githubapp-secret")
			b.DefaultLimit(resource.MustParse("1000m"), resource.MustParse("1024Mi"))
			b.EnableRemoteCache("127.0.0.1:4567")
			b.EnableRemoteAssetAPI()
			b.Vault("https://127.0.0.1:7000")

			var j *config.Job
			var r *database.SourceRepository
			var ta *database.Task
			if tc.Mutation != nil {
				j, r, ta = tc.Mutation(job.Copy(), repo.Copy(), task.Copy())
			} else {
				j, r, ta = job.Copy(), repo.Copy(), task.Copy()
			}
			if j != nil {
				b.Job(j)
			}
			if r != nil {
				b.Repo(r)
			}
			if ta != nil {
				b.Task(ta)
			}
			if tc.Platform != "" {
				b.Platform(tc.Platform)
			}
			objs, err := b.Build()
			if tc.Error != "" {
				require.Error(t, err, err)
				assert.EqualError(t, err, tc.Error)
			} else {
				require.NoError(t, err)

				expectObjects := make(map[objectIdentifier]runtime.Object)
				for _, o := range tc.ExpectObjects {
					m, err := meta.Accessor(o)
					require.NoError(t, err)
					gvk := o.GetObjectKind().GroupVersionKind()
					id := objectIdentifier{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind, Namespace: m.GetNamespace(), Name: m.GetName()}
					expectObjects[id] = o
				}

				for _, obj := range objs {
					m, err := meta.Accessor(obj)
					require.NoError(t, err)
					gvk := obj.GetObjectKind().GroupVersionKind()
					id := objectIdentifier{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind, Namespace: m.GetNamespace(), Name: m.GetName()}

					expectObject, ok := expectObjects[id]
					if !assert.True(t, ok, "expect object is not found") {
						t.Log(id)
					}
					delete(expectObjects, id)

					if tc.ObjectMutation != nil {
						obj := expectObject.DeepCopyObject()
						for _, m := range tc.ObjectMutation[expectObject] {
							m(obj)
						}
						expectObject = obj
					}
					assert.Equal(t, expectObject, obj)
				}
				assert.Len(t, expectObjects, 0, "Some objects are not built")
			}
		})
	}
}

func RemoveVolume(name string) k8sfactory.Trait {
	var fn k8sfactory.Trait
	fn = func(obj any) {
		switch v := obj.(type) {
		case *corev1.PodSpec:
			for i, vol := range v.Volumes {
				if vol.Name == name {
					v.Volumes = append(v.Volumes[:i], v.Volumes[i+1:]...)
					break
				}
			}
			for k := range v.InitContainers {
				con := &v.InitContainers[k]
				for i, vm := range con.VolumeMounts {
					if vm.Name == name {
						con.VolumeMounts = append(con.VolumeMounts[:i], con.VolumeMounts[i+1:]...)
						break
					}
				}
			}
			for k := range v.Containers {
				con := &v.Containers[k]
				for i, vm := range con.VolumeMounts {
					if vm.Name == name {
						con.VolumeMounts = append(con.VolumeMounts[:i], con.VolumeMounts[i+1:]...)
						break
					}
				}
			}
		case *batchv1.Job:
			fn(&v.Spec.Template.Spec)
		}
	}
	return fn
}

func AddSecretVolume(containerName, volName, secretName, mountPath string) k8sfactory.Trait {
	var fn k8sfactory.Trait
	fn = func(obj any) {
		switch v := obj.(type) {
		case *corev1.PodSpec:
			for i := range v.InitContainers {
				con := &v.InitContainers[i]
				if con.Name == containerName {
					con.VolumeMounts = append(con.VolumeMounts, corev1.VolumeMount{Name: volName, MountPath: mountPath, ReadOnly: true})
					break
				}
			}
			for i := range v.Containers {
				con := &v.Containers[i]
				if con.Name == containerName {
					con.VolumeMounts = append(con.VolumeMounts, corev1.VolumeMount{Name: volName, MountPath: mountPath, ReadOnly: true})
					break
				}
			}
			v.Volumes = append(v.Volumes, corev1.Volume{Name: volName, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: secretName}}})
		case *batchv1.Job:
			fn(&v.Spec.Template.Spec)
		}
	}
	return fn
}

func AddCSIVolume(name string, csi *corev1.CSIVolumeSource) k8sfactory.Trait {
	var fn k8sfactory.Trait
	fn = func(obj any) {
		switch v := obj.(type) {
		case *corev1.PodSpec:
			v.Volumes = append(v.Volumes, corev1.Volume{Name: name, VolumeSource: corev1.VolumeSource{CSI: csi}})
		case *batchv1.Job:
			fn(&v.Spec.Template.Spec)
		}
	}
	return fn
}

func AddEmptyVolume(name string) k8sfactory.Trait {
	var fn k8sfactory.Trait
	fn = func(obj any) {
		switch v := obj.(type) {
		case *corev1.PodSpec:
			v.Volumes = append(v.Volumes, corev1.Volume{Name: name, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}})
		case *batchv1.Job:
			fn(&v.Spec.Template.Spec)
		}
	}
	return fn
}

func RemoveContainer(name string) k8sfactory.Trait {
	var fn k8sfactory.Trait
	fn = func(obj any) {
		switch v := obj.(type) {
		case *corev1.PodSpec:
			for i, con := range v.InitContainers {
				if con.Name == name {
					v.InitContainers = append(v.InitContainers[:i], v.InitContainers[i+1:]...)
					break
				}
			}
			for i, con := range v.Containers {
				if con.Name == name {
					v.Containers = append(v.Containers[:i], v.Containers[i+1:]...)
					break
				}
			}
		case *batchv1.Job:
			fn(&v.Spec.Template.Spec)
		}
	}
	return fn
}

func RemoveArgs(args ...string) k8sfactory.Trait {
	return func(obj any) {
		m := make(map[string]struct{})
		for _, v := range args {
			m[v] = struct{}{}
		}

		switch v := obj.(type) {
		case *corev1.Container:
			args := make([]string, 0)
			for _, a := range v.Args {
				if _, ok := m[a]; !ok {
					args = append(args, a)
				}
			}
			v.Args = args
		}
	}
}

func AddArgs(args ...string) k8sfactory.Trait {
	return func(obj any) {
		switch v := obj.(type) {
		case *corev1.Container:
			v.Args = append(v.Args, args...)
		}
	}
}

func AddArgsBefore(before string, args ...string) k8sfactory.Trait {
	return func(obj any) {
		switch v := obj.(type) {
		case *corev1.Container:
			v.Args = enumerable.InsertBefore(v.Args, before, args...)
		}
	}
}

type objectIdentifier struct {
	Group     string
	Version   string
	Kind      string
	Namespace string
	Name      string
}

func printAsManifest(obj runtime.Object) {
	k8smanifest.NewEncoder(os.Stdout).Encode(obj)
}

func testJobBuilderFixtures() (*config.Job, *database.SourceRepository, *database.Task, *corev1.ServiceAccount, *batchv1.Job) {
	job := &config.Job{
		Name:            "test_all",
		RepositoryOwner: "f110",
		RepositoryName:  "example",
		Command:         "test",
		Targets:         []string{"//..."},
	}
	repo := &database.SourceRepository{
		Id:       1,
		CloneUrl: "https://github.com/f110/example.git",
	}
	task := &database.Task{
		Id:       100,
		IsTrunk:  true,
		Command:  job.Command,
		Targets:  strings.Join(job.Targets, "\n"),
		Revision: "e192aef54cb0d31afd7cae64b079be2a12a56a74",
	}
	saObject := k8sfactory.ServiceAccountFactory(&corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-" + job.RepositoryName,
			Namespace: metav1.NamespaceDefault,
		},
	})
	jobObject := k8sfactory.JobFactory(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d-linux-amd64", job.RepositoryName, task.Id),
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				labelKeyRepoId:    fmt.Sprintf("%d", repo.Id),
				labelKeyJobId:     job.Identification(),
				labelKeyTaskId:    fmt.Sprintf("%d", task.Id),
				labelKeyCtrlBy:    "bazel-build",
				watcher.TypeLabel: "bazelBuilder",
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{labelKeyCtrlBy: "bazel-build", labelKeyTaskId: fmt.Sprintf("%d", task.Id)},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "build-" + job.RepositoryName,
					RestartPolicy:      corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						{
							Name:            "pre-process",
							Image:           "registry/sidecar",
							ImagePullPolicy: corev1.PullAlways,
							Args:            []string{"clone", "--work-dir=/work", "--url=https://github.com/f110/example.git", "--commit=e192aef54cb0d31afd7cae64b079be2a12a56a74"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "workdir",
									MountPath: "/work",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "main",
							Image:           "registry/bazel:bazelisk",
							ImagePullPolicy: corev1.PullAlways,
							Args: []string{
								"test", "--remote_cache=127.0.0.1:4567", "--experimental_remote_downloader=127.0.0.1:4567", "--platforms=@io_bazel_rules_go//go/toolchain:linux_amd64",
								"--build_event_binary_file=/comm/bep", "--cache_test_results=no",
								"--", job.Targets[0],
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("1024Mi"),
								},
							},
							Env: []corev1.EnvVar{
								{Name: "WORKSPACE", Value: "/work"},
								{Name: "BAZELISK_FORMAT_URL", Value: "https://bazelmirror/bazel-%v-%o-%m%e"},
							},
							WorkingDir: "/work",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "workdir",
									MountPath: "/work",
								},
								{
									Name:      "comm",
									MountPath: "/comm",
								},
							},
						},
						{
							Name:            "report",
							Image:           "registry/sidecar",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            []string{"report", "--event-binary-file=/comm/bep", "--startup-timeout=10m"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "comm",
									MountPath: "/comm",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "comm",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "workdir",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
			PodFailurePolicy: &batchv1.PodFailurePolicy{
				Rules: []batchv1.PodFailurePolicyRule{
					{Action: batchv1.PodFailurePolicyActionFailJob, OnExitCodes: &batchv1.PodFailurePolicyOnExitCodesRequirement{ContainerName: varptr.Ptr("main"), Operator: batchv1.PodFailurePolicyOnExitCodesOpNotIn, Values: []int32{0}}},
				},
			},
			BackoffLimit: varptr.Ptr[int32](0),
		},
	})

	return job, repo, task, saObject, jobObject
}
