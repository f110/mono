package builder

import (
	"testing"
	"time"

	"go.f110.dev/mono/go/githubutil"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestBuildServerConfig(t *testing.T) {
	ghClient := githubutil.NewGitHubClientFactory("", true)
	ghClient.AppID = 12345
	opt := Options{
		GitHubClient:                ghClient,
		Dev:                         true,
		EnableLeaderElection:        true,
		Namespace:                   "build",
		UseBazelisk:                 true,
		DefaultBazelVersion:         "7.0.0",
		RemoteCache:                 "grpc://cache:9092",
		TaskCPULimit:                "1000m",
		TaskMemoryLimit:             "4096Mi",
		WithGC:                      true,
		GitDataListen:               "0.0.0.0:9000",
		GitDataServiceURL:           "git-data:9000",
		GitDataRefreshInterval:      5 * time.Minute,
		GitDataRefreshWorkers:       3,
		ExternalReleasePollInterval: time.Hour,
		EventReconcileInterval:      30 * time.Second,
		VaultAddr:                   "https://vault:8200",
		DashboardUrl:                "http://localhost",
		BazelImage:                  "registry.f110.dev/build/bazel",
		SidecarImage:                "registry.f110.dev/build/sidecar",
		BazelMirrorURL:              "https://mirror.example.com",
		CentralRegistryMirrorURL:    "https://registry.example.com",
		RemoteAssetApi:              true,
		PullAlways:                  true,
	}

	c := buildServerConfig(opt)

	assertion.True(t, c.GetDev())
	assertion.True(t, c.GetLeaderElection())
	assertion.Equal(t, c.GetNamespace(), "build")
	assertion.True(t, c.GetUseBazelisk())
	assertion.Equal(t, c.GetDefaultBazelVersion(), "7.0.0")
	assertion.Equal(t, c.GetRemoteCache(), "grpc://cache:9092")
	assertion.Equal(t, c.GetTaskCpuLimit(), "1000m")
	assertion.Equal(t, c.GetTaskMemoryLimit(), "4096Mi")
	assertion.True(t, c.GetGcEnabled())
	assertion.Equal(t, c.GetGitDataServiceListen(), "0.0.0.0:9000")
	assertion.Equal(t, c.GetGitDataServiceUrl(), "git-data:9000")
	assertion.Equal(t, c.GetGitDataRefreshInterval(), "5m0s")
	assertion.Equal(t, c.GetGitDataRefreshWorkers(), int32(3))
	assertion.Equal(t, c.GetExternalReleasePollInterval(), "1h0m0s")
	assertion.Equal(t, c.GetEventReconcileInterval(), "30s")
	assertion.Equal(t, c.GetGithubAppId(), int64(12345))
	assertion.Equal(t, c.GetVaultAddr(), "https://vault:8200")
	assertion.Equal(t, c.GetDashboardUrl(), "http://localhost")
	assertion.Equal(t, c.GetBazelImage(), "registry.f110.dev/build/bazel")
	assertion.Equal(t, c.GetSidecarImage(), "registry.f110.dev/build/sidecar")
	assertion.Equal(t, c.GetBazelMirrorUrl(), "https://mirror.example.com")
	assertion.Equal(t, c.GetCentralRegistryMirrorUrl(), "https://registry.example.com")
	assertion.True(t, c.GetRemoteAssetApi())
	assertion.True(t, c.GetPullAlways())
}

func TestFormatConfigDuration(t *testing.T) {
	assertion.Equal(t, formatConfigDuration(0), "")
	assertion.Equal(t, formatConfigDuration(-1), "")
	assertion.Equal(t, formatConfigDuration(90*time.Second), "1m30s")
}
