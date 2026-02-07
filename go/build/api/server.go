package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v73/github"
	"go.f110.dev/protoc-ddl/probe"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
	"go.f110.dev/mono/go/varptr"
)

const (
	AllowCommand           = "/allow-build"
	SkipCI                 = "[skip ci]"
	BuildConfigurationFile = "build.star"
	BazelVersionFile       = ".bazelversion"
)

type Builder interface {
	Build(ctx context.Context, repo *database.SourceRepository, job *config.JobV2, revision, bazelVersion, command string, targets, platforms []string, via string, isMainBranch bool) ([]*database.Task, error)
	ForceStop(ctx context.Context, taskId int32) error
}

type Api struct {
	*http.Server

	builder           Builder
	dao               dao.Options
	githubClient      *github.Client
	stClient          *storage.S3
	bazelMirrorPrefix string
}

func NewApi(addr string, builder Builder, dao dao.Options, ghClient *github.Client, stClient *storage.S3, bazelMirrorPrefix string) (*Api, error) {
	api := &Api{
		builder:           builder,
		dao:               dao,
		githubClient:      ghClient,
		stClient:          stClient,
		bazelMirrorPrefix: bazelMirrorPrefix,
	}
	mux := http.NewServeMux()
	mux.Handle("/favicon.ico", http.NotFoundHandler())
	mux.HandleFunc("/liveness", api.handleLiveness)
	mux.HandleFunc("/readiness", api.handleReadiness)
	mux.HandleFunc("/webhook", api.handleWebHook)

	bs := newAPIService(builder, dao, ghClient)
	grpcServer := grpc.NewServer()
	RegisterAPIServer(grpcServer, bs)
	s := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.ProtoMajor == 2 && strings.HasPrefix(
				req.Header.Get("Content-Type"), "application/grpc") {
				grpcServer.ServeHTTP(w, req)
			} else {
				mux.ServeHTTP(w, req)
			}
		}),
		Protocols: new(http.Protocols),
	}
	s.Protocols.SetHTTP1(true)
	s.Protocols.SetHTTP2(true)
	s.Protocols.SetUnencryptedHTTP2(true)
	api.Server = s

	return api, nil
}

func (a *Api) handleWebHook(w http.ResponseWriter, req *http.Request) {
	// Skip validate payload. Because validating body was done by the upstream proxy.
	payload, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Log.Warn("Failed read body", logger.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	messageType := github.WebHookType(req)
	event, err := github.ParseWebHook(messageType, payload)
	if err != nil {
		logger.Log.Warn("Failed parse webhook's payload", logger.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch event := event.(type) {
	case *github.PushEvent:
		repo := a.findRepository(req.Context(), event.Repo.GetHTMLURL())

		if isMainBranch(event.GetRef(), event.Repo.GetMasterBranch()) {
			if ok, err := a.skipCI(req.Context(), event); ok || err != nil {
				logger.Log.Info("Skip build", zap.String("repo", event.Repo.GetFullName()), zap.String("commit", event.GetHead()))
				return
			}
			if err := a.buildByPushEvent(req.Context(), repo, event, true); err != nil {
				logger.Log.Warn("Failed to build", zap.String("repo", repo.Name), logger.Error(err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
		// Currently, we don't need to build other branch when got PushEvent.
	case *github.PullRequestEvent:
		switch event.GetAction() {
		case "opened":
			if ok, err := a.allowPullRequest(req.Context(), event); err != nil {
				logger.Log.Info("Failed check the build permission", zap.String("repo", event.Repo.GetFullName()), zap.Int("number", event.PullRequest.GetNumber()))
				return
			} else if !ok {
				body := "Sorry, We could not build this pull request. Because building this pull request is not allowed due to security reason.\n\n" +
					"For author, Thank you for your contribution. We appreciate your work. Please wait for permitting to build this pull request by administrator.\n" +
					"For administrator, If you are going to allow this pull request, please comment `" + AllowCommand + "`."
				_, _, err := a.githubClient.Issues.CreateComment(
					req.Context(),
					event.Repo.GetOwner().GetLogin(),
					event.Repo.GetName(),
					event.PullRequest.GetNumber(),
					&github.IssueComment{Body: varptr.Ptr(body)},
				)
				if err != nil {
					logger.Log.Warn("Failed create the comment", logger.Error(err), zap.String("repo", event.Repo.GetFullName()), zap.Int("number", event.PullRequest.GetNumber()))
					w.WriteHeader(http.StatusInternalServerError)
				}
				return
			} else {
				if ok, err := a.skipCI(req.Context(), event); ok || err != nil {
					logger.Log.Info("Skip build", zap.String("repo", event.Repo.GetFullName()), zap.Int("number", event.PullRequest.GetNumber()), logger.Error(err), logger.StackTrace(err))
					return
				}
				repo := a.findRepository(req.Context(), event.Repo.GetHTMLURL())
				if err := a.buildByPullRequest(req.Context(), repo, event); err != nil {
					logger.Log.Warn("Failed build the pull request", logger.Error(err))
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
		case "synchronize":
			if ok, _ := a.allowPullRequest(req.Context(), event); ok {
				if ok, err := a.skipCI(req.Context(), event); ok || err != nil {
					logger.Log.Info("Skip build", zap.String("repo", event.Repo.GetFullName()), zap.Int("number", event.PullRequest.GetNumber()))
					return
				}
				repo := a.findRepository(req.Context(), event.Repo.GetHTMLURL())
				if err := a.buildByPullRequest(req.Context(), repo, event); err != nil {
					logger.Log.Warn("Failed build the pull request", logger.Error(err), zap.String("repo", event.Repo.GetFullName()), zap.Int("number", event.PullRequest.GetNumber()))
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
		case "closed":
			permits, err := a.dao.PermitPullRequest.ListByRepositoryAndNumber(req.Context(), event.Repo.GetFullName(), int32(event.PullRequest.GetNumber()))
			if err != nil {
				return
			}
			if len(permits) == 0 {
				return
			}
			permit := permits[0]
			if err := a.dao.PermitPullRequest.Delete(req.Context(), permit.Id); err != nil {
				logger.Log.Warn("Failed delete PermitPullRequest", logger.Error(err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	case *github.IssueCommentEvent:
		if err := a.issueComment(req.Context(), event); err != nil {
			logger.Log.Warn("Failed handle comment", logger.Error(err), logger.StackTrace(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case *github.ReleaseEvent:
		switch event.GetAction() {
		case "published":
			repo := a.findRepository(req.Context(), event.Repo.GetHTMLURL())

			if err := a.buildByRelease(req.Context(), repo, event); err != nil {
				logger.Log.Warn("Failed to build", zap.String("repo", repo.Name), logger.Error(err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	}
}

func (a *Api) fetchBuildConfig(ctx context.Context, owner, repoName, revision string, useCommittedConfig bool) (*config.Config, string, error) {
	// Find the configuration file
	var commit *github.RepositoryCommit
	if useCommittedConfig {
		// Read configuration file of the revision
		c, _, err := a.githubClient.Repositories.GetCommit(ctx, owner, repoName, revision, nil)
		if err != nil {
			return nil, "", xerrors.WithMessagef(err, "failed to get the commit: %s", revision)
		}
		commit = c
	} else {
		// If the revision doesn't belong to the main branch, the build configuration will be read from the main branch.
		c, _, err := a.githubClient.Repositories.GetCommit(ctx, owner, repoName, "HEAD", nil)
		if err != nil {
			return nil, "", xerrors.WithMessage(err, "failed to get HEAD commit")
		}
		commit = c
	}

	// Parse the configuration file
	conf, err := config.ReadFromSpecifiedCommit(ctx, a.githubClient, owner, repoName, commit.GetCommit().GetTree().GetSHA())
	if err != nil {
		return nil, "", err
	}
	if len(conf.Jobs) == 0 {
		logger.Log.Info("Skip build because there is no job", zap.String("owner", owner), zap.String("repo", repoName), zap.String("revision", revision))
		return nil, "", nil
	}

	return conf, conf.BazelVersion, nil
}

func (a *Api) findRepository(ctx context.Context, repoURL string) *database.SourceRepository {
	repos, err := a.dao.Repository.ListByUrl(ctx, repoURL)
	if err != nil {
		logger.Log.Warn("Could not find repository", zap.Error(err))
		return nil
	}
	if len(repos) != 1 {
		logger.Log.Warn("Can not decide the repository by url", zap.String("url", repoURL))
		return nil
	}
	return repos[0]
}

func (a *Api) allowPullRequest(ctx context.Context, event *github.PullRequestEvent) (bool, error) {
	if event.Repo.Owner.GetLogin() == event.Sender.GetLogin() {
		logger.Log.Info("The sender of PushRequestEvent is the repository owner", zap.String("owner", event.Repo.Owner.GetLogin()), zap.String("sender", event.Sender.GetLogin()))
		return true, nil
	}

	users, err := a.dao.TrustedUser.ListByGithubId(ctx, event.Sender.GetID())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Log.Warn("Could not get trusted user", zap.Error(err), zap.Int64("sender.id", event.Sender.GetID()))
		return false, err
	}
	if users != nil && len(users) == 1 {
		return true, nil
	}

	permitPullRequest, err := a.dao.PermitPullRequest.ListByRepositoryAndNumber(ctx, event.Repo.GetFullName(), int32(event.PullRequest.GetNumber()))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Log.Warn("Could not get permit pull request", zap.Error(err), zap.String("repo", event.Repo.GetFullName()), zap.Int("number", event.PullRequest.GetNumber()))
		return false, err
	}
	if permitPullRequest != nil {
		return true, nil
	}

	return false, nil
}

func (a *Api) skipCI(_ context.Context, e interface{}) (bool, error) {
	switch event := e.(type) {
	case *github.PushEvent:
		if strings.HasPrefix(event.HeadCommit.GetMessage(), SkipCI) {
			return true, nil
		}
	case *github.PullRequestEvent:
		if strings.HasPrefix(event.PullRequest.GetTitle(), SkipCI) {
			return true, nil
		}
	}

	return false, nil
}

func (a *Api) buildByPushEvent(ctx context.Context, repo *database.SourceRepository, event *github.PushEvent, isMainBranch bool) error {
	owner, repoName, revision := event.Repo.Owner.GetLogin(), event.Repo.GetName(), event.HeadCommit.GetID()
	conf, bazelVersion, err := a.fetchBuildConfig(ctx, owner, repoName, revision, isMainBranch)
	if err != nil {
		logger.Log.Info("Skip build", logger.Error(err), zap.String("owner", owner), zap.String("repo", repoName), zap.String("revision", revision))
		return nil
	}
	if conf == nil {
		logger.Log.Info("Skip build because build file is not found", zap.String("owner", owner), zap.String("repo", repoName))
		return nil
	}
	jobs := conf.Job(config.EventPush)

	if err := a.build(ctx, owner, repoName, repo, jobs, bazelVersion, revision, "push", isMainBranch); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (a *Api) buildByPullRequest(ctx context.Context, repo *database.SourceRepository, event *github.PullRequestEvent) error {
	owner, repoName, revision := event.Repo.Owner.GetLogin(), event.Repo.GetName(), event.PullRequest.Head.GetSHA()
	conf, bazelVersion, err := a.fetchBuildConfig(ctx, owner, repoName, revision, false)
	if err != nil {
		logger.Log.Info("Skip build", logger.Error(err), logger.StackTrace(err), zap.String("owner", owner), zap.String("repo", repoName), zap.String("revision", revision))
		return nil
	}
	if conf == nil {
		logger.Log.Info("Skip build because build file is not found", zap.String("owner", owner), zap.String("repo", repoName))
		return nil
	}
	jobs := conf.Job(config.EventPullRequest)

	if err := a.build(ctx, owner, repoName, repo, jobs, bazelVersion, revision, "pull_request", false); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (a *Api) buildByRelease(ctx context.Context, repo *database.SourceRepository, event *github.ReleaseEvent) error {
	ref, _, err := a.githubClient.Git.GetRef(ctx, event.Repo.Owner.GetLogin(), event.Repo.GetName(), fmt.Sprintf("tags/%s", event.Release.GetTagName()))
	if err != nil {
		return xerrors.WithStack(err)
	}
	a.githubClient.Git.GetTag(ctx, event.Repo.Owner.GetLogin(), event.Repo.GetName(), event.Release.GetTagName())

	owner, repoName, revision := event.Repo.Owner.GetLogin(), event.Repo.GetName(), ref.Object.GetSHA()
	conf, bazelVersion, err := a.fetchBuildConfig(ctx, owner, repoName, revision, true)
	if err != nil {
		logger.Log.Info("Skip build", logger.Error(err), zap.String("owner", owner), zap.String("repo", repoName), zap.String("revision", revision))
		return nil
	}
	if conf == nil {
		logger.Log.Info("Skip build because build file is not found", zap.String("owner", owner), zap.String("repo", repoName))
		return nil
	}
	jobs := conf.Job(config.EventRelease)

	if err := a.build(ctx, owner, repoName, repo, jobs, bazelVersion, revision, "release", false); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (a *Api) buildPullRequest(ctx context.Context, repo *database.SourceRepository, owner, repoName string, number int) error {
	pr, res, err := a.githubClient.PullRequests.Get(ctx, owner, repoName, number)
	if err != nil {
		log.Println(1)
		return xerrors.WithStack(err)
	}
	if res.StatusCode != http.StatusOK {
		return xerrors.Define("could not get pr").WithStack()
	}
	revision := pr.GetHead().GetSHA()
	conf, bazelVersion, err := a.fetchBuildConfig(ctx, owner, repoName, revision, false)
	if err != nil {
		logger.Log.Info("Skip build", logger.Error(err), zap.String("owner", owner), zap.String("repo", repoName), zap.String("revision", revision))
		return nil
	}
	if conf == nil {
		logger.Log.Info("Skip build because build file is not found", zap.String("owner", owner), zap.String("repo", repoName))
		return nil
	}
	jobs := conf.Job(config.EventPullRequest)

	if err := a.build(ctx, owner, repoName, repo, jobs, bazelVersion, revision, "pr", false); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (a *Api) build(ctx context.Context, owner, repoName string, repo *database.SourceRepository, jobs []*config.JobV2, bazelVersion, revision, via string, isMainBranch bool) error {
	for _, v := range jobs {
		// Trigger the job when Command is build or test only.
		// In other words, If command is run, we are not trigger the job via PushEvent.
		switch v.Command {
		case "build", "test", "run":
		default:
			logger.Log.Warn("Skip creating job", zap.String("command", v.Command))
			continue
		}

		if _, err := a.builder.Build(ctx, repo, v, revision, bazelVersion, v.Command, v.Targets, v.Platforms, via, isMainBranch); err != nil {
			logger.Log.Warn("Failed start job", zap.Error(err), zap.String("owner", owner), zap.String("repo", repoName))
			return xerrors.WithStack(err)
		}
	}

	logger.Log.Debug("Successfully create build task", zap.String("repo", repo.Name), zap.String("revision", revision))
	return nil
}

func (a *Api) issueComment(ctx context.Context, event *github.IssueCommentEvent) error {
	switch event.GetAction() {
	case "created":
		if strings.Contains(event.Comment.GetBody(), AllowCommand) {
			users, err := a.dao.TrustedUser.ListByGithubId(ctx, event.Sender.GetID())
			if err != nil {
				log.Println(2)
				return xerrors.WithStack(err)
			}
			if len(users) != 1 {
				return nil
			}
			user := users[0]
			if user == nil {
				logger.Log.Info("Skip handling comment due to user is not trusted user", zap.String("user", event.Sender.GetLogin()))
				return nil
			}

			_, err = a.dao.PermitPullRequest.Create(ctx, &database.PermitPullRequest{
				Repository: event.Repo.GetFullName(),
				Number:     int32(event.Issue.GetNumber()),
			})
			if err != nil {
				log.Print("1")
				return xerrors.WithStack(err)
			}

			body := "Understood. This pull request added to allow list.\n" +
				"We are going to build the job."
			_, _, err = a.githubClient.Issues.CreateComment(
				ctx,
				event.Repo.GetOwner().GetLogin(),
				event.Repo.GetName(),
				event.Issue.GetNumber(),
				&github.IssueComment{Body: varptr.Ptr(body)},
			)
			if err != nil {
				return xerrors.WithStack(err)
			}

			repo := a.findRepository(ctx, event.Repo.GetHTMLURL())
			if err := a.buildPullRequest(ctx, repo, event.Repo.Owner.GetLogin(), event.Repo.GetName(), event.Issue.GetNumber()); err != nil {
				return err
			}
		}
	}
	return nil
}

type ReadinessResponse struct {
	Versions []string `json:"versions"`
}

func (a *Api) handleReadiness(w http.ResponseWriter, req *http.Request) {
	p := probe.NewProbe(a.dao.RawConnection)
	if !p.Ready(req.Context(), database.SchemaHash) {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	objs, err := a.stClient.List(req.Context(), a.bazelMirrorPrefix)
	if err != nil {
		logger.Log.Error("Failed to get the list of the file from the object storage", zap.Error(err), zap.String("prefix", a.bazelMirrorPrefix))
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	var versions semver.Collection
	for _, v := range objs {
		name := filepath.Base(v.Name)
		if !strings.HasPrefix(name, "bazel-") {
			continue
		}
		ver := name[6:]
		ver = ver[:strings.Index(ver, "-")]
		if v, err := semver.NewVersion(ver); err != nil {
			continue
		} else {
			versions = append(versions, v)
		}
	}
	versions = enumerable.Uniq(versions, func(t *semver.Version) string { return t.String() })
	sort.Sort(versions)

	res := &ReadinessResponse{Versions: enumerable.Map(versions, func(t *semver.Version) string { return t.String() })}
	if err := json.NewEncoder(w).Encode(res); err != nil {
		logger.Log.Error("Failed to encode to json", zap.Error(err))
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
}

func (*Api) handleLiveness(_ http.ResponseWriter, _ *http.Request) {}

func isMainBranch(ref, masterBranch string) bool {
	b := strings.SplitN(ref, "/", 3)
	if len(b) < 3 {
		return false
	}
	branch := b[2]

	return branch == masterBranch
}
