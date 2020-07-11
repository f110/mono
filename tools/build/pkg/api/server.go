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
	Build(ctx context.Context, job *database.Job, revision, via string) (*database.Task, error)
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
	mux.HandleFunc("/run", api.handleRun)
	mux.HandleFunc("/liveness", api.handleLiveness)
	mux.HandleFunc("/readiness", api.handleReadiness)
	mux.HandleFunc("/discovery", api.handleDiscovery)
	mux.HandleFunc("/webhook", api.handleWebHook)
	mux.Handle("/favicon.ico", http.NotFoundHandler())
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
			if err := a.discovery.FindOut(repo); err != nil {
				logger.Log.Warn("Could not start discovery job", zap.Error(err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		if isMainBranch(event.GetRef(), event.Repo.GetMasterBranch()) {
			if err := a.buildPush(req.Context(), event); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	case *github.PullRequestEvent:
		if ok, err := a.allowPullRequest(req.Context(), event); err != nil {
			logger.Log.Info("Failed check the build permission", zap.String("repo", event.Repo.GetFullName()), zap.Int("number", event.PullRequest.GetNumber()))
			return
		} else if !ok {
			body := "We could not build this pull request. because this pull request is not allowed due to security reason.\n\n" +
				"For author, Thank you for your contribution. We appreciate your work. Please wait for permitting to build this pull request.\n" +
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
		}
		if err := a.buildPullRequest(req.Context(), event); err != nil {
			logger.Log.Warn("Failed build the pull request", zap.Error(err), zap.String("repo", event.Repo.GetFullName()), zap.Int("number", event.PullRequest.GetNumber()))
			w.WriteHeader(http.StatusInternalServerError)
			return
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

func (a *Api) buildPush(ctx context.Context, event *github.PushEvent) error {
	if err := a.build(ctx, event.Repo.GetHTMLURL(), event.GetAfter(), "push"); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (a *Api) buildPullRequest(ctx context.Context, event *github.PullRequestEvent) error {
	if err := a.build(ctx, event.Repo.GetHTMLURL(), event.GetAfter(), "pull_request"); err != nil {
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
		return err
	}
	for _, v := range jobs {
		// Trigger the job when Command is build or test only.
		// In other words, If command is run, we are not trigger the job via PushEvent.
		switch v.Command {
		case "build", "test":
		default:
			continue
		}

		if _, err := a.builder.Build(ctx, v, revision, via); err != nil {
			logger.Log.Warn("Failed start job", zap.Error(err), zap.Int32("job.id", v.Id))
			return err
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
		}
	}
	return nil
}

func (a *Api) handleDiscovery(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	repoId, err := strconv.Atoi(q.Get("repository_id"))
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

	if err := a.discovery.FindOut(repo); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (a *Api) handleRun(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	jobId, err := strconv.Atoi(q.Get("job_id"))
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

	task, err := a.builder.Build(req.Context(), job, q.Get("revision"), "api")
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
