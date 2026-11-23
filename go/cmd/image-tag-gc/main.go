package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"go.f110.dev/kubeproto/go/apis/corev1"
	"go.f110.dev/kubeproto/go/apis/metav1"
	"go.f110.dev/kubeproto/go/k8sclient"
	"go.f110.dev/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/client-go/rest"

	"go.f110.dev/mono/go/build/api"
	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/logger"
)

// imageTagGC deletes the tags of the container images which Build pushed and nobody uses.
// The tag which Build pushes is the ID of the task. Once the tag is deleted, the registry removes the manifest.
type imageTagGC struct {
	buildClient api.APIClient
	k8sClient   *k8sclient.Set
	insecure    bool
	dryRun      bool
}

func newImageTagGC(buildClient api.APIClient, k8sClient *k8sclient.Set, insecure, dryRun bool) *imageTagGC {
	return &imageTagGC{buildClient: buildClient, k8sClient: k8sClient, insecure: insecure, dryRun: dryRun}
}

func (gc *imageTagGC) Run(ctx context.Context) error {
	res, err := gc.buildClient.ListContainerImageRepositories(ctx, &api.RequestListContainerImageRepositories{})
	if err != nil {
		return xerrors.WithStack(err)
	}
	repositories := res.GetRepositories()
	values, err := gc.listImageValues(ctx)
	if err != nil {
		return err
	}
	refs := findImageReferences(values, repositories)

	for _, v := range repositories {
		var nameOpts []name.Option
		if gc.insecure {
			nameOpts = append(nameOpts, name.Insecure)
		}
		repo, err := name.NewRepository(v, nameOpts...)
		if err != nil {
			return xerrors.WithStack(err)
		}
		tags, err := listTags(ctx, repo)
		if err != nil {
			return err
		}
		for _, tag := range deletableTags(tags, refs[v]) {
			if gc.dryRun {
				logger.Log.Info("Delete the tag (dry run)", slog.String("repository", v), slog.String("tag", tag), slog.String("digest", tags[tag]))
				continue
			}
			if err := remote.Delete(repo.Tag(tag), remote.WithContext(ctx)); err != nil && !isNotFound(err) {
				return xerrors.WithStack(err)
			}
			logger.Log.Info("Deleted the tag", slog.String("repository", v), slog.String("tag", tag), slog.String("digest", tags[tag]))
		}
	}
	return nil
}

// listImageValues returns the strings which may refer to the container images: the images of the running containers,
// and the images, the arguments and the environment variables of the pod templates of the workloads.
// The pod template of the ReplicaSet is included because Deployment keeps it as the destination of the rollback.
func (gc *imageTagGC) listImageValues(ctx context.Context) ([]string, error) {
	var values []string
	pods, err := gc.k8sClient.CoreV1.ListPod(ctx, metav1.NamespaceAll, metav1.ListOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	for _, v := range pods.Items {
		if v.Status == nil || (v.Status.Phase != corev1.PodPhasePending && v.Status.Phase != corev1.PodPhaseRunning) {
			continue
		}
		values = append(values, podSpecValues(v.Spec)...)
		for _, s := range slices.Concat(v.Status.InitContainerStatuses, v.Status.ContainerStatuses) {
			values = append(values, s.Image, s.ImageID)
		}
	}

	deployments, err := gc.k8sClient.AppsV1.ListDeployment(ctx, metav1.NamespaceAll, metav1.ListOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	for _, v := range deployments.Items {
		if v.Spec != nil {
			values = append(values, podSpecValues(v.Spec.Template.Spec)...)
		}
	}
	replicaSets, err := gc.k8sClient.AppsV1.ListReplicaSet(ctx, metav1.NamespaceAll, metav1.ListOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	for _, v := range replicaSets.Items {
		if v.Spec != nil && v.Spec.Template != nil {
			values = append(values, podSpecValues(v.Spec.Template.Spec)...)
		}
	}
	statefulSets, err := gc.k8sClient.AppsV1.ListStatefulSet(ctx, metav1.NamespaceAll, metav1.ListOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	for _, v := range statefulSets.Items {
		if v.Spec != nil {
			values = append(values, podSpecValues(v.Spec.Template.Spec)...)
		}
	}
	daemonSets, err := gc.k8sClient.AppsV1.ListDaemonSet(ctx, metav1.NamespaceAll, metav1.ListOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	for _, v := range daemonSets.Items {
		if v.Spec != nil {
			values = append(values, podSpecValues(v.Spec.Template.Spec)...)
		}
	}
	jobs, err := gc.k8sClient.BatchV1.ListJob(ctx, metav1.NamespaceAll, metav1.ListOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	for _, v := range jobs.Items {
		if v.Spec != nil {
			values = append(values, podSpecValues(v.Spec.Template.Spec)...)
		}
	}
	cronJobs, err := gc.k8sClient.BatchV1.ListCronJob(ctx, metav1.NamespaceAll, metav1.ListOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	for _, v := range cronJobs.Items {
		if v.Spec != nil && v.Spec.JobTemplate.Spec != nil {
			values = append(values, podSpecValues(v.Spec.JobTemplate.Spec.Template.Spec)...)
		}
	}
	return values, nil
}

func ImageTagGC() error {
	var buildAPI string
	var registryInsecure, dryRun bool
	cmd := &cli.Command{
		Use: "image-tag-gc",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			conn, err := grpc.NewClient(buildAPI, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return xerrors.WithStack(err)
			}
			defer conn.Close()
			cfg, err := rest.InClusterConfig()
			if err != nil {
				return xerrors.WithStack(err)
			}
			k8sClient, err := k8sclient.NewSet(cfg)
			if err != nil {
				return xerrors.WithStack(err)
			}
			return newImageTagGC(api.NewAPIClient(conn), k8sClient, registryInsecure, dryRun).Run(ctx)
		},
	}
	fs := cmd.Flags()
	fs.String("build-api", "The address of the API of Build").Var(&buildAPI).Required()
	fs.Bool("registry-insecure", "Access the registry over plain HTTP").Var(&registryInsecure)
	fs.Bool("dry-run", "Show the tags which will be deleted without deleting").Var(&dryRun)

	return cmd.Execute(os.Args)
}

// imageReferences is the tags and the digests of a repository which are referred.
type imageReferences struct {
	Tags    []string
	Digests []string
}

func main() {
	if err := ImageTagGC(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}

func podSpecValues(spec *corev1.PodSpec) []string {
	if spec == nil {
		return nil
	}
	var values []string
	for _, c := range slices.Concat(spec.InitContainers, spec.Containers) {
		values = append(values, c.Image)
		values = append(values, c.Command...)
		values = append(values, c.Args...)
		for _, e := range c.Env {
			values = append(values, e.Value)
		}
	}
	return values
}

// findImageReferences returns the tags and the digests of the repositories which the values refer to.
// The values are split into the words, so the reference in the argument like `--sidecar-image=<image>` is also found.
func findImageReferences(values, repositories []string) map[string]*imageReferences {
	refs := make(map[string]*imageReferences)
	for _, v := range values {
		words := strings.FieldsFunc(v, func(r rune) bool {
			return unicode.IsSpace(r) || r == '=' || r == ',' || r == '"' || r == '\''
		})
		for _, w := range words {
			if _, after, ok := strings.Cut(w, "://"); ok {
				w = after
			}
			for _, repo := range repositories {
				if len(w) <= len(repo) || !strings.HasPrefix(w, repo) || (w[len(repo)] != ':' && w[len(repo)] != '@') {
					continue
				}
				if refs[repo] == nil {
					refs[repo] = &imageReferences{}
				}
				rest, digest, _ := strings.Cut(w[len(repo):], "@")
				if tag, ok := strings.CutPrefix(rest, ":"); ok && tag != "" {
					refs[repo].Tags = append(refs[repo].Tags, tag)
				}
				if digest != "" {
					refs[repo].Digests = append(refs[repo].Digests, digest)
				}
			}
		}
	}
	return refs
}

// deletableTags returns the task ID tags which can be deleted from tags (the map of the tag to the digest) in ascending order.
// The tags which are referred and the latest tag are kept. The latest tag has not been deployed yet.
// The tags of the digest which has the other tag than the task ID are also kept because the other tag is attached intentionally.
func deletableTags(tags map[string]string, refs *imageReferences) []string {
	latest := -1
	var taggedDigests []string
	for tag, digest := range tags {
		if !isTaskID(tag) {
			taggedDigests = append(taggedDigests, digest)
			continue
		}
		if id, _ := strconv.Atoi(tag); id > latest {
			latest = id
		}
	}

	var deletable []int
	for tag, digest := range tags {
		if !isTaskID(tag) {
			continue
		}
		id, _ := strconv.Atoi(tag)
		if id == latest || slices.Contains(taggedDigests, digest) {
			continue
		}
		if refs != nil && (slices.Contains(refs.Tags, tag) || slices.Contains(refs.Digests, digest)) {
			continue
		}
		deletable = append(deletable, id)
	}
	slices.Sort(deletable)

	result := make([]string, len(deletable))
	for i, v := range deletable {
		result[i] = strconv.Itoa(v)
	}
	return result
}

// listTags returns the map of the tag to the digest.
func listTags(ctx context.Context, repo name.Repository) (map[string]string, error) {
	tags, err := remote.List(repo, remote.WithContext(ctx))
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, xerrors.WithStack(err)
	}
	result := make(map[string]string)
	for _, tag := range tags {
		desc, err := remote.Head(repo.Tag(tag), remote.WithContext(ctx))
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return nil, xerrors.WithStack(err)
		}
		result[tag] = desc.Digest.String()
	}
	return result, nil
}

// isTaskID reports whether the tag is the task ID which Build tags the image with.
func isTaskID(tag string) bool {
	id, err := strconv.Atoi(tag)
	return err == nil && strconv.Itoa(id) == tag
}

func isNotFound(err error) bool {
	var terr *transport.Error
	return errors.As(err, &terr) && terr.StatusCode == http.StatusNotFound
}
