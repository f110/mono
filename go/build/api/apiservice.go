package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v85/github"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/model"
	"go.f110.dev/mono/go/collections/set"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/storage"
)

type apiService struct {
	builder           Builder
	dao               dao.Options
	githubClient      *github.Client
	gitDataClient     git.GitDataClient
	stClient          *storage.S3
	bazelMirrorPrefix string
	addRepo           chan<- *git.RepositoryConfig
	serverConfig      *ServerConfig
	artifactRegistry  ArtifactRegistry
}

// maxArtifactSize caps the total bytes accepted for a single artifact upload.
const maxArtifactSize = 2 << 30 // 2 GiB

var _ APIServer = (*apiService)(nil)

func newAPIService(builder Builder, dao dao.Options, githubClient *github.Client, gitDataClient git.GitDataClient, stClient *storage.S3, bazelMirrorPrefix string, addRepo chan<- *git.RepositoryConfig, serverConfig *ServerConfig, artifactRegistry ArtifactRegistry) *apiService {
	return &apiService{builder: builder, dao: dao, githubClient: githubClient, gitDataClient: gitDataClient, stClient: stClient, bazelMirrorPrefix: bazelMirrorPrefix, addRepo: addRepo, serverConfig: serverConfig, artifactRegistry: artifactRegistry}
}

func (s *apiService) ListRepositories(ctx context.Context, _ *RequestListRepositories) (*ResponseListRepositories, error) {
	allRepo, err := s.dao.Repository.ListAll(ctx)
	if err != nil {
		slogger.Log.Warn("Failed to list repositories", slogger.E(err))
		return nil, status.Error(codes.Internal, "Failed to list repositories")
	}
	repositories := enumerable.Map(allRepo, s.dbRepoToAPIRepo)
	if s.gitDataClient != nil {
		for i, repo := range allRepo {
			if repo.DefaultBranch == "" {
				continue
			}
			resp, err := s.gitDataClient.GetReference(ctx, &git.RequestGetReference{Repo: repo.Name, Ref: "refs/heads/" + repo.DefaultBranch})
			if err != nil {
				slogger.Log.Warn("Failed to get HEAD revision from git-data-service", slogger.E(err), slog.String("repo", repo.Name))
				continue
			}
			repositories[i].SetHeadRevision(resp.GetRef().GetHash())
		}
	}
	return ResponseListRepositories_builder{Repositories: repositories}.Build(), nil
}

func (s *apiService) ListTasks(ctx context.Context, req *RequestListTasks) (*ResponseListTasks, error) {
	// TODO: Implement pagination
	if len(req.GetIds()) > 0 {
		if len(req.GetIds()) > 100 {
			return nil, status.Error(codes.InvalidArgument, "too many ids specified")
		}
		t, err := s.dao.Task.SelectMulti(ctx, req.GetIds()...)
		if err != nil {
			return nil, status.Error(codes.Internal, "Failed to list tasks")
		}
		var tasks []*model.Task
		tasks = enumerable.Map(t, s.dbTaskToAPITaskWithTestReport(ctx))
		return ResponseListTasks_builder{Tasks: tasks}.Build(), nil
	} else if len(req.GetRepositoryIds()) > 0 {
		var allTasks []*model.Task
		for _, v := range req.GetRepositoryIds() {
			t, err := s.dao.Task.ListByRepositoryId(ctx, v, dao.Sort("id"), dao.Desc)
			if err != nil {
				return nil, status.Error(codes.Internal, "Failed to list tasks")
			}
			tasks := enumerable.Map(t, s.dbTaskToAPITaskWithTestReport(ctx))
			allTasks = append(allTasks, tasks...)
		}
		return ResponseListTasks_builder{Tasks: allTasks}.Build(), nil
	}

	// We don't return test reports when requested all tasks.
	pageSize := 100
	if req.HasPageSize() {
		pageSize = int(req.GetPageSize())
	}
	if pageSize > 100 {
		pageSize = 100
	}
	var receivedTasks []*database.Task
	if req.HasPageToken() {
		boundary, err := strconv.Atoi(req.GetPageToken())
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid page_token")
		}
		t, err := s.dao.Task.ListOffsetAll(ctx, int32(boundary), dao.Limit(pageSize+1), dao.Sort("id"), dao.Desc)
		if err != nil {
			slogger.Log.Warn("Failed to list all tasks", slogger.E(err))
			return nil, status.Error(codes.Internal, "failed to list all tasks")
		}
		receivedTasks = t
	} else {
		t, err := s.dao.Task.ListAll(ctx, dao.Desc, dao.Limit(pageSize+1), dao.Desc)
		if err != nil {
			slogger.Log.Warn("Failed to list all tasks", slogger.E(err))
			return nil, status.Error(codes.Internal, "failed to list all tasks")
		}
		receivedTasks = t
	}
	var nextPageToken *string
	if len(receivedTasks) == pageSize+1 {
		nextPageToken = new(fmt.Sprintf("%d", receivedTasks[pageSize].Id))
		receivedTasks = receivedTasks[:pageSize]
	}
	return ResponseListTasks_builder{
		Tasks:         enumerable.Map(receivedTasks, s.dbTaskToAPITask),
		NextPageToken: nextPageToken,
	}.Build(), nil
}

func (s *apiService) SaveRepository(ctx context.Context, req *RequestSaveRepository) (*ResponseSaveRepository, error) {
	if !req.HasRepository() {
		return nil, status.Error(codes.InvalidArgument, "no repository specified")
	}
	if req.GetRepository().HasId() {
		return nil, status.Error(codes.InvalidArgument, "mutating the repository is not supported yet")
	}

	repo, err := s.dao.Repository.Create(ctx, &database.SourceRepository{
		Name:     req.GetRepository().GetName(),
		Url:      req.GetRepository().GetUrl(),
		CloneUrl: req.GetRepository().GetCloneUrl(),
		Private:  req.GetRepository().GetPrivate(),
	})
	if err != nil {
		slogger.Log.Error("Failed to create repository", slogger.E(err))
		return nil, status.Error(codes.Internal, "failed to save repository")
	}
	if s.addRepo != nil {
		s.addRepo <- &git.RepositoryConfig{Name: repo.Name, URL: repo.CloneUrl, Prefix: repo.Name}
	}
	return ResponseSaveRepository_builder{Repository: s.dbRepoToAPIRepo(repo)}.Build(), nil
}

func (s *apiService) DeleteRepository(ctx context.Context, req *RequestDeleteRepository) (*ResponseDeleteRepository, error) {
	if err := s.dao.Repository.Delete(ctx, req.GetRepositoryId()); err != nil {
		return nil, status.Error(codes.Internal, "failed to delete repository")
	}
	return ResponseDeleteRepository_builder{}.Build(), nil
}

func (s *apiService) ListJobs(ctx context.Context, req *RequestListJobs) (*ResponseListJobs, error) {
	j, err := s.dao.Job.ListByRepositoryId(ctx, req.GetRepositoryId())
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list jobs")
	}
	jobs := enumerable.Map(j, dbJobToAPIJob)
	return ResponseListJobs_builder{Jobs: jobs}.Build(), nil
}

func (s *apiService) InvokeJob(ctx context.Context, req *RequestInvokeJob) (*ResponseInvokeJob, error) {
	if req.HasTaskId() {
		task, err := s.dao.Task.Select(ctx, req.GetTaskId())
		if err != nil {
			slogger.Log.Info("Task is not found", slogger.E(err))
			return nil, status.Error(codes.NotFound, "Task not found")
		}
		u, err := url.Parse(task.Repository.Url)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to parse repository url")
		}
		p := strings.Split(u.Path, "/")
		owner, repoName := p[1], p[2]

		jobConfiguration := &config.JobV2{}
		if task.JobConfiguration != nil && len(*task.JobConfiguration) > 0 {
			j := &config.Job{}
			if err := config.UnmarshalJob([]byte(*task.JobConfiguration), j); err != nil {
				if err := config.UnmarshalJobV2([]byte(*task.JobConfiguration), jobConfiguration, owner, repoName); err != nil {
					return nil, status.Error(codes.FailedPrecondition, err.Error())
				}
			} else {
				jobConfiguration = j.ToV2()
			}
		} else if len(task.ParsedJobConfiguration) > 0 {
			j := &config.Job{}
			if err := config.UnmarshalJob(task.ParsedJobConfiguration, j); err != nil {
				if err := config.UnmarshalJobV2(task.ParsedJobConfiguration, jobConfiguration, owner, repoName); err != nil {
					return nil, status.Error(codes.FailedPrecondition, err.Error())
				}
			} else {
				jobConfiguration = j.ToV2()
			}
		}
		newTasks, err := s.builder.Build(
			ctx,
			task.Repository,
			jobConfiguration,
			task.Revision,
			task.BazelVersion,
			task.Command,
			jobConfiguration.Targets,
			jobConfiguration.Platforms,
			"api",
			false,
		)
		if err != nil {
			slogger.Log.Warn("Failed build job", slogger.E(err))
			return nil, status.Error(codes.Internal, "Failed to build job")
		}

		slogger.Log.Info("Success enqueue redo-job", slog.Int("task_id", int(task.Id)), slog.Int("new_task_id", int(newTasks[len(newTasks)-1].Id)))
		return ResponseInvokeJob_builder{TaskId: new(newTasks[len(newTasks)-1].Id)}.Build(), nil
	}

	if !req.HasRepositoryId() || !req.HasJobName() {
		return nil, status.Error(codes.InvalidArgument, "no repository or job name specified")
	}

	repo, err := s.dao.Repository.Select(ctx, req.GetRepositoryId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "repository not found")
	}
	u, err := url.Parse(repo.Url)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to parse repository url")
	}
	p := strings.Split(u.Path, "/")
	owner, repoName := p[1], p[2]
	jobRow, err := s.dao.Job.Select(ctx, repo.Id, req.GetJobName())
	if err != nil {
		return nil, status.Error(codes.NotFound, "job not found")
	}
	if len(jobRow.Configuration) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "job configuration is not cached yet")
	}
	job := &config.JobV2{}
	if err := config.UnmarshalJobV2(jobRow.Configuration, job, owner, repoName); err != nil {
		return nil, status.Error(codes.Internal, "failed to decode job configuration")
	}
	revision, err := s.resolveDefaultBranchRevision(ctx, owner, repoName, repo.DefaultBranch)
	if err != nil {
		slogger.Log.Warn("Failed to resolve default branch revision", slogger.E(err), slog.String("owner", owner), slog.String("repo", repoName), slog.String("branch", repo.DefaultBranch))
		return nil, status.Error(codes.Internal, "failed to resolve default branch revision")
	}
	newTasks, err := s.builder.Build(
		ctx,
		repo,
		job,
		revision,
		repo.BazelVersion,
		job.Command,
		job.Targets,
		job.Platforms,
		"manual",
		false,
	)
	if err != nil {
		slogger.Log.Warn("Failed to invoke job", slogger.E(err), slog.String("owner", owner), slog.String("repo", repoName), slog.String("job_name", job.Name))
		return nil, status.Error(codes.Internal, "failed to invoke job")
	}
	if newTasks == nil {
		slogger.Log.Warn("Failed to invoke job without error", slog.String("owner", owner), slog.String("repo", repoName), slog.String("job_name", job.Name))
		return nil, status.Error(codes.Internal, "failed to invoke job")
	}

	return ResponseInvokeJob_builder{TaskId: new(newTasks[0].Id)}.Build(), nil
}

// resolveDefaultBranchRevision returns the commit SHA at the tip of branch.
// Prefers git-data-service when configured, otherwise falls back to the
// GitHub API.
func (s *apiService) resolveDefaultBranchRevision(ctx context.Context, owner, repoName, branch string) (string, error) {
	if s.gitDataClient != nil {
		resp, err := s.gitDataClient.GetReference(ctx, &git.RequestGetReference{Repo: repoName, Ref: "refs/heads/" + branch})
		if err != nil {
			return "", err
		}
		return resp.GetRef().GetHash(), nil
	}
	c, _, err := s.githubClient.Repositories.GetCommit(ctx, owner, repoName, branch, nil)
	if err != nil {
		return "", err
	}
	return c.GetSHA(), nil
}

func (s *apiService) ForceStopTask(ctx context.Context, req *RequestForceStopTask) (*ResponseForceStopTask, error) {
	if err := s.builder.ForceStop(ctx, req.GetTaskId()); err != nil {
		return nil, status.Error(codes.Internal, "failed to force stop task")
	}
	return ResponseForceStopTask_builder{}.Build(), nil
}

func (s *apiService) CompleteTask(ctx context.Context, req *RequestCompleteTask) (*ResponseCompleteTask, error) {
	if s.serverConfig == nil || !s.serverConfig.GetEmulateJobControl() {
		return nil, status.Error(codes.FailedPrecondition, "emulate job control is disabled")
	}
	if err := s.builder.CompleteTask(ctx, req.GetTaskId(), req.GetSuccess()); err != nil {
		slogger.Log.Warn("Failed to complete task in emulation", slogger.E(err))
		return nil, status.Error(codes.Internal, "failed to complete task")
	}
	return ResponseCompleteTask_builder{}.Build(), nil
}

func (s *apiService) GetServerInfo(ctx context.Context, _ *RequestGetServerInfo) (*ResponseGetServerInfo, error) {
	objs, err := s.stClient.List(ctx, s.bazelMirrorPrefix)
	if err != nil {
		slogger.Log.Warn("Failed to list bazel mirror objects", slogger.E(err))
	}
	var versions semver.Collection
	for _, v := range objs {
		name := filepath.Base(v.Name)
		if !strings.HasPrefix(name, "bazel-") {
			continue
		}
		ver := name[6:]
		if idx := strings.Index(ver, "-"); idx >= 0 {
			ver = ver[:idx]
		}
		if sv, err := semver.NewVersion(ver); err == nil {
			versions = append(versions, sv)
		}
	}
	versions = enumerable.Uniq(versions, func(t *semver.Version) string { return t.String() })
	sort.Sort(versions)
	versionStrings := enumerable.Map(versions, func(t *semver.Version) string { return t.String() })
	schemaVersion := database.SchemaHash
	return ResponseGetServerInfo_builder{SupportedBazelVersions: versionStrings, SchemaVersion: &schemaVersion, Config: s.serverConfig}.Build(), nil
}

func (s *apiService) ListExternalReleaseTriggers(ctx context.Context, req *RequestListExternalReleaseTriggers) (*ResponseListExternalReleaseTriggers, error) {
	var rows []*database.ExternalReleaseTrigger
	var err error
	if req.GetRepositoryId() > 0 {
		rows, err = s.dao.ExternalReleaseTrigger.ListByRepositoryId(ctx, req.GetRepositoryId())
	} else {
		rows, err = s.dao.ExternalReleaseTrigger.ListAll(ctx)
	}
	if err != nil {
		slogger.Log.Warn("Failed to list external_release_trigger", slogger.E(err))
		return nil, status.Error(codes.Internal, "failed to list external_release_trigger")
	}

	repoCache := make(map[int32]*database.SourceRepository)
	triggers := make([]*model.ExternalReleaseTrigger, 0, len(rows))
	for _, r := range rows {
		repo, ok := repoCache[r.RepositoryId]
		if !ok {
			sr, err := s.dao.Repository.Select(ctx, r.RepositoryId)
			if err != nil {
				slogger.Log.Warn("Failed to load source_repository for trigger", slogger.E(err))
				continue
			}
			repoCache[r.RepositoryId] = sr
			repo = sr
		}
		externalURL := externalRepoURL(r.Provider, r.ExternalRepo)
		triggers = append(triggers, model.ExternalReleaseTrigger_builder{
			Id:                new(r.Id),
			RepositoryId:      new(r.RepositoryId),
			RepositoryName:    new(repo.Name),
			RepositoryUrl:     new(repo.Url),
			JobName:           new(r.JobName),
			Provider:          new(r.Provider),
			ExternalRepo:      new(r.ExternalRepo),
			ExternalRepoUrl:   new(externalURL),
			Kind:              new(r.Kind),
			TagPattern:        new(r.TagPattern),
			IncludePrerelease: new(r.IncludePrerelease),
		}.Build())
	}
	return ResponseListExternalReleaseTriggers_builder{Triggers: triggers}.Build(), nil
}

func (s *apiService) ListGithubEvents(ctx context.Context, req *RequestListGithubEvents) (*ResponseListGithubEvents, error) {
	if req.HasEventId() {
		row, err := s.dao.GithubEvent.Select(ctx, req.GetEventId())
		if err != nil {
			slogger.Log.Info("github_event is not found", slogger.E(err))
			return nil, status.Error(codes.NotFound, "github_event not found")
		}
		return ResponseListGithubEvents_builder{Events: []*model.GithubEvent{dbGithubEventToModel(row)}}.Build(), nil
	}

	rows, err := s.dao.GithubEvent.ListAll(ctx, dao.Sort("id"), dao.Desc)
	if err != nil {
		slogger.Log.Warn("Failed to list github_event", slogger.E(err))
		return nil, status.Error(codes.Internal, "failed to list github_event")
	}

	events := make([]*model.GithubEvent, 0, len(rows))
	for _, r := range rows {
		events = append(events, dbGithubEventToModel(r))
	}
	return ResponseListGithubEvents_builder{Events: events}.Build(), nil
}

// dbGithubEventToModel projects the on-disk row into the wire format the
// dashboard consumes. The proto enum name (e.g. "PENDING") is more useful
// than the integer to a human reader, and status is sent as a raw JSON
// string since its schema varies by event_type.
func dbGithubEventToModel(r *database.GithubEvent) *model.GithubEvent {
	repo, repoURL := extractRepositoryFromPayload(r.Payload)
	b := model.GithubEvent_builder{
		Id:            new(r.Id),
		DeliveryId:    new(r.DeliveryId),
		EventType:     new(r.EventType),
		Action:        new(r.Action),
		State:         new(githubEventStateName(r.State)),
		Status:        new(string(r.Status)),
		LastError:     new(r.LastError),
		CreatedAt:     timestamppb.New(r.CreatedAt),
		Repository:    &repo,
		RepositoryUrl: &repoURL,
	}
	if r.UpdatedAt != nil {
		b.UpdatedAt = timestamppb.New(*r.UpdatedAt)
	}
	return b.Build()
}

// extractRepositoryFromPayload pulls repository.full_name / html_url out of a
// GitHub webhook payload. Every GitHub event payload includes a `repository`
// object, so this works uniformly across event_type. Returns empty strings on
// any parse failure so the API row is still renderable.
func extractRepositoryFromPayload(payload []byte) (name, htmlURL string) {
	if len(payload) == 0 {
		return "", ""
	}
	var p struct {
		Repository struct {
			FullName string `json:"full_name"`
			HTMLURL  string `json:"html_url"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return "", ""
	}
	return p.Repository.FullName, p.Repository.HTMLURL
}

func githubEventStateName(s database.GithubEventState) string {
	switch s {
	case database.GithubEventStatePending:
		return "PENDING"
	case database.GithubEventStateProcessing:
		return "PROCESSING"
	case database.GithubEventStateSucceeded:
		return "SUCCEEDED"
	case database.GithubEventStateFailed:
		return "FAILED"
	case database.GithubEventStateExpired:
		return "EXPIRED"
	case database.GithubEventStateSkipped:
		return "SKIPPED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

func externalRepoURL(provider, repo string) string {
	switch provider {
	case "github":
		return "https://github.com/" + repo
	default:
		return ""
	}
}

func (*apiService) dbTaskToAPITask(task *database.Task) *model.Task {
	var startAt, finishedAt *timestamppb.Timestamp
	if task.StartAt != nil {
		startAt = timestamppb.New(*task.StartAt)
	}
	if task.FinishedAt != nil {
		finishedAt = timestamppb.New(*task.FinishedAt)
	}
	var cpuLimit, memoryLimit string
	if task.JobConfiguration != nil && len(*task.JobConfiguration) > 0 {
		jobConf := &config.Job{}
		if err := config.UnmarshalJob([]byte(*task.JobConfiguration), jobConf); err == nil {
			cpuLimit = jobConf.CPULimit
			memoryLimit = jobConf.MemoryLimit
		}
	} else if len(task.ParsedJobConfiguration) > 0 {
		jobConf := &config.Job{}
		if err := config.UnmarshalJob(task.ParsedJobConfiguration, jobConf); err == nil {
			cpuLimit = jobConf.CPULimit
			memoryLimit = jobConf.MemoryLimit
		} else {
			j := &config.JobV2{}
			if err := config.UnmarshalJobV2(task.ParsedJobConfiguration, j, "", ""); err == nil {
				cpuLimit = j.CPULimit
				memoryLimit = j.MemoryLimit
			}
		}
	}
	return model.Task_builder{
		Id:                  new(task.Id),
		RepositoryId:        new(task.RepositoryId),
		JobName:             new(task.JobName),
		Revision:            new(task.Revision),
		BazelVersion:        new(task.BazelVersion),
		Command:             new(task.Command),
		IsTrunk:             new(task.IsTrunk),
		Success:             new(task.Success),
		LogFile:             new(task.LogFile),
		Targets:             strings.Split(task.Targets, ","),
		Platform:            new(task.Platform),
		Via:                 new(task.Via),
		ConfigName:          new(task.ConfigName),
		Node:                new(task.Node),
		Manifest:            new(task.Manifest),
		Container:           new(task.Container),
		CpuLimit:            new(cpuLimit),
		MemoryLimit:         new(memoryLimit),
		ExecutedTestsCount:  new(task.ExecutedTestsCount),
		SucceededTestsCount: new(task.SucceededTestsCount),
		StartAt:             startAt,
		FinishedAt:          finishedAt,
		CreatedAt:           timestamppb.New(task.CreatedAt),
		UpdatedAt:           timestamppb.New(task.CreatedAt),
	}.Build()
}

func (s *apiService) dbTaskToAPITaskWithTestReport(ctx context.Context) func(v *database.Task) *model.Task {
	return func(v *database.Task) *model.Task {
		task := s.dbTaskToAPITask(v)
		tr, err := s.dao.TestReport.ListByTaskId(ctx, task.GetId())
		if err != nil {
			slogger.Log.Warn("Failed to get test report by task id", slog.Int("task_id", int(task.GetId())))
			return task
		}
		testReports := enumerable.Map(tr, s.dbTestReportToAPITestReport)
		task.SetTestReports(testReports)

		bm, err := s.dao.BuildMetrics.ListByTaskId(ctx, task.GetId())
		if err != nil {
			slogger.Log.Warn("Failed to get build metrics by task id", slog.Int("task_id", int(task.GetId())))
			return task
		}
		if len(bm) > 0 {
			task.SetBuildMetrics(dbBuildMetricsToAPIBuildMetrics(bm[0]))
		}
		return task
	}
}

func (*apiService) dbTestReportToAPITestReport(tr *database.TestReport) *model.TestReport {
	return model.TestReport_builder{
		Label:    new(tr.Label),
		Status:   new(model.TestStatus(tr.Status)),
		Duration: new(tr.Duration),
	}.Build()
}

func dbBuildMetricsToAPIBuildMetrics(m *database.BuildMetrics) *model.BuildMetrics {
	return model.BuildMetrics_builder{
		RemoteCacheHits:               new(m.RemoteCacheHits),
		WallTimeMs:                    new(m.WallTimeMs),
		CpuTimeMs:                     new(m.CpuTimeMs),
		AnalysisPhaseTimeMs:           new(m.AnalysisPhaseTimeMs),
		ExecutionPhaseTimeMs:          new(m.ExecutionPhaseTimeMs),
		ActionsCreated:                new(m.ActionsCreated),
		ActionsExecuted:               new(m.ActionsExecuted),
		GraphActionCount:              new(m.GraphActionCount),
		OutputArtifactCount:           new(m.OutputArtifactCount),
		SkyframeNodeCount:             new(m.SkyframeNodeCount),
		TargetsConfigured:             new(m.TargetsConfigured),
		PackagesLoaded:                new(m.PackagesLoaded),
		SourceArtifactsReadBytes:      new(m.SourceArtifactsReadBytes),
		SourceArtifactsReadCount:      new(m.SourceArtifactsReadCount),
		OutputArtifactsSeenBytes:      new(m.OutputArtifactsSeenBytes),
		OutputArtifactsSeenCount:      new(m.OutputArtifactsSeenCount),
		OutputArtifactsFromCacheBytes: new(m.OutputArtifactsFromCacheBytes),
		OutputArtifactsFromCacheCount: new(m.OutputArtifactsFromCacheCount),
	}.Build()
}

func (*apiService) dbRepoToAPIRepo(repo *database.SourceRepository) *model.Repository {
	return model.Repository_builder{
		Id:       new(repo.Id),
		Name:     new(repo.Name),
		Url:      new(repo.Url),
		CloneUrl: new(repo.CloneUrl),
		Private:  new(repo.Private),
	}.Build()
}

func dbJobToAPIJob(job *database.Job) *model.Job {
	return model.Job_builder{
		Name:         new(job.Name),
		RepositoryId: new(job.RepositoryId),
	}.Build()
}

// ListArtifacts returns the artifacts recorded for a single task (task_id), or
// aggregated over a commit's tasks (repository_id + revision). A commit can be
// built by several tasks (platforms/jobs), so the commit form unions them.
func (s *apiService) ListArtifacts(ctx context.Context, req *RequestListArtifacts) (*ResponseListArtifacts, error) {
	if req.GetTaskId() > 0 {
		rows, err := s.dao.Artifact.ListByTaskId(ctx, req.GetTaskId(), dao.Sort("id"))
		if err != nil {
			slogger.Log.Warn("Failed to list artifacts by task", slogger.E(err), slog.Int("task_id", int(req.GetTaskId())))
			return nil, status.Error(codes.Internal, "failed to list artifacts")
		}
		return ResponseListArtifacts_builder{Artifacts: enumerable.Map(rows, dbArtifactToAPIArtifact)}.Build(), nil
	}
	if req.GetRepositoryId() > 0 && req.GetRevision() != "" {
		tasks, err := s.dao.Task.ListByRevision(ctx, req.GetRepositoryId(), req.GetRevision(), dao.Sort("id"))
		if err != nil {
			slogger.Log.Warn("Failed to list tasks by revision", slogger.E(err), slog.String("revision", req.GetRevision()))
			return nil, status.Error(codes.Internal, "failed to list artifacts")
		}
		var artifacts []*model.Artifact
		for _, t := range tasks {
			rows, err := s.dao.Artifact.ListByTaskId(ctx, t.Id, dao.Sort("id"))
			if err != nil {
				slogger.Log.Warn("Failed to list artifacts by task", slogger.E(err), slog.Int("task_id", int(t.Id)))
				return nil, status.Error(codes.Internal, "failed to list artifacts")
			}
			artifacts = append(artifacts, enumerable.Map(rows, dbArtifactToAPIArtifact)...)
		}
		return ResponseListArtifacts_builder{Artifacts: artifacts}.Build(), nil
	}
	return nil, status.Error(codes.InvalidArgument, "task_id or repository_id+revision must be specified")
}

func dbArtifactToAPIArtifact(a *database.Artifact) *model.Artifact {
	return model.Artifact_builder{
		Id:         new(a.Id),
		TaskId:     new(a.TaskId),
		Type:       new(model.ArtifactType(a.Type)),
		Hash:       new(a.Hash),
		Name:       new(a.Name),
		Repository: new(a.Repository),
		Reference:  new(a.Reference),
		Size:       new(a.Size),
		CreatedAt:  timestamppb.New(a.CreatedAt),
	}.Build()
}

// SyncArtifacts records a production row for the task for every artifact whose
// content is already stored (reusing the existing storage location, no bytes
// needed) and returns the content hashes not yet stored, which the sidecar then
// uploads. This keeps per-commit transmission minimal while still recording which
// artifacts each commit produced.
func (s *apiService) SyncArtifacts(ctx context.Context, req *RequestSyncArtifacts) (*ResponseSyncArtifacts, error) {
	var newHashes []string
	seen := set.New[string]()
	for _, a := range req.GetArtifacts() {
		h := a.GetHash()
		if h == "" {
			continue
		}
		if seen.Has(h) {
			continue
		}
		seen.Add(h)

		existing, err := s.dao.Artifact.ListByHash(ctx, h)
		if err != nil {
			slogger.Log.Warn("Failed to look up artifact by hash", slogger.E(err))
			return nil, status.Error(codes.Internal, "failed to look up artifact")
		}
		if len(existing) == 0 {
			newHashes = append(newHashes, h) // content not stored yet: needs upload
			continue
		}
		if _, err := s.dao.Artifact.Create(ctx, &database.Artifact{
			TaskId:     req.GetTaskId(),
			Type:       database.ArtifactType(a.GetType()),
			Hash:       h,
			Name:       a.GetName(),
			Repository: existing[0].Repository,
			Reference:  existing[0].Reference,
			Size:       existing[0].Size,
		}); err != nil {
			slogger.Log.Warn("Failed to record existing artifact", slogger.E(err), slog.String("hash", h))
		}
	}
	return ResponseSyncArtifacts_builder{NewHashes: newHashes}.Build(), nil
}

// UploadArtifact receives a single artifact (metadata first, then content
// chunks), verifies its declared hash, pushes the content to the OCI registry
// and records a row in the artifact table. It is disabled when the builder has
// no artifact registry configured.
func (s *apiService) UploadArtifact(stream grpc.ClientStreamingServer[RequestUploadArtifact, ResponseUploadArtifact]) error {
	if s.artifactRegistry == nil {
		return status.Error(codes.FailedPrecondition, "artifact registry is not configured")
	}
	meta, data, err := collectUpload(stream.Recv)
	if err != nil {
		return err
	}
	res, err := s.storeArtifact(stream.Context(), meta, data)
	if err != nil {
		return err
	}
	return stream.SendAndClose(res)
}

// storeArtifact verifies the declared hash, pushes the content to the OCI
// registry and records a row in the artifact table. It is separated from the
// stream handling so it can be unit-tested without a fake gRPC stream.
func (s *apiService) storeArtifact(ctx context.Context, meta *ArtifactUploadMeta, data []byte) (*ResponseUploadArtifact, error) {
	if !meta.HasArtifact() {
		return nil, status.Error(codes.InvalidArgument, "missing artifact metadata")
	}
	ref := meta.GetArtifact()
	if ref.GetHash() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing artifact hash")
	}

	var reference string
	var err error
	switch ref.GetType() {
	case ArtifactType_ARTIFACT_TYPE_BINARY, ArtifactType_ARTIFACT_TYPE_K8S_MANIFEST:
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != ref.GetHash() {
			return nil, status.Errorf(codes.InvalidArgument, "content hash mismatch: got %s, declared %s", got, ref.GetHash())
		}
		reference, err = s.artifactRegistry.PushFile(ctx, ref.GetRepository(), ref.GetTag(), artifactMediaType(ref.GetType()), ref.GetName(), data)
	case ArtifactType_ARTIFACT_TYPE_CONTAINER_IMAGE:
		reference, err = s.artifactRegistry.PushImage(ctx, ref.GetRepository(), ref.GetTag(), ref.GetHash(), bytes.NewReader(data))
	default:
		return nil, status.Error(codes.InvalidArgument, "unknown artifact type")
	}
	if err != nil {
		slogger.Log.Warn("Failed to push artifact to registry", slogger.E(err))
		return nil, status.Error(codes.Internal, "failed to store artifact")
	}

	if _, err := s.dao.Artifact.Create(ctx, &database.Artifact{
		TaskId:     meta.GetTaskId(),
		Type:       database.ArtifactType(ref.GetType()),
		Hash:       ref.GetHash(),
		Name:       ref.GetName(),
		Repository: ref.GetRepository(),
		Reference:  reference,
		Size:       int64(len(data)),
	}); err != nil {
		// A retry races on the unique (task, hash) index. Treat a duplicate as
		// success: the content is stored either way.
		if rows, e := s.dao.Artifact.ListByHash(ctx, ref.GetHash()); e == nil && len(rows) > 0 {
			return ResponseUploadArtifact_builder{Repository: new(rows[0].Repository), Reference: new(rows[0].Reference)}.Build(), nil
		}
		slogger.Log.Warn("Failed to record artifact", slogger.E(err))
		return nil, status.Error(codes.Internal, "failed to record artifact")
	}
	return ResponseUploadArtifact_builder{Repository: new(ref.GetRepository()), Reference: new(reference)}.Build(), nil
}

// collectUpload assembles an UploadArtifact stream: the first message must carry
// the metadata, the rest the content bytes. Split out from the RPC method so the
// assembly logic is unit-testable with a fake recv.
func collectUpload(recv func() (*RequestUploadArtifact, error)) (*ArtifactUploadMeta, []byte, error) {
	var meta *ArtifactUploadMeta
	var buf bytes.Buffer
	for {
		msg, err := recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		switch {
		case msg.HasMeta():
			if meta != nil {
				return nil, nil, status.Error(codes.InvalidArgument, "duplicate metadata message")
			}
			meta = msg.GetMeta()
		case msg.HasChunk():
			if meta == nil {
				return nil, nil, status.Error(codes.InvalidArgument, "metadata must be the first message")
			}
			if buf.Len()+len(msg.GetChunk()) > maxArtifactSize {
				return nil, nil, status.Error(codes.InvalidArgument, "artifact exceeds maximum size")
			}
			buf.Write(msg.GetChunk())
		}
	}
	if meta == nil {
		return nil, nil, status.Error(codes.InvalidArgument, "no metadata received")
	}
	return meta, buf.Bytes(), nil
}

// artifactMediaTypes maps an artifact type to the OCI artifactType/config media
// type used when storing file artifacts.
var artifactMediaTypes = map[ArtifactType]string{
	ArtifactType_ARTIFACT_TYPE_BINARY:       "application/vnd.mono.build.binary",
	ArtifactType_ARTIFACT_TYPE_K8S_MANIFEST: "application/vnd.mono.build.k8s-manifest",
}

func artifactMediaType(t ArtifactType) string {
	if mt, ok := artifactMediaTypes[t]; ok {
		return mt
	}
	return "application/octet-stream"
}
