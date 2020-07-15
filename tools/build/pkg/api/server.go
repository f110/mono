package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v32/github"
	"go.f110.dev/protoc-ddl/probe"
	"go.uber.org/zap"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/lib/logger"
	"go.f110.dev/mono/tools/build/pkg/database"
	"go.f110.dev/mono/tools/build/pkg/database/dao"
	"go.f110.dev/mono/tools/build/pkg/discovery"
)

const (
	AllowCommand = "/allow-build"
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

func NewApi(addr string, builder Builder, discovery *discovery.Discover, dao dao.Options, appId, installationId int64, privateKeyFile string) (*Api, error) {
	var transport *ghinstallation.Transport
	if privateKeyFile != "" {
		t, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appId, installationId, privateKeyFile)
		if err != nil {
			return nil, xerrors.Errorf(": %v", err)
		}
		transport = t
	}

	api := &Api{
		builder:      builder,
		discovery:    discovery,
		dao:          dao,
		githubClient: github.NewClient(&http.Client{Transport: transport}),
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
			repo, err := a.dao.Repository.SelectByUrl(req.Context(), event.Repo.GetHTMLURL())
			if err != nil {
				logger.Log.Warn("Could not find repository", zap.Error(err))
				return
			}

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
				if err := a.buildByPullRequest(req.Context(), event); err != nil {
					logger.Log.Warn("Failed build the pull request", zap.Error(err))
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
		case "synchronize":
			if ok, _ := a.allowPullRequest(req.Context(), event); ok {
				if err := a.buildByPullRequest(req.Context(), event); err != nil {
					logger.Log.Warn("Failed build the pull request", zap.Error(err), zap.String("repo", event.Repo.GetFullName()), zap.Int("number", event.PullRequest.GetNumber()))
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
		case "closed":
			permit, err := a.dao.PermitPullRequest.SelectByRepositoryAndNumber(req.Context(), event.Repo.GetFullName(), int32(event.PullRequest.GetNumber()))
			if err != nil {
				return
			}
			if permit == nil {
				return
			}
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
	}
}

func (a *Api) allowPullRequest(ctx context.Context, event *github.PullRequestEvent) (bool, error) {
	user, err := a.dao.TrustedUser.SelectByGithubId(ctx, event.Sender.GetID())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Log.Warn("Could not get trusted user", zap.Error(err), zap.Int64("sender.id", event.Sender.GetID()))
		return false, err
	}
	if user != nil {
		return true, nil
	}

	permitPullRequest, err := a.dao.PermitPullRequest.SelectByRepositoryAndNumber(ctx, event.Repo.GetFullName(), int32(event.PullRequest.GetNumber()))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Log.Warn("Could not get permit pull request", zap.Error(err), zap.String("repo", event.Repo.GetFullName()), zap.Int("number", event.PullRequest.GetNumber()))
		return false, err
	}
	if permitPullRequest != nil {
		return true, nil
	}

	return false, nil
}

func (a *Api) buildByPushEvent(ctx context.Context, event *github.PushEvent) error {
	if err := a.build(ctx, event.Repo.GetHTMLURL(), event.GetAfter(), "push"); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (a *Api) buildByPullRequest(ctx context.Context, event *github.PullRequestEvent) error {
	if err := a.build(ctx, event.Repo.GetHTMLURL(), event.PullRequest.Head.GetSHA(), "pull_request"); err != nil {
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

	if err := a.build(ctx, repoUrl, pr.GetHead().GetSHA(), "pr"); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	return nil
}

func (a *Api) build(ctx context.Context, repoUrl, revision, via string) error {
	repo, err := a.dao.Repository.SelectByUrl(ctx, repoUrl)
	if err != nil {
		logger.Log.Info("Repository not found or could not get", zap.Error(err))
		return nil
	}
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
			user, err := a.dao.TrustedUser.SelectByGithubId(ctx, event.Sender.GetID())
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
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

func (a *Api) handleDiscovery(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
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

	repo, err := a.dao.Repository.SelectById(req.Context(), int32(repoId))
	if err != nil {
		logger.Log.Info("repository not found", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := a.discovery.FindOut(repo, ""); err != nil {
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

	job, err := a.dao.Job.SelectById(req.Context(), int32(jobId))
	if err != nil {
		logger.Log.Info("job not found", zap.Int("job_id", jobId), zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	task, err := a.builder.Build(req.Context(), job, req.FormValue("revision"), job.Command, job.Target, "api")
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

	task, err := a.dao.Task.SelectById(req.Context(), int32(taskId))
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
			}
		}
	}

	return false
}