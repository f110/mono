package consumer

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v29/github"
	"golang.org/x/xerrors"
	"gopkg.in/src-d/go-git.v4"
	gitConfig "gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	gogitHttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/f110/tools/bot/continuous-delivery/pkg/config"
)

const (
	builderServiceAccount         = "build"
	buildSidecarImage             = "registry.f110.dev/k8s-cluster-maintenance-bot/sidecar"
	bazelImage                    = "l.gcr.io/google/bazel"
	defaultBazelVersion           = "2.0.0"
	repositoryBuildConfigFilePath = ".bot/build.yaml"

	labelKeyJobId  = "k8s-cluster-maintenance-bot.f110.dev/job-id"
	labelKeyCtrlBy = "k8s-cluster-maintenance-bot.f110.dev/control-by"
)

var (
	errBuildFailure = xerrors.New("build failed")
)

var letters = "abcdefghijklmnopqrstuvwxyz1234567890"

type BazelBuild struct {
	Namespace              string
	AppId                  int64
	InstallationId         int64
	StorageHost            string
	StorageTokenSecretName string
	ArtifactBucket         string
	HostAliases            []config.HostAlias
	AuthorName             string
	AuthorEmail            string

	transport  *ghinstallation.Transport
	workingDir string
	debug      bool
}

func errorLog(err error) {
	fmt.Fprintf(os.Stderr, "%+v\n", err)
}

func NewBuildConsumer(namespace string, conf *config.Config, debug bool) (*BazelBuild, error) {
	t, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, conf.GitHubAppId, conf.GitHubInstallationId, conf.GitHubAppPrivateKeyFile)
	if err != nil {
		return nil, xerrors.Errorf(": %v", err)
	}

	return &BazelBuild{
		Namespace:              namespace,
		AppId:                  conf.GitHubAppId,
		InstallationId:         conf.GitHubInstallationId,
		StorageHost:            conf.StorageHost,
		StorageTokenSecretName: conf.StorageTokenSecretName,
		ArtifactBucket:         conf.ArtifactBucket,
		HostAliases:            conf.HostAliases,
		AuthorName:             conf.CommitAuthor,
		AuthorEmail:            conf.CommitEmail,
		debug:                  debug,
		transport:              t,
	}, nil
}

func (b *BazelBuild) Build(e interface{}) {
	event, ok := e.(*github.PushEvent)
	if !ok {
		log.Print("Not push event")
		return
	}
	buildCtx := NewEventContextFromPushEvent(event)

	if contents, err := buildCtx.FetchRuleFile(&http.Client{Transport: b.transport}, repositoryBuildConfigFilePath); err != nil {
		errorLog(err)
		return
	} else {
		rule, err := config.ParseBuildRule(contents)
		if err != nil {
			errorLog(err)
			return
		}
		buildCtx.Rule = rule
	}

	s := strings.SplitN(event.GetRef(), "/", 3)
	branch := s[2]
	if buildCtx.Rule.Branch != "" && buildCtx.Rule.Branch != branch {
		log.Printf("Skip build because %s is not target branch", branch)
		return
	}

	client, err := NewKubernetesClient()
	if err != nil {
		errorLog(err)
		return
	}

	buildId := newBuildId()
	defer func() {
		if err := b.cleanup(client, buildId); err != nil {
			errorLog(err)
			return
		}
	}()

	err = b.buildRepository(buildCtx, client, buildId)
	if err != nil && err != errBuildFailure {
		errorLog(err)
		return
	}

	if buildCtx.Rule.PostProcess != nil {
		if err := b.postProcess(buildCtx, buildId); err != nil {
			errorLog(err)
			return
		}
	}
}

func (b *BazelBuild) cleanup(client *kubernetes.Clientset, buildId string) error {
	if b.debug {
		return nil
	}

	podList, err := client.CoreV1().Pods(b.Namespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", labelKeyJobId, buildId),
	})
	if err != nil {
		return xerrors.Errorf(": %v", err)
	}

	for _, v := range podList.Items {
		err := client.CoreV1().Pods(b.Namespace).Delete(v.Name, nil)
		if err != nil {
			return xerrors.Errorf(": %v", err)
		}
	}

	return nil
}

func (b *BazelBuild) buildRepository(buildCtx *eventContext, client *kubernetes.Clientset, buildId string) error {
	buildPod := b.buildPod(buildCtx, buildId)
	_, err := client.CoreV1().Pods(b.Namespace).Create(buildPod)
	if err != nil {
		return xerrors.Errorf(": %v", err)
	}
	success, err := WaitForFinish(client, b.Namespace, buildPod.Name)
	if err != nil {
		return xerrors.Errorf(": %v", err)
	}

	if !success {
		return errBuildFailure
	}

	return nil
}

func (b *BazelBuild) postProcess(buildCtx *eventContext, buildId string) error {
	artifactDir, err := b.downloadArtifact(buildCtx, buildId)
	if artifactDir != "" {
		defer os.RemoveAll(artifactDir)
	}
	if err != nil {
		return xerrors.Errorf(": %v", err)
	}

	s := strings.SplitN(buildCtx.Rule.PostProcess.Repo, "/", 2)
	r, err := newGitRepo(b.transport, s[0], s[1], buildCtx.Rule.PostProcess.Image, b.AuthorName, b.AuthorEmail)
	if err != nil {
		return xerrors.Errorf(": %v", err)
	}
	defer r.Close()

	artifactPath := filepath.Join(artifactDir, filepath.Base(buildCtx.Rule.Artifacts[0]))
	if err := r.UpdateKustomization(buildCtx, artifactPath, buildCtx.Rule.PostProcess.Paths); err != nil {
		return xerrors.Errorf(": %v", err)
	}

	return nil
}

func (b *BazelBuild) downloadArtifact(buildCtx *eventContext, buildId string) (string, error) {
	cfg := &aws.Config{
		Endpoint:         aws.String(b.StorageHost),
		Region:           aws.String("us-east-1"),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      credentials.NewEnvCredentials(),
	}
	sess := session.Must(session.NewSession(cfg))
	s3Client := s3manager.NewDownloaderWithClient(s3.New(sess))

	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		return "", xerrors.Errorf(": %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, err = s3Client.Download(tmpFile, &s3.GetObjectInput{
		Bucket: aws.String(b.ArtifactBucket),
		Key:    aws.String(fmt.Sprintf("%s-%s-%s.tar", buildCtx.Owner, buildCtx.Repo, buildId)),
	})
	if err != nil {
		return "", xerrors.Errorf(": %v", err)
	}

	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", xerrors.Errorf(": %v", err)
	}

	tmpFile.Seek(0, 0)
	t := tar.NewReader(tmpFile)
	for {
		hdr, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", xerrors.Errorf(": %v", err)
		}
		f, err := os.Create(filepath.Join(dir, hdr.Name))
		if err != nil {
			return "", xerrors.Errorf(": %v", err)
		}
		if _, err := io.Copy(f, t); err != nil {
			return "", xerrors.Errorf(": %v", err)
		}
	}

	return dir, nil
}

func (b *BazelBuild) buildPod(buildCtx *eventContext, buildId string) *corev1.Pod {
	mainImage := fmt.Sprintf("%s:%s", bazelImage, defaultBazelVersion)
	if buildCtx.Rule.BazelVersion != "" {
		mainImage = fmt.Sprintf("%s:%s", bazelImage, buildCtx.Rule.BazelVersion)
	}
	hostAliases := make([]corev1.HostAlias, 0)
	for _, v := range b.HostAliases {
		hostAliases = append(hostAliases, corev1.HostAlias{Hostnames: v.Hostnames, IP: v.IP})
	}

	env := make([]corev1.EnvVar, 0)
	for _, v := range buildCtx.Rule.Env {
		env = append(env, v.ToEnvVar())
	}

	volumes := []corev1.Volume{
		{
			Name: "workdir",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "outdir",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{Name: "workdir", MountPath: "/work"},
		{Name: "outdir", MountPath: "/out"},
	}

	if buildCtx.Rule.DockerConfigSecretName != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "docker-config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: buildCtx.Rule.DockerConfigSecretName,
				},
			}})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "docker-config",
			MountPath: "/home/bazel/.docker/config.json",
			SubPath:   ".dockerconfigjson"})
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%s", buildCtx.Owner, buildCtx.Repo, buildId),
			Namespace: b.Namespace,
			Labels: map[string]string{
				labelKeyJobId:  buildId,
				labelKeyCtrlBy: "bazel-build",
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: builderServiceAccount,
			RestartPolicy:      corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{
				{
					Name:  "pre-process",
					Image: buildSidecarImage,
					Args:  []string{"--action=clone", "--work-dir=/work", fmt.Sprintf("--url=%s", buildCtx.CloneURL())},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "workdir", MountPath: "/work"},
					},
				},
			},
			HostAliases: hostAliases,
			Containers: []corev1.Container{
				{
					Name:         "main",
					Image:        mainImage,
					Args:         []string{"--output_user_root=/out", "run", buildCtx.Rule.Target},
					WorkingDir:   "/work",
					Env:          append(env, corev1.EnvVar{Name: "DOCKER_CONFIG", Value: "/home/bazel/.docker"}),
					VolumeMounts: volumeMounts,
				},
				{
					Name:  "post-process",
					Image: buildSidecarImage,
					Args: []string{
						"--action=wait",
						fmt.Sprintf("--artifact-host=%s", b.StorageHost),
						fmt.Sprintf("--artifact-bucket=%s", b.ArtifactBucket),
						fmt.Sprintf("--artifact-path=%s", buildCtx.Rule.Artifacts[0]),
					},
					WorkingDir: "/work",
					Env: []corev1.EnvVar{
						{Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.name",
							},
						}},
						{Name: "POD_NAMESPACE", ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.namespace",
							},
						}},
						{Name: "AWS_ACCESS_KEY_ID", ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: b.StorageTokenSecretName,
								},
								Key: "accesskey",
							},
						}},
						{Name: "AWS_SECRET_ACCESS_KEY", ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: b.StorageTokenSecretName,
								},
								Key: "secretkey",
							},
						}},
						{Name: "JOB_NAME", Value: fmt.Sprintf("%s-%s", buildCtx.Owner, buildCtx.Repo)},
						{Name: "JOB_ID", Value: buildId},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "workdir", MountPath: "/work"},
						{Name: "outdir", MountPath: "/out"},
					},
				},
			},
			Volumes: volumes,
		},
	}
}

func newBuildId() string {
	buf := make([]byte, 8)

	rand.Seed(time.Now().UnixNano())
	for i := range buf {
		buf[i] = letters[rand.Intn(len(letters))]
	}

	return string(buf)
}

type gitRepo struct {
	dir         string
	owner       string
	repoName    string
	image       string
	authorName  string
	authorEmail string

	repo      *git.Repository
	transport *ghinstallation.Transport
}

func newGitRepo(transport *ghinstallation.Transport, owner, repo, image, authorName, authorEmail string) (*gitRepo, error) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, xerrors.Errorf(": %v", err)
	}

	t, err := transport.Token(context.Background())
	if err != nil {
		return nil, xerrors.Errorf(": %v", err)
	}
	u := fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)
	log.Printf("git clone %s", u)
	r, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:   u,
		Depth: 1,
		Auth:  &gogitHttp.BasicAuth{Username: "octocast", Password: t},
	})
	if err != nil {
		return nil, xerrors.Errorf(": %v", err)
	}

	log.Printf("New git repo: %s/%s in %s with image name: %s", owner, repo, dir, image)
	return &gitRepo{
		dir:         dir,
		owner:       owner,
		repoName:    repo,
		image:       image,
		authorName:  authorName,
		authorEmail: authorEmail,
		repo:        r,
		transport:   transport,
	}, nil
}

func (g *gitRepo) switchBranch() (string, *git.Worktree, error) {
	branchName := fmt.Sprintf("update-kustomization-%d", time.Now().Unix())

	masterRef, err := g.repo.Reference("refs/remotes/origin/master", true)
	if err != nil {
		return "", nil, err
	}

	ref := plumbing.NewHashReference(plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branchName)), masterRef.Hash())
	if err := g.repo.Storer.SetReference(ref); err != nil {
		return "", nil, err
	}

	tree, err := g.repo.Worktree()
	if err != nil {
		return "", nil, err
	}
	if err := tree.Checkout(&git.CheckoutOptions{Branch: ref.Name()}); err != nil {
		return "", nil, err
	}

	return branchName, tree, nil
}

func (g *gitRepo) commit(tree *git.Worktree, path string) error {
	if _, err := tree.Add(path); err != nil {
		return err
	}
	st, err := tree.Status()
	if err != nil {
		return err
	}
	if st.IsClean() {
		return errors.New("changeset is empty")
	}
	_, err = tree.Commit(fmt.Sprintf("Update %s", path), &git.CommitOptions{
		Author: &object.Signature{
			Name:  g.authorName,
			Email: g.authorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func (g *gitRepo) push(branchName string) error {
	token, err := g.transport.Token(context.Background())
	if err != nil {
		return xerrors.Errorf(": %v", err)
	}
	refSpec := fmt.Sprintf("refs/heads/%s:refs/heads/%s", branchName, branchName)
	log.Printf("git push origin %s with %s", refSpec, token)
	return g.repo.Push(&git.PushOptions{
		Auth:       &gogitHttp.BasicAuth{Username: "octocat", Password: token},
		RemoteName: "origin",
		RefSpecs:   []gitConfig.RefSpec{gitConfig.RefSpec(refSpec)},
	})
}

func (g *gitRepo) createPullRequest(buildCtx *eventContext, branch string, editedFiles []string) error {
	client := github.NewClient(&http.Client{Transport: g.transport})

	desc := "Change file(s):\n"
	for _, v := range editedFiles {
		desc += v + "\n"
	}
	_, _, err := client.PullRequests.Create(context.Background(), g.owner, g.repoName, &github.NewPullRequest{
		Title: github.String(fmt.Sprintf("Update %s", buildCtx.Repo)),
		Body:  github.String(desc),
		Base:  github.String("master"),
		Head:  github.String(branch),
	})

	return err
}

func (g *gitRepo) modifyKustomization(paths []string, newImageHash string) ([]string, error) {
	editFiles := make([]string, 0)
	for _, in := range paths {
		absPath := filepath.Join(g.dir, in)
		log.Printf("Read: %s", absPath)
		b, err := ioutil.ReadFile(absPath)
		if err != nil {
			return nil, xerrors.Errorf(": %v", err)
		}
		if len(b) == 0 {
			return nil, errors.New("file is empty")
		}

		lines := strings.Split(string(b), "\n")
		changed := false
		for i, v := range lines {
			if strings.Index(v, "digest") == -1 {
				continue
			}

			if strings.Index(v, "# bot:") > 0 {
				re := regexp.MustCompile(`digest:\s+(sha256:[a-zA-Z0-9]+)\s# bot:` + g.image)
				m := re.FindStringSubmatch(v)
				if len(m) == 0 {
					continue
				}
				changed = true
				lines[i] = strings.Replace(v, m[1], newImageHash, 1)
			}
		}

		if changed {
			editFiles = append(editFiles, in)
			if err := ioutil.WriteFile(absPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
				return nil, xerrors.Errorf(": %v", err)
			}
		}
	}

	return editFiles, nil
}

func (g *gitRepo) UpdateKustomization(buildCtx *eventContext, artifactPath string, paths []string) error {
	buf, err := ioutil.ReadFile(artifactPath)
	if err != nil {
		return xerrors.Errorf(": %v", err)
	}
	if !bytes.HasPrefix(buf, []byte("sha256:")) {
		return xerrors.New("artifact file does not contain an image hash")
	}
	newImageHash := strings.TrimSuffix(string(buf), "\n")

	branchName, tree, err := g.switchBranch()
	if err != nil {
		return err
	}

	editedFiles, err := g.modifyKustomization(paths, newImageHash)
	if err != nil {
		return xerrors.Errorf(": %v", err)
	}
	for _, v := range editedFiles {
		if err := g.commit(tree, v); err != nil {
			return xerrors.Errorf(": %v", err)
		}
	}

	if len(editedFiles) == 0 {
		log.Print("Skip creating a pull request because not have any change")
		return nil
	}

	if err := g.push(branchName); err != nil {
		return err
	}

	if err := g.createPullRequest(buildCtx, branchName, editedFiles); err != nil {
		return err
	}

	log.Print("Success create a pull request")
	return nil
}

func (g *gitRepo) Close() {
	os.RemoveAll(g.dir)
}
