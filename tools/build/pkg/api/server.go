package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/go-github/v32/github"
	"go.f110.dev/protoc-ddl/probe"
	"go.uber.org/zap"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/lib/logger"
	"go.f110.dev/mono/tools/build/pkg/database"
	"go.f110.dev/mono/tools/build/pkg/database/dao"
	"go.f110.dev/mono/tools/build/pkg/discovery"
	"go.f110.dev/mono/tools/build/pkg/job"
)

const (
	AllowCommand = "/allow-build"
	SkipCI       = "[skip ci]"
)

type Builder interface {
	Build(ctx context.Context, job *database.Job, revision, command, target, via string) (*database.Task, error)
}

type Api struct {
	*http.Server

	builder      Builder
	discovery    *discovery.Discover
	dao          dao.Options
	githubClient *github.Client
}

func NewApi(addr string, builder Builder, discovery *discovery.Discover, dao dao.Options, ghClient *github.Client) (*Api, error) {
	api := &Api{
		builder:      builder,
		discovery:    discovery,
		dao:          dao,
		githubClient: ghClient,
	}
	mux := http.NewServeMux()
	mux.Handle("/favicon.ico", http.NotFoundHandler())
	mux.HandleFunc("/run", api.handleRun)
	mux.HandleFunc("/liveness", api.handleLiveness)
	mux.HandleFunc("/readiness", api.handleReadiness)
	mux.HandleFunc("/discovery", api.handleDiscovery)
	mux.HandleFunc("/redo", api.handleRedo)
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
	payload, err := ioutil.ReadAll(req.Body)
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
		if modifiedRuleFile(event) {
			repos, err := a.dao.Repository.ListByUrl(req.Context(), event.Repo.GetHTMLURL())
			if err != nil {
				logger.Log.Warn("Could not find repository", zap.Error(err))
				return
			}
			if len(repos) != 1 {
				logger.Log.Warn("Can not decide the repository by url", zap.String("url", event.Repo.GetHTMLURL()))
				return
			}
			repo := repos[0]

			// If push event is on main branch, Set a revision to discovery job.
			// This is intended to rebuild all jobs after discovering.
			rev := ""
			if isMainBranch(event.GetRef(), event.Repo.GetMasterBranch()) {
				rev = event.GetAfter()
			}
			if err := a.discovery.FindOut(repo, rev); err != nil {
				logger.Log.Warn("Could not start discovery job", zap.Error(err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			return
		}

		if isMainBranch(event.GetRef(), event.Repo.GetMasterBranch()) {
			if ok, err := a.skipCI(req.Context(), event); ok || err != nil {
				logger.Log.Info("Skip build", zap.String("repo", event.Repo.GetFullName()), zap.String("commit", event.GetHead()))
				return
			}
			if err := a.buildByPushEvent(req.Context(), event); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
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
				if err := a.buildByPullRequest(req.Context(), event); err != nil {
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
				if err := a.buildByPullRequest(req.Context(), event); err != nil {
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
		if err := a.githubRelease(req.Context(), event); err != nil {
			return
		}
	}
}

func (a *Api) allowPullRequest(ctx context.Context, event *github.PullRequestEvent) (bool, error) {
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

func (a *Api) buildByPushEvent(ctx context.Context, event *github.PushEvent) error {
	if err := a.build(ctx, event.Repo.GetHTMLURL(), event.GetAfter(), job.TypeCommit, "push"); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (a *Api) buildByPullRequest(ctx context.Context, event *github.PullRequestEvent) error {
	if err := a.build(ctx, event.Repo.GetHTMLURL(), event.PullRequest.Head.GetSHA(), job.TypeCommit, "pull_request"); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (a *Api) buildPullRequest(ctx context.Context, repoUrl, ownerAndRepo string, number int) error {
	s := strings.Split(ownerAndRepo, "/")
	owner, repo := s[0], s[1]
	pr, res, err := a.githubClient.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return xerrors.New("could not get pr")
	}

	if err := a.build(ctx, repoUrl, pr.GetHead().GetSHA(), job.TypeCommit, "pr"); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	return nil
}

func (a *Api) build(ctx context.Context, repoUrl, revision, jobType, via string) error {
	repos, err := a.dao.Repository.ListByUrl(ctx, repoUrl)
	if err != nil {
		logger.Log.Info("Repository not found or could not get", zap.Error(err))
		return nil
	}
	if len(repos) != 1 {
		return nil
	}
	repo := repos[0]

	jobs, err := a.dao.Job.ListBySourceRepositoryId(ctx, repo.Id)
	if err != nil {
		logger.Log.Warn("Could not get jobs", zap.Error(err))
		return xerrors.Errorf(": %w", err)
	}
	for _, v := range jobs {
		// Trigger the job when Command is build or test only.
		// In other words, If command is run, we are not trigger the job via PushEvent.
		switch v.Command {
		case "build", "test":
		default:
			continue
		}

		if v.JobType != "" && v.JobType != jobType {
			continue
		}

		if _, err := a.builder.Build(ctx, v, revision, v.Command, v.Target, via); err != nil {
			logger.Log.Warn("Failed start job", zap.Error(err), zap.Int32("job.id", v.Id))
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func (a *Api) issueComment(ctx context.Context, event *github.IssueCommentEvent) error {
	switch event.GetAction() {
	case "created":
		if strings.Contains(event.Comment.GetBody(), AllowCommand) {
			users, err := a.dao.TrustedUser.ListByGithubId(ctx, event.Sender.GetID())
			if err != nil {
				return xerrors.Errorf(": %w", err)
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
				return xerrors.Errorf(": %w", err)
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
				return xerrors.Errorf(": %w", err)
			}

			if err := a.buildPullRequest(ctx, event.Repo.GetHTMLURL(), event.Repo.GetFullName(), event.Issue.GetNumber()); err != nil {
				return xerrors.Errorf(": %w", err)
			}
		}
	}
	return nil
}

func (a *Api) githubRelease(ctx context.Context, event *github.ReleaseEvent) error {
	switch event.GetAction() {
	case "published":
		ref, _, err := a.githubClient.Git.GetRef(ctx, event.Repo.Owner.GetLogin(), event.Repo.GetName(), fmt.Sprintf("tags/%s", event.Release.GetTagName()))
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		if err := a.build(ctx, event.Repo.GetHTMLURL(), ref.Object.GetSHA(), job.TypeRelease, "release"); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func (a *Api) handleDiscovery(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if req.Header.Get("Origin") != "" {
		w.Header().Set("Access-Control-Allow-Origin", req.Header.Get("Origin"))
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	if err := req.ParseForm(); err != nil {
		logger.Log.Info("Failed parse form", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	repoId, err := strconv.Atoi(req.FormValue("repository_id"))
	if err != nil {
		logger.Log.Info("Failed parse repository_id", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	repo, err := a.dao.Repository.Select(req.Context(), int32(repoId))
	if err != nil {
		logger.Log.Info("repository not found", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := a.discovery.FindOut(repo, ""); err != nil {
		logger.Log.Warn("Failed start discovery", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (a *Api) handleRun(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if req.Header.Get("Origin") != "" {
		w.Header().Set("Access-Control-Allow-Origin", req.Header.Get("Origin"))
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	if err := req.ParseForm(); err != nil {
		logger.Log.Info("Failed parse form", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	jobId, err := strconv.Atoi(req.FormValue("job_id"))
	if err != nil {
		logger.Log.Info("Failed parse job id", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	j, err := a.dao.Job.Select(req.Context(), int32(jobId))
	if err != nil {
		logger.Log.Info("job not found", zap.Int("job_id", jobId), zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	rev := req.FormValue("revision")
	if rev == "" {
		u, err := url.Parse(j.Repository.Url)
		if err != nil {
			logger.Log.Info("Could not parse repository url", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		s := strings.Split(u.Path, "/")
		owner, repo := s[1], s[2]
		r, _, err := a.githubClient.Repositories.GetCommitSHA1(req.Context(), owner, repo, "master", "")
		if err != nil {
			logger.Log.Info("Could not get revision of master", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		rev = r
	}

	via := req.FormValue("via")
	if via == "" {
		via = "api"
	}

	task, err := a.builder.Build(req.Context(), j, rev, j.Command, j.Target, via)
	if err != nil {
		logger.Log.Warn("Failed build job", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	logger.Log.Info("Success enqueue job", zap.Int("job_id", jobId))
	if err := json.NewEncoder(w).Encode(RunResponse{TaskId: task.Id}); err != nil {
		logger.Log.Warn("Failed encode response", zap.Error(err))
	}
}

type RunResponse struct {
	TaskId int32 `json:"task_id"`
}

func (a *Api) handleRedo(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if req.Header.Get("Origin") != "" {
		w.Header().Set("Access-Control-Allow-Origin", req.Header.Get("Origin"))
		w.Header().Set("Access-Control-Allow-Credentials", "true")
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

	newTask, err := a.builder.Build(req.Context(), task.Job, task.Revision, task.Command, task.Target, "api")
	if err != nil {
		logger.Log.Warn("Failed build job", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	logger.Log.Info("Success enqueue redo-job", zap.Int32("task_id", task.Id), zap.Int32("new_task_id", newTask.Id))
	if err := json.NewEncoder(w).Encode(RunResponse{TaskId: newTask.Id}); err != nil {
		logger.Log.Warn("Failed encode response", zap.Error(err))
	}
}

func (a *Api) handleReadiness(w http.ResponseWriter, req *http.Request) {
	p := probe.NewProbe(a.dao.RawConnection)
	if !p.Ready(req.Context(), database.SchemaHash) {
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

func modifiedRuleFile(e *github.PushEvent) bool {
	for _, v := range e.Commits {
		files := append(v.Added, v.Removed...)
		files = append(files, v.Modified...)
		for _, f := range files {
			b := filepath.Base(f)
			switch b {
			case "BUILD", "BUILD.bazel":
				return true
			case ".bazelversion":
				return true
			}
		}
	}

	return false
}
