package buildctl

import (
	"testing"

	"go.f110.dev/kubeproto/go/apis/batchv1"
	"go.f110.dev/kubeproto/go/apis/corev1"
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/build/api"
	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/testing/assertion"
)

func testServerConfig() *api.ServerConfig {
	return api.ServerConfig_builder{
		Namespace:                new("build"),
		BazelImage:               new("registry.f110.dev/build/bazel"),
		SidecarImage:             new("registry.f110.dev/build/sidecar"),
		DefaultBazelVersion:      new("7.0.0"),
		RemoteCache:              new("127.0.0.1:4567"),
		CentralRegistryMirrorUrl: new("https://registry.example.com"),
		GithubAppId:              new(int64(100)),
		GithubInstallationId:     new(int64(200)),
		GithubAppSecretName:      new("github-app"),
	}.Build()
}

func findJob(t *testing.T, objs []runtime.Object) *batchv1.Job {
	t.Helper()
	for _, o := range objs {
		if j, ok := o.(*batchv1.Job); ok {
			return j
		}
	}
	t.Fatal("no Job found in built objects")
	return nil
}

func countJobs(objs []runtime.Object) int {
	n := 0
	for _, o := range objs {
		if _, ok := o.(*batchv1.Job); ok {
			n++
		}
	}
	return n
}

func TestBuildManifests(t *testing.T) {
	job := &config.JobV2{
		Name:      "test-all",
		Command:   "test",
		Targets:   []string{"//..."},
		Platforms: []string{"@rules_go//go/toolchain:linux_amd64"},
	}
	objs, err := buildManifests(testServerConfig(), job, "https://github.com/f110/mono.git", "abcdef", "@rules_go//go/toolchain:linux_amd64", true)
	assertion.MustNoError(t, err)

	j := findJob(t, objs)
	// CLI manifests are named "<repository>-<job>" and carry no control labels
	// or finalizer (those are coordinator-only runtime metadata).
	assertion.Equal(t, "mono-test-all", j.GetName())
	assertion.Equal(t, "build", j.GetNamespace())
	assertion.Len(t, j.GetFinalizers(), 0)
	assertion.Len(t, j.GetLabels(), 0)

	spec := j.Spec.Template.Spec
	assertion.Equal(t, "Never", string(spec.RestartPolicy))

	// pre-process clones from the origin with GitHub App credentials for a
	// private repository. This is the behavior that the standalone
	// reimplementation used to miss.
	assertion.Len(t, spec.InitContainers, 1)
	preArgs := spec.InitContainers[0].Args
	assertion.Contains(t, preArgs, "--url=https://github.com/f110/mono.git")
	assertion.Contains(t, preArgs, "--commit=abcdef")
	assertion.Contains(t, preArgs, "--github-app-id=100")
	assertion.Contains(t, preArgs, "--github-installation-id=200")
	assertion.Contains(t, preArgs, "--private-key-file=/etc/github/privatekey.pem")
	assertion.Equal(t, "registry.f110.dev/build/sidecar", spec.InitContainers[0].Image)
}

func TestBuildManifests_JobNameUnderscoreReplaced(t *testing.T) {
	job := &config.JobV2{
		Name:      "publish_build",
		Command:   "test",
		Targets:   []string{"//..."},
		Platforms: []string{"@rules_go//go/toolchain:linux_amd64"},
	}
	objs, err := buildManifests(testServerConfig(), job, "https://github.com/f110/mono.git", "", "", false)
	assertion.MustNoError(t, err)
	assertion.Equal(t, "mono-publish-build", findJob(t, objs).GetName())
}

func TestBuildManifests_PublicOmitsGithubApp(t *testing.T) {
	job := &config.JobV2{
		Name:      "test-all",
		Command:   "test",
		Targets:   []string{"//..."},
		Platforms: []string{"@rules_go//go/toolchain:linux_amd64"},
	}
	objs, err := buildManifests(testServerConfig(), job, "https://github.com/f110/mono.git", "abcdef", "", false)
	assertion.MustNoError(t, err)

	preArgs := findJob(t, objs).Spec.Template.Spec.InitContainers[0].Args
	assertion.NotContains(t, preArgs, "--github-app-id=100")
}

func TestBuildManifests_DropsServiceAccount(t *testing.T) {
	job := &config.JobV2{
		Name:      "test-all",
		Command:   "test",
		Targets:   []string{"//..."},
		Platforms: []string{"@rules_go//go/toolchain:linux_amd64"},
	}
	objs, err := buildManifests(testServerConfig(), job, "https://github.com/f110/mono.git", "", "", false)
	assertion.MustNoError(t, err)

	// A standalone manifest must not include the ServiceAccount; it is assumed
	// to already exist in the cluster.
	for _, o := range objs {
		if _, ok := o.(*corev1.ServiceAccount); ok {
			t.Fatal("ServiceAccount must not be included in a standalone manifest")
		}
	}
}

func TestBuildManifests_Platforms(t *testing.T) {
	job := &config.JobV2{
		Name:      "test-all",
		Command:   "test",
		Targets:   []string{"//..."},
		Platforms: []string{"@rules_go//go/toolchain:linux_amd64", "@rules_go//go/toolchain:linux_arm64"},
	}

	t.Run("one Job per platform", func(t *testing.T) {
		objs, err := buildManifests(testServerConfig(), job, "https://github.com/f110/mono.git", "", "", false)
		assertion.MustNoError(t, err)
		assertion.Equal(t, 2, countJobs(objs))
	})

	t.Run("filter to a single platform", func(t *testing.T) {
		objs, err := buildManifests(testServerConfig(), job, "https://github.com/f110/mono.git", "", "@rules_go//go/toolchain:linux_arm64", false)
		assertion.MustNoError(t, err)
		assertion.Equal(t, 1, countJobs(objs))
	})

	t.Run("unknown platform is rejected", func(t *testing.T) {
		_, err := buildManifests(testServerConfig(), job, "https://github.com/f110/mono.git", "", "unknown", false)
		assertion.Error(t, err)
	})

	t.Run("a job without platforms is rejected", func(t *testing.T) {
		noPlatform := &config.JobV2{Name: "noplatform", Command: "test", Targets: []string{"//..."}}
		_, err := buildManifests(testServerConfig(), noPlatform, "https://github.com/f110/mono.git", "", "", false)
		assertion.Error(t, err)
	})
}
