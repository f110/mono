package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/go-github/v32/github"
	"go.f110.dev/protoc-ddl/probe"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/build/config"
	database2 "go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/logger"
)

const (
	AllowCommand           = "/allow-build"
	SkipCI                 = "[skip ci]"
	BuildConfigurationFile = "build.star"
	BazelVersionFile       = ".bazelversion"
)

type Builder interface {
	Build(ctx context.Context, repo *database2.SourceRepository, job *config.Job, revision, bazelVersion, command string, targets, platforms []string, via string) ([]*database2.Task, error)
}

type Api struct {
	*http.Server

	builder      Builder
	dao          dao.Options
	githubClient *github.Client
}

func NewApi(addr string, builder Builder, dao dao.Options, ghClient *github.Client) (*Api, error) {
	api := &Api{
		builder:      builder,
		dao:          dao,
		githubClient: ghClient,
	}
	mux := http.NewServeMux()
	mux.Handle("/favicon.ico", http.NotFoundHandler())
	mux.HandleFunc("/liveness", api.handleLiveness)
	mux.HandleFunc("/readiness", api.handleReadiness)
	mux.HandleFunc("/redo", api.handleRedo)
	mux.HandleFunc("/run", api.handleRun)
	mux.HandleFunc("/webhook", api.handleWebHook)
	s := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	api.Server = s

	return api, nil
}

func (a *Api) handleWebHook(w http.ResponseWriter, req *http.Request) {
	// Skip validate payload. Because validating body was done by the upstream proxy.
	payload, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Log.Warn("Failed read body", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	messageType := github.WebHookType(req)
	event, err := github.ParseWebHook(messageType, payload)
	if err != nil {
		logger.Log.Warn("Failed parse webhook's payload", zap.Error(err))
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
					&github.IssueComment{Body: github.String(body)},
				)
				if err != nil {
					logger.Log.Warn("Failed create the comment", zap.Error(err), zap.String("repo", event.Repo.GetFullName()), zap.Int("number", event.PullRequest.GetNumber()))
					w.WriteHeader(http.StatusInternalServerError)
				}
				return
			} else {
				if ok, err := a.skipCI(req.Context(), event); ok || err != nil {
					logger.Log.Info("Skip build", zap.String("repo", event.Repo.GetFullName()), zap.Int("number", event.PullRequest.GetNumber()))
					return
				}
				repo := a.findRepository(req.Context(), event.Repo.GetHTMLURL())
				if err := a.buildByPullRequest(req.Context(), repo, event); err != nil {
					logger.Log.Warn("Failed build the pull request", zap.Error(err))
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
					logger.Log.Warn("Failed build the pull request", zap.Error(err), zap.String("repo", event.Repo.GetFullName()), zap.Int("number", event.PullRequest.GetNumber()))
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
				logger.Log.Warn("Failed delete PermitPullRequest", zap.Error(err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	case *github.IssueCommentEvent:
		if err := a.issueComment(req.Context(), event); err != nil {
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

func (a *Api) fetchBuildConfig(ctx context.Context, owner, repoName, revision string, isMainBranch bool) (*config.Config, string, error) {
	// Find the configuration file
	var commitSHA string
	if isMainBranch {
		// Read configuration file of the revision
		commit, _, err := a.githubClient.Git.GetCommit(ctx, owner, repoName, revision)
		if err != nil {
			return nil, "", xerrors.WithMessagef(err, "failed to get the commit: %s", revision)
		}
		commitSHA = commit.GetSHA()
	} else {
		// If the revision doesn't belong to the main branch, the build configuration will be read from the main branch.
		commit, _, err := a.githubClient.Git.GetCommit(ctx, owner, repoName, "HEAD")
		if err != nil {
			return nil, "", xerrors.WithMessage(err, "failed to get HEAD commit")
		}
		commitSHA = commit.GetSHA()
	}
	tree, _, err := a.githubClient.Git.GetTree(ctx, owner, repoName, commitSHA, false)
	if err != nil {
		return nil, "", xerrors.WithMessagef(err, "failed to get the tree: %s", commitSHA)
	}
	var blobSHA, versionBlobSHA string
	for _, e := range tree.Entries {
		switch e.GetPath() {
		case BuildConfigurationFile:
			blobSHA = e.GetSHA()
		case BazelVersionFile:
			versionBlobSHA = e.GetSHA()
		}
	}
	if blobSHA == "" {
		logger.Log.Debug("build configuration file is not found", zap.String("repo", repoName), zap.String("revision", commitSHA))
		return nil, "", nil
	}
	buildConfFileBlob, _, err := a.githubClient.Git.GetBlobRaw(ctx, owner, repoName, blobSHA)
	if err != nil {
		logger.Log.Info("Skip build", logger.Error(err), zap.String("owner", owner), zap.String("repo", repoName), zap.String("revision", revision))
		return nil, "", nil
	}
	var bazelVersion string
	if versionBlobSHA != "" {
		if blob, _, err := a.githubClient.Git.GetBlobRaw(ctx, owner, repoName, versionBlobSHA); err == nil {
			bazelVersion = strings.TrimRight(string(blob), "\n")
		} else {
			logger.Log.Info("Failed to get the blob of .bazelversion", zap.Error(err))
		}
	}

	// Parse the configuration file
	conf, err := config.Read(bytes.NewReader(buildConfFileBlob), owner, repoName)
	if err != nil {
		return nil, "", err
	}
	if len(conf.Jobs) == 0 {
		logger.Log.Info("Skip build because there is no job", zap.String("owner", owner), zap.String("repo", repoName), zap.String("revision", revision))
		return nil, "", nil
	}

	return conf, bazelVersion, nil
}

func (a *Api) findRepository(ctx context.Context, repoURL string) *database2.SourceRepository {
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

func (a *Api) buildByPushEvent(ctx context.Context, repo *database2.SourceRepository, event *github.PushEvent, isMainBranch bool) error {
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

	if err := a.build(ctx, owner, repoName, repo, jobs, bazelVersion, revision, "push"); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (a *Api) buildByPullRequest(ctx context.Context, repo *database2.SourceRepository, event *github.PullRequestEvent) error {
	owner, repoName, revision := event.Repo.Owner.GetLogin(), event.Repo.GetName(), event.PullRequest.Head.GetSHA()
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

	if err := a.build(ctx, owner, repoName, repo, jobs, bazelVersion, revision, "pull_request"); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

func (a *Api) buildByRelease(ctx context.Context, repo *database2.SourceRepository, event *github.ReleaseEvent) error {
	ref, _, err := a.githubClient.Git.GetRef(ctx, event.Repo.Owner.GetLogin(), event.Repo.GetName(), fmt.Sprintf("tags/%s", event.Release.GetTagName()))
	if err != nil {
		return xerrors.WithStack(err)
	}

	owner, repoName, revision := event.Repo.Owner.GetLogin(), event.Repo.GetName(), ref.Object.GetSHA()
	conf, bazelVersion, err := a.fetchBuildConfig(ctx, owner, repoName, revision, false)
	if err != nil {
		logger.Log.Info("Skip build", logger.Error(err), zap.String("owner", owner), zap.String("repo", repoName), zap.String("revision", revision))
		return nil
	}
	if conf == nil {
		logger.Log.Info("Skip build because build file is not found", zap.String("owner", owner), zap.String("repo", repoName))
		return nil
	}
	jobs := conf.Job(config.EventRelease)

	if err := a.build(ctx, owner, repoName, repo, jobs, bazelVersion, revision, "release"); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (a *Api) buildPullRequest(ctx context.Context, repo *database2.SourceRepository, owner, repoName string, number int) error {
	pr, res, err := a.githubClient.PullRequests.Get(ctx, owner, repoName, number)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if res.StatusCode != http.StatusOK {
		return xerrors.New("could not get pr")
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

	if err := a.build(ctx, owner, repoName, repo, jobs, bazelVersion, revision, "pr"); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (a *Api) build(ctx context.Context, owner, repoName string, repo *database2.SourceRepository, jobs []*config.Job, bazelVersion, revision, via string) error {
	for _, v := range jobs {
		// Trigger the job when Command is build or test only.
		// In other words, If command is run, we are not trigger the job via PushEvent.
		switch v.Command {
		case "build", "test":
		default:
			logger.Log.Debug("Skip creating job", zap.String("command", v.Command))
			continue
		}

		if _, err := a.builder.Build(ctx, repo, v, revision, bazelVersion, v.Command, v.Targets, v.Platforms, via); err != nil {
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

			_, err = a.dao.PermitPullRequest.Create(ctx, &database2.PermitPullRequest{
				Repository: event.Repo.GetFullName(),
				Number:     int32(event.Issue.GetNumber()),
			})
			if err != nil {
				return xerrors.WithStack(err)
			}

			body := "Understood. This pull request added to allow list.\n" +
				"We are going to build the job."
			_, _, err = a.githubClient.Issues.CreateComment(
				ctx,
				event.Repo.GetOwner().GetLogin(),
				event.Repo.GetName(),
				event.Issue.GetNumber(),
				&github.IssueComment{Body: github.String(body)},
			)
			if err != nil {
				return xerrors.WithStack(err)
			}

			repo := a.findRepository(ctx, event.Repo.GetHTMLURL())
			if err := a.buildPullRequest(ctx, repo, event.Repo.Owner.GetLogin(), event.Repo.GetName(), event.Issue.GetNumber()); err != nil {
				return xerrors.WithStack(err)
			}
		}
	}
	return nil
}

type RunResponse struct {
	TaskId int32 `json:"task_id"`
}

func (a *Api) handleRedo(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := req.ParseForm(); err != nil {
		logger.Log.Info("Failed parse form", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	taskId, err := strconv.Atoi(req.FormValue("task_id"))
	if err != nil {
		logger.Log.Info("Failed parse task id", zap.Error(err), zap.String("task_id", req.FormValue("task_id")))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	task, err := a.dao.Task.Select(req.Context(), int32(taskId))
	if err != nil {
		logger.Log.Info("Task is not found", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	jobConfiguration := &config.Job{}
	if err := json.Unmarshal([]byte(task.JobConfiguration), jobConfiguration); err != nil {
		return
	}
	newTasks, err := a.builder.Build(
		req.Context(),
		task.Repository,
		jobConfiguration,
		task.Revision,
		task.BazelVersion,
		task.Command,
		jobConfiguration.Targets,
		jobConfiguration.Platforms,
		"api",
	)
	if err != nil {
		logger.Log.Warn("Failed build job", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	logger.Log.Info("Success enqueue redo-job", zap.Int32("task_id", task.Id), zap.Int32("new_task_id", newTasks[len(newTasks)-1].Id))
	if err := json.NewEncoder(w).Encode(RunResponse{TaskId: newTasks[len(newTasks)-1].Id}); err != nil {
		logger.Log.Warn("Failed encode response", zap.Error(err))
	}
}

func (a *Api) handleRun(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := req.ParseForm(); err != nil {
		logger.Log.Info("Failed to parse form", logger.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	repositoryId, err := strconv.Atoi(req.FormValue("repository_id"))
	if err != nil {
		logger.Log.Info("Failed to parse repository id", logger.Error(err), zap.String("repository_id", req.FormValue("repository_id")))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	repo, err := a.dao.Repository.Select(req.Context(), int32(repositoryId))
	if err != nil {
		logger.Log.Info("Failed to get the repository", logger.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	u, err := url.Parse(repo.Url)
	if err != nil {
		logger.Log.Info("Failed to parse repository URL", logger.Error(err), zap.String("url", repo.Url))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if u.Hostname() != "github.com" {
		logger.Log.Info("The repository is not hosted github.com")
		return
	}
	// u.Path is /owner/repo if URL is github.com.
	s := strings.Split(u.Path, "/")
	owner, repoName := s[1], s[2]
	githubRepo, _, err := a.githubClient.Repositories.Get(req.Context(), owner, repoName)
	if err != nil {
		logger.Log.Info("Failed to get the information of repository from github", logger.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	branch, _, err := a.githubClient.Repositories.GetBranch(req.Context(), owner, repoName, githubRepo.GetDefaultBranch())
	if err != nil {
		logger.Log.Info("Failed to branch", logger.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	conf, bazelVersion, err := a.fetchBuildConfig(req.Context(), owner, repoName, branch.Commit.GetSHA(), true)
	if err != nil {
		logger.Log.Info("Skip build", logger.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var job *config.Job
	for _, v := range conf.Jobs {
		if v.Name == req.FormValue("job_name") {
			job = v
			break
		}
	}
	if job == nil {
		logger.Log.Info("The job is not found", zap.String("job_name", req.FormValue("job_name")))
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if !enumerable.IsInclude(job.Event, "manual") {
		logger.Log.Info("The job is not intended to trigger manually", zap.String("job_name", job.Name))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	newTasks, err := a.builder.Build(
		req.Context(),
		repo,
		job,
		githubRepo.GetDefaultBranch(),
		bazelVersion,
		job.Command,
		job.Targets,
		job.Platforms,
		"manual",
	)
	if err != nil {
		logger.Log.Warn("Failed to start building job", logger.Error(err))
		return
	}

	if err := json.NewEncoder(w).Encode(RunResponse{TaskId: newTasks[len(newTasks)-1].Id}); err != nil {
		logger.Log.Warn("Failed to encode the response", logger.Error(err))
	}

}

func (a *Api) handleReadiness(w http.ResponseWriter, req *http.Request) {
	p := probe.NewProbe(a.dao.RawConnection)
	if !p.Ready(req.Context(), database2.SchemaHash) {
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
