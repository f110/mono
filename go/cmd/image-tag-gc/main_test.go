package main

import (
	"context"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"go.f110.dev/kubeproto/go/apis/appsv1"
	"go.f110.dev/kubeproto/go/apis/batchv1"
	"go.f110.dev/kubeproto/go/apis/corev1"
	"go.f110.dev/kubeproto/go/apis/metav1"
	"go.f110.dev/kubeproto/go/k8stestingclient"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/build/api"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestFindImageReferences(t *testing.T) {
	repositories := []string{"repo.example.com/build/build", "repo.example.com/build/sidecar"}
	cases := []struct {
		name   string
		values []string
		want   map[string]*imageReferences
	}{
		{
			name:   "tag",
			values: []string{"repo.example.com/build/build:10"},
			want:   map[string]*imageReferences{"repo.example.com/build/build": {Tags: []string{"10"}}},
		},
		{
			name:   "digest",
			values: []string{"repo.example.com/build/build@sha256:aaa"},
			want:   map[string]*imageReferences{"repo.example.com/build/build": {Digests: []string{"sha256:aaa"}}},
		},
		{
			name:   "tag and digest",
			values: []string{"repo.example.com/build/build:10@sha256:aaa"},
			want:   map[string]*imageReferences{"repo.example.com/build/build": {Tags: []string{"10"}, Digests: []string{"sha256:aaa"}}},
		},
		{
			name:   "image id with the scheme",
			values: []string{"docker-pullable://repo.example.com/build/build@sha256:aaa"},
			want:   map[string]*imageReferences{"repo.example.com/build/build": {Digests: []string{"sha256:aaa"}}},
		},
		{
			name:   "argument",
			values: []string{"--sidecar-image=repo.example.com/build/sidecar@sha256:bbb"},
			want:   map[string]*imageReferences{"repo.example.com/build/sidecar": {Digests: []string{"sha256:bbb"}}},
		},
		{
			name:   "command line",
			values: []string{"builder --sidecar-image repo.example.com/build/sidecar:3 --dev"},
			want:   map[string]*imageReferences{"repo.example.com/build/sidecar": {Tags: []string{"3"}}},
		},
		{
			name:   "repository which has the same prefix",
			values: []string{"repo.example.com/build/buildctl:10", "repo.example.com/build/build"},
			want:   map[string]*imageReferences{},
		},
		{
			name:   "other repository",
			values: []string{"registry.example.com/build/build:10"},
			want:   map[string]*imageReferences{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findImageReferences(tc.values, repositories)
			assertion.Equal(t, tc.want, got)
		})
	}
}

func TestDeletableTags(t *testing.T) {
	defaultTags := map[string]string{"1": "sha256:1", "2": "sha256:2", "10": "sha256:10", "11": "sha256:11"}
	cases := []struct {
		name string
		tags map[string]string
		refs *imageReferences
		want []string
	}{
		{
			name: "no reference keeps the latest",
			want: []string{"1", "2", "10"},
		},
		{
			name: "referred by the tag",
			refs: &imageReferences{Tags: []string{"2"}},
			want: []string{"1", "10"},
		},
		{
			name: "referred by the digest",
			refs: &imageReferences{Digests: []string{"sha256:1"}},
			want: []string{"2", "10"},
		},
		{
			name: "the digest which has the other tag",
			tags: map[string]string{"1": "sha256:1", "2": "sha256:2", "stable": "sha256:2", "10": "sha256:10"},
			want: []string{"1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tags := tc.tags
			if tags == nil {
				tags = defaultTags
			}
			assertion.Equal(t, tc.want, deletableTags(tags, tc.refs))
		})
	}
}

func TestImageTagGC(t *testing.T) {
	assertion.MustNoError(t, logger.Init())
	s := httptest.NewServer(registry.New())
	t.Cleanup(s.Close)
	host := strings.TrimPrefix(s.URL, "http://")
	serverRepo := host + "/issue/server"
	sidecarRepo := host + "/build/sidecar"

	server := map[string]v1.Hash{}
	for _, tag := range []string{"1", "2", "3", "4", "5", "latest"} {
		server[tag] = pushRandomImage(t, serverRepo+":"+tag)
	}
	assertion.MustNoError(t, crane.Tag(serverRepo+":3", "stable"))
	sidecar := map[string]v1.Hash{}
	for _, tag := range []string{"7", "8", "9"} {
		sidecar[tag] = pushRandomImage(t, sidecarRepo+":"+tag)
	}

	k8sClient := k8stestingclient.NewSet()
	objs := []runtime.Object{
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "running", Namespace: "issue"},
			Spec:       &corev1.PodSpec{Containers: []corev1.Container{{Name: "server", Image: serverRepo + ":4"}}},
			Status: &corev1.PodStatus{
				Phase:             corev1.PodPhaseRunning,
				ContainerStatuses: []corev1.ContainerStatus{{Name: "server", ImageID: serverRepo + "@" + server["4"].String()}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "succeeded", Namespace: "issue"},
			Spec:       &corev1.PodSpec{Containers: []corev1.Container{{Name: "server", Image: serverRepo + ":1"}}},
			Status:     &corev1.PodStatus{Phase: corev1.PodPhaseSucceeded},
		},
		&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{Name: "server-old", Namespace: "issue"},
			Spec: &appsv1.ReplicaSetSpec{
				Template: &corev1.PodTemplateSpec{
					Spec: &corev1.PodSpec{Containers: []corev1.Container{{Name: "server", Image: serverRepo + "@" + server["2"].String()}}},
				},
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "builder", Namespace: "build"},
			Spec: &appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: &corev1.PodSpec{Containers: []corev1.Container{{Name: "builder", Args: []string{"--sidecar-image=" + sidecarRepo + "@" + sidecar["8"].String()}}}},
				},
			},
		},
		&batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{Name: "gc", Namespace: "build"},
			Spec: &batchv1.CronJobSpec{
				JobTemplate: batchv1.JobTemplateSpec{
					Spec: &batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: &corev1.PodSpec{Containers: []corev1.Container{{Name: "gc", Image: sidecarRepo + ":7"}}},
						},
					},
				},
			},
		},
	}
	for _, v := range objs {
		assertion.MustNoError(t, k8sClient.Tracker().Add(v))
	}

	buildClient := &fakeBuildClient{repositories: []string{serverRepo, sidecarRepo}}
	gc := newImageTagGC(buildClient, &k8sClient.Set, true, false)
	assertion.MustNoError(t, gc.Run(context.Background()))

	tags, err := crane.ListTags(serverRepo)
	assertion.MustNoError(t, err)
	slices.Sort(tags)
	assertion.Equal(t, []string{"2", "3", "4", "5", "latest", "stable"}, tags)
	tags, err = crane.ListTags(sidecarRepo)
	assertion.MustNoError(t, err)
	slices.Sort(tags)
	assertion.Equal(t, []string{"7", "8", "9"}, tags)
}

type fakeBuildClient struct {
	api.APIClient
	repositories []string
}

func (f *fakeBuildClient) ListContainerImageRepositories(_ context.Context, _ *api.RequestListContainerImageRepositories, _ ...grpc.CallOption) (*api.ResponseListContainerImageRepositories, error) {
	return api.ResponseListContainerImageRepositories_builder{Repositories: f.repositories}.Build(), nil
}

func pushRandomImage(t *testing.T, ref string) v1.Hash {
	img, err := random.Image(64, 1)
	assertion.MustNoError(t, err)
	assertion.MustNoError(t, crane.Push(img, ref))
	digest, err := img.Digest()
	assertion.MustNoError(t, err)
	return digest
}
