package artifactupdater

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gogitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.f110.dev/xerrors"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
	"oras.land/oras-go/v2"
	orascontent "oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"

	"go.f110.dev/mono/go/build/api"
	"go.f110.dev/mono/go/build/model"
	"go.f110.dev/mono/go/logger"
)

const (
	trailerRule      = "Artifact-Rule"
	trailerBuildTask = "Build-Task"

	maxPushAttempts = 5
	ackWait         = 5 * time.Minute
	redeliveryDelay = time.Minute
)

var errNonFastForward = xerrors.Define("the push is not fast-forward")

// TokenSource provides the token to push to the managed repository.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// Registry fetches the artifacts from the OCI registry.
type Registry interface {
	// FetchFile returns the content of the file artifact whose manifest digest is digest in repository.
	FetchFile(ctx context.Context, repository, digest string) ([]byte, error)
}

// Updater updates the files in the repositories according to the rules of on_artifact_published when an artifact is
// published. The rules are looked up by the repository of the artifact with the API of Build.
type Updater struct {
	authorName  string
	authorEmail string
	tokenSource TokenSource
	buildClient api.APIClient
	registry    Registry
}

// NewUpdater returns an Updater which updates the default branch of the repositories.
// When tokenSource is nil, the repositories are accessed without the authentication.
func NewUpdater(authorName, authorEmail string, tokenSource TokenSource, buildClient api.APIClient, registry Registry) *Updater {
	return &Updater{
		authorName:  authorName,
		authorEmail: authorEmail,
		tokenSource: tokenSource,
		buildClient: buildClient,
		registry:    registry,
	}
}

// Run consumes the events from the subject "<streamName>.<subject>" with the durable consumer consumerName until ctx
// is canceled. An event is acknowledged after the update has been pushed.
func (u *Updater) Run(ctx context.Context, natsURL, streamName, subject, consumerName string) error {
	nc, err := nats.Connect(natsURL, nats.RetryOnFailedConnect(true), nats.PingInterval(30*time.Second))
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		return xerrors.WithStack(err)
	}
	consumer, err := js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Durable:       consumerName,
		FilterSubject: fmt.Sprintf("%s.%s", streamName, subject),
		DeliverPolicy: jetstream.DeliverNewPolicy,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       ackWait,
	})
	if err != nil {
		return xerrors.WithStack(err)
	}
	cc, err := consumer.Consume(func(msg jetstream.Msg) {
		ev := &model.ArtifactPublishedEvent{}
		if err := proto.Unmarshal(msg.Data(), ev); err != nil {
			logger.Log.Warn("Failed to unmarshal the event", logger.E(err))
			if err := msg.Term(); err != nil {
				logger.Log.Warn("Failed to terminate the message", logger.E(err))
			}
			return
		}
		if err := u.Update(ctx, ev); err != nil {
			logger.Log.Warn("Failed to update", logger.E(err), slog.Int("artifact_id", int(ev.GetArtifactId())),
				slog.Int("task_id", int(ev.GetTaskId())))
			if err := msg.NakWithDelay(redeliveryDelay); err != nil {
				logger.Log.Warn("Failed to nak the message", logger.E(err))
			}
			return
		}
		if err := msg.Ack(); err != nil {
			logger.Log.Warn("Failed to ack the message", logger.E(err))
		}
	})
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer cc.Stop()

	logger.Log.Info("Start consuming", slog.String("stream", streamName), slog.String("consumer", consumerName))
	<-ctx.Done()
	return nil
}

// Update applies the rules of on_artifact_published whose artifact is the repository of the artifact of ev, and pushes
// the change to each repository which defines the rules. When the push is rejected because it is not fast-forward,
// Update starts over from cloning the repository.
//
// The rules which have been applied by the newer or the same task are skipped. The task which applied the rule last
// time is recorded in the trailers of the commit. Nothing is committed if the files are not changed.
func (u *Updater) Update(ctx context.Context, ev *model.ArtifactPublishedEvent) error {
	res, err := u.buildClient.ListArtifacts(ctx, api.RequestListArtifacts_builder{TaskId: new(ev.GetTaskId())}.Build())
	if err != nil {
		return xerrors.WithStack(err)
	}
	var artifact *model.Artifact
	for _, v := range res.GetArtifacts() {
		if v.GetId() == ev.GetArtifactId() {
			artifact = v
			break
		}
	}
	if artifact == nil {
		return xerrors.Definef("artifact %d is not found in task %d", ev.GetArtifactId(), ev.GetTaskId()).WithStack()
	}

	rulesRes, err := u.buildClient.ListArtifactPublishedRules(ctx, api.RequestListArtifactPublishedRules_builder{Artifact: new(artifact.GetRepository())}.Build())
	if err != nil {
		return xerrors.WithStack(err)
	}
	var cloneURLs []string
	rulesByRepo := make(map[string][]*model.ArtifactPublishedRule)
	for _, rule := range rulesRes.GetRules() {
		cloneURL := rule.GetRepository().GetCloneUrl()
		if _, ok := rulesByRepo[cloneURL]; !ok {
			cloneURLs = append(cloneURLs, cloneURL)
		}
		rulesByRepo[cloneURL] = append(rulesByRepo[cloneURL], rule)
	}

	for _, cloneURL := range cloneURLs {
		var err error
		for range maxPushAttempts {
			err = u.update(ctx, cloneURL, rulesByRepo[cloneURL], artifact)
			if !errors.Is(err, errNonFastForward) {
				break
			}
			logger.Log.Info("Push was rejected. Retry", slog.String("repository", cloneURL))
		}
		if errors.Is(err, errNonFastForward) {
			return xerrors.Definef("failed to push to %s %d times because the push is not fast-forward", cloneURL, maxPushAttempts).WithStack()
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (u *Updater) update(ctx context.Context, cloneURL string, rules []*model.ArtifactPublishedRule, artifact *model.Artifact) error {
	var gitAuth transport.AuthMethod
	if u.tokenSource != nil {
		token, err := u.tokenSource.Token(ctx)
		if err != nil {
			return err
		}
		gitAuth = &gogitHttp.BasicAuth{Username: "x-access-token", Password: token}
	}
	dir, err := os.MkdirTemp("", "artifact-updater-")
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer os.RemoveAll(dir)
	repo, err := git.PlainCloneContext(ctx, dir, false, &git.CloneOptions{
		URL:          cloneURL,
		Auth:         gitAuth,
		SingleBranch: true,
	})
	if err != nil {
		return xerrors.WithStack(err)
	}

	lastTasks := make(map[string]int32)
	iter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		return xerrors.WithStack(err)
	}
	err = iter.ForEach(func(c *object.Commit) error {
		var appliedRules []string
		var taskID int64
		for _, line := range strings.Split(c.Message, "\n") {
			k, v, ok := strings.Cut(line, ": ")
			if !ok {
				continue
			}
			switch k {
			case trailerRule:
				appliedRules = append(appliedRules, v)
			case trailerBuildTask:
				taskID, _ = strconv.ParseInt(v, 10, 32)
			}
		}
		for _, v := range appliedRules {
			if _, ok := lastTasks[v]; !ok {
				lastTasks[v] = int32(taskID)
			}
		}
		for _, rule := range rules {
			if _, ok := lastTasks[rule.GetName()]; !ok {
				return nil
			}
		}
		return storer.ErrStop
	})
	iter.Close()
	if err != nil {
		return xerrors.WithStack(err)
	}

	tree, err := repo.Worktree()
	if err != nil {
		return xerrors.WithStack(err)
	}
	var applied []string
	for _, rule := range rules {
		if lastTask := lastTasks[rule.GetName()]; lastTask >= artifact.GetTaskId() {
			logger.Log.Info("Skip the rule which has been applied by the newer task", slog.String("rule", rule.GetName()),
				slog.Int("task_id", int(artifact.GetTaskId())), slog.Int("applied_task_id", int(lastTask)))
			continue
		}

		var path string
		var content []byte
		switch {
		case rule.HasReplace():
			if artifact.GetType() == model.ArtifactType_ARTIFACT_TYPE_CONTAINER_IMAGE {
				return xerrors.Definef("rule %s: replace can't be applied to the container image", rule.GetName()).WithStack()
			}
			path = rule.GetReplace().GetPath()
			content, err = u.registry.FetchFile(ctx, artifact.GetRepository(), artifact.GetHash())
			if err != nil {
				return err
			}
		case rule.HasKustomizeImage():
			if artifact.GetType() != model.ArtifactType_ARTIFACT_TYPE_CONTAINER_IMAGE {
				return xerrors.Definef("rule %s: kustomize_image can only be applied to the container image", rule.GetName()).WithStack()
			}
			path = rule.GetKustomizeImage().GetPath()
			buf, err := os.ReadFile(filepath.Join(dir, path))
			if err != nil {
				return xerrors.WithStack(err)
			}
			content, err = updateKustomizeImage(buf, rule.GetKustomizeImage().GetName(), artifact.GetHash())
			if err != nil {
				return xerrors.WithMessagef(err, "rule %s", rule.GetName())
			}
		default:
			return xerrors.Definef("rule %s has no action", rule.GetName()).WithStack()
		}

		current, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil && !os.IsNotExist(err) {
			return xerrors.WithStack(err)
		}
		if bytes.Equal(current, content) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0755); err != nil {
			return xerrors.WithStack(err)
		}
		if err := os.WriteFile(filepath.Join(dir, path), content, 0644); err != nil {
			return xerrors.WithStack(err)
		}
		if _, err := tree.Add(path); err != nil {
			return xerrors.WithStack(err)
		}
		applied = append(applied, rule.GetName())
	}
	if len(applied) == 0 {
		return nil
	}

	msg := new(strings.Builder)
	fmt.Fprintf(msg, "Update %s\n\n", artifact.GetRepository())
	for _, v := range applied {
		fmt.Fprintf(msg, "%s: %s\n", trailerRule, v)
	}
	fmt.Fprintf(msg, "%s: %d\n", trailerBuildTask, artifact.GetTaskId())
	_, err = tree.Commit(msg.String(), &git.CommitOptions{
		Author: &object.Signature{Name: u.authorName, Email: u.authorEmail, When: time.Now()},
	})
	if err != nil {
		return xerrors.WithStack(err)
	}
	err = repo.PushContext(ctx, &git.PushOptions{Auth: gitAuth})
	if err != nil && strings.Contains(err.Error(), "non-fast-forward") {
		return errNonFastForward
	}
	if err != nil {
		return xerrors.WithStack(err)
	}
	logger.Log.Info("Pushed", slog.String("repository", cloneURL), slog.Any("rules", applied),
		slog.Int("task_id", int(artifact.GetTaskId())))
	return nil
}

type orasRegistry struct {
	insecure bool
	store    credentials.Store
}

// NewORASRegistry returns a Registry which reads the credentials from the docker config. When insecure is true the
// registries are accessed over plain HTTP.
func NewORASRegistry(insecure bool) (Registry, error) {
	store, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return &orasRegistry{insecure: insecure, store: store}, nil
}

func (r *orasRegistry) FetchFile(ctx context.Context, repository, digest string) ([]byte, error) {
	repo, err := remote.NewRepository(repository)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	repo.PlainHTTP = r.insecure
	repo.Client = &auth.Client{Client: retry.DefaultClient, Cache: auth.NewCache(), Credential: credentials.Credential(r.store)}

	_, buf, err := oras.FetchBytes(ctx, repo, digest, oras.DefaultFetchBytesOptions)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	var manifest ocispec.Manifest
	if err := json.Unmarshal(buf, &manifest); err != nil {
		return nil, xerrors.WithStack(err)
	}
	if len(manifest.Layers) != 1 {
		return nil, xerrors.Definef("%s@%s has %d layers. The file artifact must have exactly one layer", repository, digest, len(manifest.Layers)).WithStack()
	}
	content, err := orascontent.FetchAll(ctx, repo, manifest.Layers[0])
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return content, nil
}

// updateKustomizeImage replaces the digest of the entry whose name is name in images of the kustomization buf.
// Only the value of the digest is replaced so that the rest of buf is kept as it is.
// It fails if there is no entry or the entry doesn't have the digest.
func updateKustomizeImage(buf []byte, name, digest string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(buf, &doc); err != nil {
		return nil, xerrors.WithStack(err)
	}
	if len(doc.Content) == 0 {
		return nil, xerrors.Define("the kustomization is empty").WithStack()
	}
	images := mappingValue(doc.Content[0], "images")
	if images == nil || images.Kind != yaml.SequenceNode {
		return nil, xerrors.Define("the kustomization doesn't have images").WithStack()
	}
	var digestNode *yaml.Node
	for _, image := range images.Content {
		if n := mappingValue(image, "name"); n != nil && n.Value == name {
			digestNode = mappingValue(image, "digest")
			if digestNode == nil {
				return nil, xerrors.Definef("the image %s doesn't have digest", name).WithStack()
			}
			break
		}
	}
	if digestNode == nil {
		return nil, xerrors.Definef("the image %s is not found in images", name).WithStack()
	}

	lines := bytes.SplitAfter(buf, []byte("\n"))
	line := lines[digestNode.Line-1]
	i := bytes.Index(line[digestNode.Column-1:], []byte(digestNode.Value))
	if i < 0 {
		return nil, xerrors.Definef("the digest of the image %s is not found at line %d", name, digestNode.Line).WithStack()
	}
	start := digestNode.Column - 1 + i
	lines[digestNode.Line-1] = append(append(append([]byte{}, line[:start]...), digest...), line[start+len(digestNode.Value):]...)
	return bytes.Join(lines, nil), nil
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
