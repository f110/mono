package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v85/github"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/model"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/storage"
)

type apiService struct {
	builder           Builder
	dao               dao.Options
	githubClient      *github.Client
	stClient          *storage.S3
	bazelMirrorPrefix string
}

var _ APIServer = (*apiService)(nil)

func newAPIService(builder Builder, dao dao.Options, githubClient *github.Client, stClient *storage.S3, bazelMirrorPrefix string) *apiService {
	return &apiService{builder: builder, dao: dao, githubClient: githubClient, stClient: stClient, bazelMirrorPrefix: bazelMirrorPrefix}
}

func (s *apiService) ListRepositories(ctx context.Context, _ *RequestListRepositories) (*ResponseListRepositories, error) {
	allRepo, err := s.dao.Repository.ListAll(ctx)
	if err != nil {
		slogger.Log.Warn("Failed to list repositories", slogger.E(err))
		return nil, status.Error(codes.Internal, "Failed to list repositories")
	}
	repositories := enumerable.Map(allRepo, s.dbRepoToAPIRepo)
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
	return ResponseSaveRepository_builder{Repository: s.dbRepoToAPIRepo(repo)}.Build(), nil
}

func (s *apiService) DeleteRepository(ctx context.Context, req *RequestDeleteRepository) (*ResponseDeleteRepository, error) {
	if err := s.dao.Repository.Delete(ctx, req.GetRepositoryId()); err != nil {
		return nil, status.Error(codes.Internal, "failed to delete repository")
	}
	return ResponseDeleteRepository_builder{}.Build(), nil
}

func (s *apiService) SyncRepository(ctx context.Context, req *RequestSyncRepository) (*ResponseSyncRepository, error) {
	repository, err := s.dao.Repository.Select(ctx, req.GetRepositoryId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "repository not found")
	}
	u, err := url.Parse(repository.Url)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to parse repository url")
	}
	p := strings.Split(u.Path, "/")
	owner, repoName := p[1], p[2]

	// Update job configurations
	c, err := config.ReadFromRepository(ctx, s.githubClient, owner, repoName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read config")
	}
	if c == nil {
		return ResponseSyncRepository_builder{}.Build(), nil
	}

	manuallyTriggerableJobs := enumerable.FindAll(c.Jobs, func(job *config.JobV2) bool { return enumerable.IsInclude(job.Event, config.EventManual) })
	if len(manuallyTriggerableJobs) == 0 {
		return ResponseSyncRepository_builder{}.Build(), nil
	}
	jobs, err := s.dao.Job.ListByRepositoryId(ctx, repository.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list jobs")
	}
	for _, v := range jobs {
		if err := s.dao.Job.Delete(ctx, v.RepositoryId, v.Name); err != nil {
			return nil, status.Error(codes.Internal, "failed to delete job")
		}
	}
	for _, v := range manuallyTriggerableJobs {
		if _, err := s.dao.Job.Create(ctx, &database.Job{RepositoryId: repository.Id, Name: v.Name}); err != nil {
			return nil, status.Error(codes.Internal, "failed to create job")
		}
	}

	repo, _, err := s.githubClient.Repositories.Get(ctx, owner, repoName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get repository")
	}
	if repo.GetDefaultBranch() != repository.DefaultBranch {
		repository.DefaultBranch = repo.GetDefaultBranch()
	}

	repository.Status = database.SourceRepositoryStatusReady
	if err := s.dao.Repository.Update(ctx, repository); err != nil {
		return nil, status.Error(codes.Internal, "failed to update repository")
	}
	return ResponseSyncRepository_builder{}.Build(), nil
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

	var taskID int32
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
	conf, err := config.ReadFromRepository(ctx, s.githubClient, owner, repoName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read config")
	}
	if conf == nil {
		return nil, status.Error(codes.Internal, "failed to read config")
	}
	i := enumerable.Index(conf.Jobs, func(job *config.JobV2) bool { return job.Name == req.GetJobName() })
	job := conf.Jobs[i]
	newTasks, err := s.builder.Build(
		ctx,
		repo,
		job,
		repo.DefaultBranch,
		conf.BazelVersion,
		job.Command,
		job.Targets,
		job.Platforms,
		"manual",
		false,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to invoke job")
	}
	if newTasks == nil {
		return nil, status.Error(codes.Internal, "failed to invoke job")
	}
	taskID = newTasks[0].Id

	return ResponseInvokeJob_builder{TaskId: new(taskID)}.Build(), nil
}

func (s *apiService) ForceStopTask(ctx context.Context, req *RequestForceStopTask) (*ResponseForceStopTask, error) {
	if err := s.builder.ForceStop(ctx, req.GetTaskId()); err != nil {
		return nil, status.Error(codes.Internal, "failed to force stop task")
	}
	return ResponseForceStopTask_builder{}.Build(), nil
}

func (s *apiService) GetServerInfo(ctx context.Context, _ *RequestGetServerInfo) (*ResponseGetServerInfo, error) {
	objs, err := s.stClient.List(ctx, s.bazelMirrorPrefix)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list bazel mirror objects")
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
	return ResponseGetServerInfo_builder{SupportedBazelVersions: versionStrings}.Build(), nil
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

func (*apiService) dbRepoToAPIRepo(repo *database.SourceRepository) *model.Repository {
	return model.Repository_builder{
		Id:       new(repo.Id),
		Name:     new(repo.Name),
		Url:      new(repo.Url),
		CloneUrl: new(repo.CloneUrl),
		Private:  new(repo.Private),
		Status:   new(model.RepositoryStatus(repo.Status)),
	}.Build()
}

func dbJobToAPIJob(job *database.Job) *model.Job {
	return model.Job_builder{
		Name:         new(job.Name),
		RepositoryId: new(job.RepositoryId),
	}.Build()
}
