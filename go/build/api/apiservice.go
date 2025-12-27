package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/go-github/v73/github"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/model"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/varptr"
)

type apiService struct {
	builder      Builder
	dao          dao.Options
	githubClient *github.Client
}

var _ APIServer = (*apiService)(nil)

func newAPIService(builder Builder, dao dao.Options, githubClient *github.Client) *apiService {
	return &apiService{builder: builder, dao: dao, githubClient: githubClient}
}

func (s *apiService) ListRepositories(ctx context.Context, _ *RequestListRepositories) (*ResponseListRepositories, error) {
	allRepo, err := s.dao.Repository.ListAll(ctx)
	if err != nil {
		logger.Log.Warn("Failed to list repositories", logger.Error(err))
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
			logger.Log.Warn("Failed to list all tasks", logger.Error(err))
			return nil, status.Error(codes.Internal, "failed to list all tasks")
		}
		receivedTasks = t
	} else {
		t, err := s.dao.Task.ListAll(ctx, dao.Desc, dao.Limit(pageSize+1), dao.Desc)
		if err != nil {
			logger.Log.Warn("Failed to list all tasks", logger.Error(err))
			return nil, status.Error(codes.Internal, "failed to list all tasks")
		}
		receivedTasks = t
	}
	var nextPageToken *string
	if len(receivedTasks) == pageSize+1 {
		nextPageToken = varptr.Ptr(fmt.Sprintf("%d", receivedTasks[pageSize].Id))
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
		logger.Log.Error("Failed to create repository", logger.Error(err))
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

	manuallyTriggerableJobs := enumerable.FindAll(c.Jobs, func(job *config.Job) bool { return enumerable.IsInclude(job.Event, config.EventManual) })
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
			logger.Log.Info("Task is not found", logger.Error(err))
			return nil, status.Error(codes.NotFound, "Task not found")
		}

		jobConfiguration := &config.Job{}
		if task.JobConfiguration != nil && len(*task.JobConfiguration) > 0 {
			if err := config.UnmarshalJob([]byte(*task.JobConfiguration), jobConfiguration); err != nil {
				return nil, status.Error(codes.FailedPrecondition, err.Error())
			}
		} else if len(task.ParsedJobConfiguration) > 0 {
			if err := config.UnmarshalJob(task.ParsedJobConfiguration, jobConfiguration); err != nil {
				return nil, status.Error(codes.FailedPrecondition, err.Error())
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
			logger.Log.Warn("Failed build job", logger.Error(err))
			return nil, status.Error(codes.Internal, "Failed to build job")
		}

		logger.Log.Info("Success enqueue redo-job", logger.Int32("task_id", task.Id), logger.Int32("new_task_id", newTasks[len(newTasks)-1].Id))
		return ResponseInvokeJob_builder{TaskId: varptr.Ptr(newTasks[len(newTasks)-1].Id)}.Build(), nil
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
	i := enumerable.Index(conf.Jobs, func(job *config.Job) bool { return job.Name == req.GetJobName() })
	job := conf.Jobs[i]
	bazelVersion, err := config.GetBazelVersionFromRepository(ctx, s.githubClient, owner, repoName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get bazel version")
	}
	newTasks, err := s.builder.Build(
		ctx,
		repo,
		job,
		repo.DefaultBranch,
		bazelVersion,
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

	return ResponseInvokeJob_builder{TaskId: varptr.Ptr(taskID)}.Build(), nil
}

func (s *apiService) ForceStopTask(ctx context.Context, req *RequestForceStopTask) (*ResponseForceStopTask, error) {
	if err := s.builder.ForceStop(ctx, req.GetTaskId()); err != nil {
		return nil, status.Error(codes.Internal, "failed to force stop task")
	}
	return ResponseForceStopTask_builder{}.Build(), nil
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
		}
	}
	return model.Task_builder{
		Id:                  varptr.Ptr(task.Id),
		RepositoryId:        varptr.Ptr(task.RepositoryId),
		JobName:             varptr.Ptr(task.JobName),
		Revision:            varptr.Ptr(task.Revision),
		BazelVersion:        varptr.Ptr(task.BazelVersion),
		Command:             varptr.Ptr(task.Command),
		IsTrunk:             varptr.Ptr(task.IsTrunk),
		Success:             varptr.Ptr(task.Success),
		LogFile:             varptr.Ptr(task.LogFile),
		Targets:             strings.Split(task.Targets, ","),
		Platform:            varptr.Ptr(task.Platform),
		Via:                 varptr.Ptr(task.Via),
		ConfigName:          varptr.Ptr(task.ConfigName),
		Node:                varptr.Ptr(task.Node),
		Manifest:            varptr.Ptr(task.Manifest),
		Container:           varptr.Ptr(task.Container),
		CpuLimit:            varptr.Ptr(cpuLimit),
		MemoryLimit:         varptr.Ptr(memoryLimit),
		ExecutedTestsCount:  varptr.Ptr(task.ExecutedTestsCount),
		SucceededTestsCount: varptr.Ptr(task.SucceededTestsCount),
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
			logger.Log.Warn("Failed to get test report by task id", logger.Int32("task_id", task.GetId()))
			return task
		}
		testReports := enumerable.Map(tr, s.dbTestReportToAPITestReport)
		task.SetTestReports(testReports)
		return task
	}
}

func (*apiService) dbTestReportToAPITestReport(tr *database.TestReport) *model.TestReport {
	return model.TestReport_builder{
		Label:    varptr.Ptr(tr.Label),
		Status:   varptr.Ptr(model.TestStatus(tr.Status)),
		Duration: varptr.Ptr(tr.Duration),
	}.Build()
}

func (*apiService) dbRepoToAPIRepo(repo *database.SourceRepository) *model.Repository {
	return model.Repository_builder{
		Id:       varptr.Ptr(repo.Id),
		Name:     varptr.Ptr(repo.Name),
		Url:      varptr.Ptr(repo.Url),
		CloneUrl: varptr.Ptr(repo.CloneUrl),
		Private:  varptr.Ptr(repo.Private),
		Status:   varptr.Ptr(model.RepositoryStatus(repo.Status)),
	}.Build()
}

func dbJobToAPIJob(job *database.Job) *model.Job {
	return model.Job_builder{
		Name:         varptr.Ptr(job.Name),
		RepositoryId: varptr.Ptr(job.RepositoryId),
	}.Build()
}
